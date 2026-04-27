package httpapi

import (
	"errors"
	"math"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"kubelens-backend/internal/auth"
	"kubelens-backend/internal/model"
)

type AuthConfig = auth.Config

type AuthToken = auth.Token

type authRuntime struct {
	enabled            bool
	trustedCSRFDomains []string
	authenticator      *auth.Authenticator
	cookieName         string
}

type AuthLoginProtectionConfig struct {
	Enabled       bool
	MaxFailures   int
	FailureWindow time.Duration
	Lockout       time.Duration
	MaxEntries    int
}

type authLoginProtection struct {
	enabled       bool
	maxFailures   int
	failureWindow time.Duration
	lockout       time.Duration
	maxEntries    int

	mu       sync.Mutex
	attempts map[string]authLoginAttempt
}

type authLoginAttempt struct {
	failures    int
	firstFailed time.Time
	lastSeen    time.Time
	lockUntil   time.Time
}

func WithAuth(config AuthConfig) Option {
	return func(s *Server) {
		s.auth.configure(config)
	}
}

func WithAuthLoginProtection(config AuthLoginProtectionConfig) Option {
	return func(s *Server) {
		if s.authLogin == nil {
			s.authLogin = newAuthLoginProtection(defaultAuthLoginProtectionConfig())
		}
		s.authLogin.configure(config)
	}
}

func (a *authRuntime) configure(config AuthConfig) {
	a.enabled = config.Enabled
	a.trustedCSRFDomains = normalizeDomains(config.TrustedCSRFDomains)
	a.authenticator = auth.NewAuthenticator(config)
	if a.authenticator != nil {
		a.cookieName = a.authenticator.CookieName()
	}
}

func defaultAuthLoginProtectionConfig() AuthLoginProtectionConfig {
	return AuthLoginProtectionConfig{
		Enabled:       true,
		MaxFailures:   5,
		FailureWindow: 15 * time.Minute,
		Lockout:       5 * time.Minute,
		MaxEntries:    4096,
	}
}

func newAuthLoginProtection(config AuthLoginProtectionConfig) *authLoginProtection {
	out := &authLoginProtection{}
	out.configure(config)
	return out
}

func (p *authLoginProtection) configure(config AuthLoginProtectionConfig) {
	maxFailures := config.MaxFailures
	if maxFailures <= 0 {
		maxFailures = 5
	}
	failureWindow := config.FailureWindow
	if failureWindow <= 0 {
		failureWindow = 15 * time.Minute
	}
	lockout := config.Lockout
	if lockout <= 0 {
		lockout = 5 * time.Minute
	}
	maxEntries := config.MaxEntries
	if maxEntries <= 0 {
		maxEntries = 4096
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.enabled = config.Enabled
	p.maxFailures = maxFailures
	p.failureWindow = failureWindow
	p.lockout = lockout
	p.maxEntries = maxEntries
	if p.attempts == nil {
		p.attempts = make(map[string]authLoginAttempt)
	}
}

func (p *authLoginProtection) allow(now time.Time, clientIP string) (bool, time.Duration) {
	if !p.enabled {
		return true, 0
	}

	key := normalizeClientKey(clientIP)

	p.mu.Lock()
	defer p.mu.Unlock()

	entry, ok := p.attempts[key]
	if !ok {
		return true, 0
	}

	if !entry.lockUntil.IsZero() {
		if now.Before(entry.lockUntil) {
			return false, entry.lockUntil.Sub(now)
		}
		delete(p.attempts, key)
		return true, 0
	}

	if !entry.firstFailed.IsZero() && now.Sub(entry.firstFailed) > p.failureWindow {
		delete(p.attempts, key)
	}

	return true, 0
}

func (p *authLoginProtection) registerFailure(now time.Time, clientIP string) (bool, time.Duration) {
	if !p.enabled {
		return false, 0
	}

	key := normalizeClientKey(clientIP)

	p.mu.Lock()
	defer p.mu.Unlock()

	entry := p.attempts[key]
	if !entry.firstFailed.IsZero() && now.Sub(entry.firstFailed) > p.failureWindow {
		entry = authLoginAttempt{}
	}

	if entry.firstFailed.IsZero() {
		entry.firstFailed = now
		entry.failures = 1
	} else {
		entry.failures++
	}
	entry.lastSeen = now

	if entry.failures >= p.maxFailures {
		entry.failures = 0
		entry.firstFailed = time.Time{}
		entry.lockUntil = now.Add(p.lockout)
		p.attempts[key] = entry
		p.enforceCapLocked()
		return true, p.lockout
	}

	p.attempts[key] = entry
	p.enforceCapLocked()
	return false, 0
}

func (p *authLoginProtection) registerSuccess(clientIP string) {
	if !p.enabled {
		return
	}

	key := normalizeClientKey(clientIP)
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.attempts, key)
}

func (p *authLoginProtection) enforceCapLocked() {
	if len(p.attempts) <= p.maxEntries {
		return
	}

	for len(p.attempts) > p.maxEntries {
		var (
			oldestKey   string
			oldestSeen  time.Time
			initialized bool
		)
		for key, entry := range p.attempts {
			if !initialized || entry.lastSeen.Before(oldestSeen) {
				oldestKey = key
				oldestSeen = entry.lastSeen
				initialized = true
			}
		}
		if !initialized {
			return
		}
		delete(p.attempts, oldestKey)
	}
}

func normalizeClientKey(clientIP string) string {
	normalized := strings.TrimSpace(clientIP)
	if normalized == "" || normalized == "-" {
		return "unknown"
	}
	return normalized
}

func isAuthBypassPath(path string) bool {
	switch path {
	case apiAuthLoginPath, apiAuthLogoutPath, apiHealthzPath, apiReadyzPath, apiOpenAPIPath:
		return true
	default:
		return false
	}
}

func (s *Server) maybeAttachAuthSessionPrincipal(r *http.Request) *http.Request {
	if !s.auth.enabled || s.auth.authenticator == nil {
		return r
	}
	principal, _, err := s.auth.authenticator.AuthenticateRequest(r)
	if err != nil {
		return r
	}
	return r.WithContext(auth.WithPrincipal(r.Context(), principal))
}

func (s *Server) authenticateProtectedRequest(r *http.Request) (auth.Principal, auth.Channel, error) {
	if !s.auth.enabled {
		return auth.Principal{User: "local-viewer", Role: auth.RoleViewer}, auth.ChannelUnknown, nil
	}
	if s.auth.authenticator == nil {
		return auth.Principal{}, auth.ChannelUnknown, errors.New("authenticator not configured")
	}
	return s.auth.authenticator.AuthenticateRequest(r)
}

type authRejection struct {
	status  int
	action  string
	message string
}

func (s *Server) authorizeProtectedRequest(r *http.Request, principal auth.Principal, channel auth.Channel) *authRejection {
	required := auth.RequiredRole(r.Method, r.URL.Path)
	if principal.Role < required {
		return &authRejection{
			status:  http.StatusForbidden,
			action:  "forbidden",
			message: "insufficient role for this action",
		}
	}
	if required >= auth.RoleOperator && !s.writesOn && auth.RequiresWriteGate(r.Method, r.URL.Path) {
		return &authRejection{
			status:  http.StatusForbidden,
			message: "mutating operations are disabled for this environment",
		}
	}
	if isMutatingMethod(r.Method) && channel == auth.ChannelCookie {
		if err := validateCSRFSameOrigin(r, s.auth.trustedCSRFDomains); err != nil {
			return &authRejection{
				status:  http.StatusForbidden,
				action:  "csrf_blocked",
				message: err.Error(),
			}
		}
	}

	return nil
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if !isAPIPath(path) || isAuthBypassPath(path) {
			next.ServeHTTP(w, r)
			return
		}

		if path == apiAuthSessionPath {
			next.ServeHTTP(w, s.maybeAttachAuthSessionPrincipal(r))
			return
		}

		principal, channel, err := s.authenticateProtectedRequest(r)
		if err != nil {
			s.recordAuthFailure(r, http.StatusUnauthorized, "unauthenticated")
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}

		if rejection := s.authorizeProtectedRequest(r, principal, channel); rejection != nil {
			if strings.TrimSpace(rejection.action) != "" {
				s.recordAuthFailure(r, rejection.status, rejection.action)
			}
			writeError(w, rejection.status, rejection.message)
			return
		}

		next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), principal)))
	})
}

type authLoginRequest struct {
	Token string `json:"token"`
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if !s.auth.enabled {
		writeError(w, http.StatusBadRequest, "auth is disabled")
		return
	}

	clientIP := s.clientIPFromRequest(r)
	if s.authLogin != nil {
		if allowed, retryAfter := s.authLogin.allow(s.now(), clientIP); !allowed {
			s.recordAuthFailure(r, http.StatusTooManyRequests, "login_rate_limited")
			setRetryAfterHeader(w, retryAfter)
			writeError(w, http.StatusTooManyRequests, "too many failed login attempts; try again later")
			return
		}
	}

	var req authLoginRequest
	if err := s.decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	if s.auth.authenticator == nil {
		writeError(w, http.StatusInternalServerError, "authenticator not configured")
		return
	}
	p, err := s.auth.authenticator.VerifyToken(r.Context(), token)
	if err != nil {
		locked := false
		var retryAfter time.Duration
		if s.authLogin != nil {
			locked, retryAfter = s.authLogin.registerFailure(s.now(), clientIP)
		}
		s.recordAuthFailure(r, http.StatusUnauthorized, "login_failed")
		if locked {
			s.recordAuthFailure(r, http.StatusTooManyRequests, "login_rate_limited")
			setRetryAfterHeader(w, retryAfter)
			writeError(w, http.StatusTooManyRequests, "too many failed login attempts; try again later")
			return
		}
		writeError(w, http.StatusUnauthorized, "invalid bearer token")
		return
	}

	if s.authLogin != nil {
		s.authLogin.registerSuccess(clientIP)
	}
	s.writeAuthCookie(w, r, token)
	writeJSON(w, http.StatusOK, model.AuthSession{
		Enabled:       true,
		Authenticated: true,
		User: &model.SessionUser{
			Name: p.User,
			Role: auth.RoleLabel(p.Role),
		},
		Permissions: auth.PermissionsForRole(p.Role),
	})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	s.clearAuthCookie(w, r)
	writeJSON(w, http.StatusOK, model.AuthSession{
		Enabled:       s.auth.enabled,
		Authenticated: false,
		Permissions:   nil,
	})
}

func (s *Server) writeAuthCookie(w http.ResponseWriter, r *http.Request, token string) {
	if strings.TrimSpace(s.auth.cookieName) == "" {
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.auth.cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   12 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   !s.runtime.Insecure || requestIsSecure(r),
	})
}

func (s *Server) clearAuthCookie(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(s.auth.cookieName) == "" {
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.auth.cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   !s.runtime.Insecure || requestIsSecure(r),
	})
}

func requestIsSecure(r *http.Request) bool {
	if r != nil && r.TLS != nil {
		return true
	}
	if r == nil {
		return false
	}

	forwarded := strings.TrimSpace(strings.ToLower(r.Header.Get("X-Forwarded-Proto")))
	return forwarded == "https"
}

func setRetryAfterHeader(w http.ResponseWriter, retryAfter time.Duration) {
	if retryAfter <= 0 {
		w.Header().Set("Retry-After", "1")
		return
	}

	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
}

func isMutatingMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func normalizeDomains(raw []string) []string {
	out := make([]string, 0, len(raw))
	for _, domain := range raw {
		normalized := strings.ToLower(strings.TrimSpace(domain))
		if normalized == "" {
			continue
		}
		if !slices.Contains(out, normalized) {
			out = append(out, normalized)
		}
	}
	return out
}

func validateCSRFSameOrigin(r *http.Request, trustedDomains []string) error {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	referer := strings.TrimSpace(r.Header.Get("Referer"))
	targetHost := strings.ToLower(strings.TrimSpace(r.Host))
	if targetHost == "" {
		return errors.New("host header is required")
	}

	if origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host == "" {
			return errors.New("invalid request origin")
		}
		if hostAllowed(strings.ToLower(parsed.Host), targetHost, trustedDomains) {
			return nil
		}
		return errors.New("cross-site request blocked")
	}

	if referer != "" {
		parsed, err := url.Parse(referer)
		if err != nil || parsed.Host == "" {
			return errors.New("invalid request referer")
		}
		if hostAllowed(strings.ToLower(parsed.Host), targetHost, trustedDomains) {
			return nil
		}
		return errors.New("cross-site request blocked")
	}

	return errors.New("csrf protection requires origin or referer header")
}

func hostAllowed(candidate, host string, trustedDomains []string) bool {
	if hostMatches(candidate, host) {
		return true
	}
	for _, domain := range trustedDomains {
		if hostMatches(candidate, domain) {
			return true
		}
	}
	return false
}

func hostMatches(candidate, expected string) bool {
	cHost, cPort := parseHostAndPort(candidate)
	eHost, ePort := parseHostAndPort(expected)
	if cHost == "" || eHost == "" {
		return false
	}
	if cHost != eHost {
		return false
	}

	// If either side omitted an explicit port, treat it as host-level match.
	if cPort == "" || ePort == "" {
		return true
	}
	return cPort == ePort
}

func parseHostAndPort(raw string) (string, string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ""
	}

	if host, port, err := net.SplitHostPort(trimmed); err == nil {
		return strings.ToLower(strings.TrimSpace(host)), strings.TrimSpace(port)
	}

	if parsed, err := url.Parse("//" + trimmed); err == nil {
		host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
		port := strings.TrimSpace(parsed.Port())
		if host != "" {
			return host, port
		}
	}

	return strings.ToLower(trimmed), ""
}

func (s *Server) handleAuthSession(w http.ResponseWriter, r *http.Request) {
	session := model.AuthSession{
		Enabled:       s.auth.enabled,
		Authenticated: false,
		Permissions:   nil,
	}
	if !s.auth.enabled {
		session.Permissions = append([]string(nil), s.anonPerms...)
	}

	p, ok := auth.PrincipalFromContext(r.Context())
	if ok {
		session.Authenticated = true
		session.User = &model.SessionUser{
			Name: p.User,
			Role: auth.RoleLabel(p.Role),
		}
		session.Permissions = auth.PermissionsForRole(p.Role)
	}

	writeJSON(w, http.StatusOK, session)
}

func (s *Server) recordAuthFailure(r *http.Request, status int, action string) {
	if s.audit == nil {
		return
	}

	s.audit.append(model.AuditEntry{
		Timestamp: s.now().UTC().Format(time.RFC3339),
		RequestID: middleware.GetReqID(r.Context()),
		Method:    r.Method,
		Path:      sanitizeAuditPath(r.URL.Path),
		Action:    action,
		Status:    status,
		ClientIP:  s.clientIPFromRequest(r),
		Success:   false,
	})
}
