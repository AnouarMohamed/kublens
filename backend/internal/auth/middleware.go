// Package auth provides request authentication primitives and RBAC helpers.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"net/http"
	"strings"
)

type Token struct {
	// Token is the shared secret value accepted by the authenticator.
	Token string
	// User is the principal name associated with the token.
	User string
	// Role is the RBAC role label mapped to Role values.
	Role string
}

// Config configures the request authenticator.
type Config struct {
	Enabled            bool
	AllowHeaderToken   bool
	TrustedCSRFDomains []string
	Tokens             []Token
	CookieName         string
	OIDC               OIDCConfig
}

type Channel int

const (
	// ChannelUnknown indicates no supported auth channel was found.
	ChannelUnknown Channel = iota
	// ChannelBearer indicates Authorization Bearer token authentication.
	ChannelBearer
	// ChannelHeader indicates X-Auth-Token header authentication.
	ChannelHeader
	// ChannelCookie indicates cookie-based session authentication.
	ChannelCookie
)

// Authenticator validates bearer/header/cookie credentials.
type Authenticator struct {
	config       Config
	staticTokens []staticToken
	oidc         *oidcVerifier
}

type staticToken struct {
	digest    [sha256.Size]byte
	principal Principal
}

// NewAuthenticator creates an Authenticator from static token and OIDC settings.
func NewAuthenticator(cfg Config) *Authenticator {
	if strings.TrimSpace(cfg.CookieName) == "" {
		cfg.CookieName = "kubelens_auth"
	}
	if cfg.OIDC.Provider != "" || strings.TrimSpace(cfg.OIDC.IssuerURL) != "" {
		cfg.OIDC.Enabled = true
	}

	items := make([]staticToken, 0, len(cfg.Tokens))
	for _, token := range cfg.Tokens {
		secret := strings.TrimSpace(token.Token)
		if secret == "" {
			continue
		}

		user := strings.TrimSpace(token.User)
		if user == "" {
			user = "operator"
		}
		items = append(items, staticToken{
			digest: sha256.Sum256([]byte(secret)),
			principal: Principal{
				User:     user,
				Role:     ParseRole(token.Role),
				Provider: "static",
			},
		})
	}

	return &Authenticator{
		config:       cfg,
		staticTokens: items,
		oidc:         newOIDCVerifier(cfg.OIDC),
	}
}

// CookieName returns the configured authentication cookie name.
func (a *Authenticator) CookieName() string {
	if a == nil {
		return ""
	}
	return a.config.CookieName
}

// AuthenticateRequest extracts credentials and authenticates the request principal.
func (a *Authenticator) AuthenticateRequest(r *http.Request) (Principal, Channel, error) {
	if a == nil {
		return Principal{}, ChannelUnknown, errors.New("authenticator not configured")
	}

	token := strings.TrimSpace(readBearerToken(r.Header.Get("Authorization")))
	channel := ChannelBearer
	if token == "" {
		if a.config.AllowHeaderToken {
			token = strings.TrimSpace(r.Header.Get("X-Auth-Token"))
			if token != "" {
				channel = ChannelHeader
			}
		}
	}
	if token == "" {
		if cookie := readAuthCookie(r, a.config.CookieName); cookie != "" {
			token = cookie
			channel = ChannelCookie
		}
	}
	if token == "" {
		return Principal{}, ChannelUnknown, errors.New("missing bearer token")
	}

	principal, err := a.VerifyToken(r.Context(), token)
	if err != nil {
		return Principal{}, channel, err
	}
	return principal, channel, nil
}

// VerifyToken validates a token using static mappings and optional OIDC verification.
func (a *Authenticator) VerifyToken(ctx context.Context, token string) (Principal, error) {
	if a == nil {
		return Principal{}, errors.New("authenticator not configured")
	}
	if principal, ok := a.verifyStaticToken(strings.TrimSpace(token)); ok {
		return principal, nil
	}

	if a.oidc != nil && a.config.OIDC.Enabled {
		principal, err := a.oidc.verify(ctx, token)
		if err != nil {
			return Principal{}, err
		}
		if principal.User == "" {
			principal.User = "oidc-user"
		}
		return principal, nil
	}

	return Principal{}, errors.New("invalid bearer token")
}

func (a *Authenticator) verifyStaticToken(token string) (Principal, bool) {
	if token == "" || len(a.staticTokens) == 0 {
		return Principal{}, false
	}

	digest := sha256.Sum256([]byte(token))
	for _, candidate := range a.staticTokens {
		if hmac.Equal(candidate.digest[:], digest[:]) {
			return candidate.principal, true
		}
	}
	return Principal{}, false
}

func readBearerToken(raw string) string {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

func readAuthCookie(r *http.Request, cookieName string) string {
	if strings.TrimSpace(cookieName) == "" {
		return ""
	}
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}
