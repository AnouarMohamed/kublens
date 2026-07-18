package httpapi

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"kubelens-backend/internal/auth"
	storesql "kubelens-backend/internal/db"
	"kubelens-backend/internal/events"
	"kubelens-backend/internal/model"
)

const (
	defaultAuditLimit = 120
	maxAuditLimit     = 500
)

type AuditConfig struct {
	MaxItems   int
	FilePath   string
	SigningKey string
	Store      string
	SQLDB      *sql.DB
	Dialect    storesql.Dialect
}

type auditLog struct {
	maxItems   int
	counter    atomic.Uint64
	sink       auditSink
	store      string
	durable    bool
	signingKey []byte
	failures   atomic.Uint64
	mu         sync.RWMutex
	items      []model.AuditEntry
	lastError  string
}

type auditSink interface {
	write(entry model.AuditEntry) error
}

type fileAuditSink struct {
	mu   sync.Mutex
	file *os.File
}

type sqlAuditSink struct {
	db      *sql.DB
	dialect storesql.Dialect
}

type auditPosture struct {
	Store       string
	Durable     bool
	Signed      bool
	Failures    uint64
	LastError   string
	EntryCount  int
	HasSink     bool
	MemoryLimit int
}

func WithAuditConfig(config AuditConfig) Option {
	return func(s *Server) {
		maxItems := config.MaxItems
		if maxItems <= 0 {
			maxItems = maxAuditLimit
		}
		config.MaxItems = maxItems
		s.audit = newAuditLogWithConfig(config, s.logger)
	}
}

func newAuditLog(maxItems int, filePath string, signingKey string, logger *slog.Logger) *auditLog {
	return newAuditLogWithConfig(AuditConfig{
		MaxItems:   maxItems,
		FilePath:   filePath,
		SigningKey: signingKey,
	}, logger)
}

func newAuditLogWithConfig(config AuditConfig, logger *slog.Logger) *auditLog {
	maxItems := config.MaxItems
	if maxItems <= 0 {
		maxItems = maxAuditLimit
	}
	store := normalizeAuditStore(config.Store, config.FilePath)
	log := &auditLog{
		maxItems:   maxItems,
		store:      store,
		signingKey: []byte(strings.TrimSpace(config.SigningKey)),
		items:      make([]model.AuditEntry, 0, maxItems),
	}

	switch store {
	case "memory":
		return log
	case "file":
		trimmedPath := strings.TrimSpace(config.FilePath)
		sink, err := newFileAuditSink(trimmedPath)
		if err != nil {
			log.lastError = err.Error()
			if logger != nil {
				logger.Warn("audit file sink disabled", "path", trimmedPath, "error", err.Error())
			}
			return log
		}
		log.sink = sink
		log.durable = true
		items, maxID, err := loadAuditEntries(trimmedPath, maxItems)
		if err != nil {
			log.lastError = err.Error()
			if logger != nil {
				logger.Warn("audit history load failed", "path", trimmedPath, "error", err.Error())
			}
			return log
		}
		log.items = items
		log.counter.Store(maxID)
		return log
	case "sql":
		if config.SQLDB == nil {
			log.lastError = "sql audit store unavailable"
			return log
		}
		dialect := config.Dialect
		if dialect == "" {
			dialect = storesql.DialectSQLite
		}
		log.sink = &sqlAuditSink{db: config.SQLDB, dialect: dialect}
		log.durable = true
		items, maxID, err := loadAuditEntriesFromSQL(config.SQLDB, dialect, maxItems)
		if err != nil {
			log.lastError = err.Error()
			if logger != nil {
				logger.Warn("audit sql history load failed", "error", err.Error())
			}
			return log
		}
		log.items = items
		log.counter.Store(maxID)
		return log
	default:
		log.store = "memory"
		return log
	}
}

func (l *auditLog) append(entry model.AuditEntry) model.AuditEntry {
	entry.ID = strconv.FormatUint(l.counter.Add(1), 10)
	if strings.TrimSpace(entry.Timestamp) == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	l.mu.Lock()
	if len(l.items) > 0 {
		entry.PreviousHash = strings.TrimSpace(l.items[len(l.items)-1].Hash)
	}
	entry.Hash = hashAuditEntry(entry)
	entry.Signature = signAuditEntry(entry, l.signingKey)
	l.items = append(l.items, entry)
	if overflow := len(l.items) - l.maxItems; overflow > 0 {
		l.items = append([]model.AuditEntry(nil), l.items[overflow:]...)
	}
	sink := l.sink
	l.mu.Unlock()

	if sink != nil {
		if err := sink.write(entry); err != nil {
			l.failures.Add(1)
			l.mu.Lock()
			l.lastError = err.Error()
			l.durable = false
			l.mu.Unlock()
		}
	}

	return entry
}

func (l *auditLog) verify(id string, now time.Time) (model.AuditVerification, bool) {
	target := strings.TrimSpace(id)
	if target == "" {
		return model.AuditVerification{}, false
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	previousHash := ""
	for _, entry := range l.items {
		expectedPrevious := previousHash
		expectedHash := hashAuditEntryWithPrevious(entry, expectedPrevious)
		expectedSignature := signAuditEntryWithHash(expectedHash, l.signingKey)
		if entry.ID == target {
			hashOK := strings.TrimSpace(entry.Hash) != "" &&
				entry.PreviousHash == expectedPrevious &&
				entry.Hash == expectedHash
			signatureOK := len(l.signingKey) == 0 ||
				(strings.TrimSpace(entry.Signature) != "" && hmac.Equal([]byte(entry.Signature), []byte(expectedSignature)))
			ok := hashOK && signatureOK
			message := "verified"
			if strings.TrimSpace(entry.Hash) == "" {
				message = "entry-missing-hash"
			} else if entry.PreviousHash != expectedPrevious {
				message = "previous-hash-mismatch"
			} else if entry.Hash != expectedHash {
				message = "hash-mismatch"
			} else if len(l.signingKey) > 0 && strings.TrimSpace(entry.Signature) == "" {
				message = "entry-missing-signature"
			} else if len(l.signingKey) > 0 && !signatureOK {
				message = "signature-mismatch"
			}
			return model.AuditVerification{
				ID:           entry.ID,
				OK:           ok,
				Message:      message,
				PreviousHash: entry.PreviousHash,
				Hash:         entry.Hash,
				Signature:    entry.Signature,
				VerifiedAt:   now.UTC().Format(time.RFC3339),
			}, true
		}
		previousHash = strings.TrimSpace(entry.Hash)
	}
	return model.AuditVerification{}, false
}

func (l *auditLog) list(limit int) []model.AuditEntry {
	if limit <= 0 {
		limit = defaultAuditLimit
	}
	if limit > maxAuditLimit {
		limit = maxAuditLimit
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	count := minInt(limit, len(l.items))
	out := make([]model.AuditEntry, 0, count)
	for i := len(l.items) - 1; i >= 0 && len(out) < count; i-- {
		out = append(out, l.items[i])
	}
	return out
}

func (l *auditLog) total() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.items)
}

func (l *auditLog) posture() auditPosture {
	if l == nil {
		return auditPosture{Store: "unavailable"}
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return auditPosture{
		Store:       l.store,
		Durable:     l.durable,
		Signed:      len(l.signingKey) > 0,
		Failures:    l.failures.Load(),
		LastError:   l.lastError,
		EntryCount:  len(l.items),
		HasSink:     l.sink != nil,
		MemoryLimit: l.maxItems,
	}
}

func (s *Server) auditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := s.now()
		next.ServeHTTP(ww, r)

		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}

		entry := model.AuditEntry{
			Timestamp:  s.now().UTC().Format(time.RFC3339),
			RequestID:  middleware.GetReqID(r.Context()),
			Method:     r.Method,
			Path:       sanitizeAuditPath(r.URL.Path),
			Route:      routePattern(r),
			Action:     actionForRequest(r.Method, r.URL.Path),
			Status:     status,
			DurationMs: s.now().Sub(start).Milliseconds(),
			Bytes:      int64(ww.BytesWritten()),
			ClientIP:   s.clientIPFromRequest(r),
			Success:    status < http.StatusBadRequest,
		}
		if p, ok := auth.PrincipalFromContext(r.Context()); ok {
			entry.User = p.User
			entry.Role = auth.RoleLabel(p.Role)
		}

		saved := s.audit.append(entry)
		if s.eventBus != nil {
			s.eventBus.Publish(events.Event{
				Type:      "audit",
				Timestamp: saved.Timestamp,
				Payload:   saved,
			})
		}
	})
}

type AuditController struct {
	audit *auditLog
}

func NewAuditController(audit *auditLog) *AuditController {
	return &AuditController{audit: audit}
}

func (ac *AuditController) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", ac.handleAuditLog)
	r.Get("/{id}/verify", ac.handleAuditVerification)
	return r
}

func (ac *AuditController) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	if ac.audit == nil {
		writeJSON(w, http.StatusOK, model.AuditLogResponse{})
		return
	}

	limit := parsePositiveInt(r.URL.Query().Get("limit"), defaultAuditLimit)
	items := ac.audit.list(limit)
	writeJSON(w, http.StatusOK, model.AuditLogResponse{
		Total: ac.audit.total(),
		Items: items,
	})
}

func (ac *AuditController) handleAuditVerification(w http.ResponseWriter, r *http.Request) {
	if ac.audit == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "audit log unavailable"})
		return
	}

	verification, ok := ac.audit.verify(chi.URLParam(r, "id"), time.Now())
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "audit entry not found"})
		return
	}
	status := http.StatusOK
	if !verification.OK {
		status = http.StatusConflict
	}
	writeJSON(w, status, verification)
}

func parsePositiveInt(raw string, fallback int) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func sanitizeAuditPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/"
	}
	return trimmed
}

func sanitizeClientIP(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(trimmed)
	if err == nil {
		return host
	}
	return trimmed
}

func newFileAuditSink(path string) (*fileAuditSink, error) {
	clean := filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(clean), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(clean, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &fileAuditSink{file: file}, nil
}

func (s *fileAuditSink) write(entry model.AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bytes, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := s.file.Write(append(bytes, '\n')); err != nil {
		return err
	}
	return s.file.Sync()
}

func (s *sqlAuditSink) write(entry model.AuditEntry) error {
	if s == nil || s.db == nil {
		return os.ErrInvalid
	}
	sequenceID, _ := strconv.ParseInt(strings.TrimSpace(entry.ID), 10, 64)
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(
		context.Background(),
		s.dialect.Bind(`INSERT INTO audit_entries (
			id, sequence_id, timestamp, action, status, success, hash, signature, entry_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		entry.ID,
		sequenceID,
		entry.Timestamp,
		entry.Action,
		entry.Status,
		entry.Success,
		entry.Hash,
		entry.Signature,
		string(payload),
	)
	return err
}

func loadAuditEntries(path string, maxItems int) ([]model.AuditEntry, uint64, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	defer file.Close()

	entries := make([]model.AuditEntry, 0, maxItems)
	var maxID uint64

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry model.AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if id, err := strconv.ParseUint(strings.TrimSpace(entry.ID), 10, 64); err == nil && id > maxID {
			maxID = id
		}
		entries = append(entries, entry)
		if overflow := len(entries) - maxItems; overflow > 0 {
			entries = append([]model.AuditEntry(nil), entries[overflow:]...)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}

	return entries, maxID, nil
}

func loadAuditEntriesFromSQL(db *sql.DB, dialect storesql.Dialect, maxItems int) ([]model.AuditEntry, uint64, error) {
	if db == nil {
		return nil, 0, os.ErrInvalid
	}
	if dialect == "" {
		dialect = storesql.DialectSQLite
	}

	rows, err := db.QueryContext(
		context.Background(),
		dialect.Bind(`SELECT id, entry_json
		   FROM audit_entries
		  ORDER BY sequence_id DESC
		  LIMIT ?`),
		maxItems,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	reversed := make([]model.AuditEntry, 0, maxItems)
	var maxID uint64
	for rows.Next() {
		var id, payload string
		if err := rows.Scan(&id, &payload); err != nil {
			continue
		}
		var entry model.AuditEntry
		if err := json.Unmarshal([]byte(payload), &entry); err != nil {
			continue
		}
		if parsed, err := strconv.ParseUint(strings.TrimSpace(id), 10, 64); err == nil && parsed > maxID {
			maxID = parsed
		}
		reversed = append(reversed, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	entries := make([]model.AuditEntry, 0, len(reversed))
	for i := len(reversed) - 1; i >= 0; i-- {
		entries = append(entries, reversed[i])
	}
	return entries, maxID, nil
}

func normalizeAuditStore(store string, filePath string) string {
	value := strings.ToLower(strings.TrimSpace(store))
	if value != "" {
		return value
	}
	if strings.TrimSpace(filePath) != "" {
		return "file"
	}
	return "memory"
}

type auditHashMaterial struct {
	ID           string `json:"id"`
	Timestamp    string `json:"timestamp"`
	RequestID    string `json:"requestId,omitempty"`
	Method       string `json:"method"`
	Path         string `json:"path"`
	Route        string `json:"route,omitempty"`
	Action       string `json:"action,omitempty"`
	Status       int    `json:"status"`
	DurationMs   int64  `json:"durationMs"`
	Bytes        int64  `json:"bytes"`
	ClientIP     string `json:"clientIp,omitempty"`
	User         string `json:"user,omitempty"`
	Role         string `json:"role,omitempty"`
	Success      bool   `json:"success"`
	PreviousHash string `json:"previousHash,omitempty"`
}

func hashAuditEntry(entry model.AuditEntry) string {
	return hashAuditEntryWithPrevious(entry, entry.PreviousHash)
}

func hashAuditEntryWithPrevious(entry model.AuditEntry, previousHash string) string {
	material := auditHashMaterial{
		ID:           entry.ID,
		Timestamp:    entry.Timestamp,
		RequestID:    entry.RequestID,
		Method:       entry.Method,
		Path:         entry.Path,
		Route:        entry.Route,
		Action:       entry.Action,
		Status:       entry.Status,
		DurationMs:   entry.DurationMs,
		Bytes:        entry.Bytes,
		ClientIP:     entry.ClientIP,
		User:         entry.User,
		Role:         entry.Role,
		Success:      entry.Success,
		PreviousHash: previousHash,
	}
	bytes, _ := json.Marshal(material)
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func signAuditEntry(entry model.AuditEntry, signingKey []byte) string {
	return signAuditEntryWithHash(entry.Hash, signingKey)
}

func signAuditEntryWithHash(hash string, signingKey []byte) string {
	if len(signingKey) == 0 || strings.TrimSpace(hash) == "" {
		return ""
	}
	mac := hmac.New(sha256.New, signingKey)
	_, _ = mac.Write([]byte(hash))
	return hex.EncodeToString(mac.Sum(nil))
}

func actionForRequest(method, path string) string {
	m := strings.ToUpper(strings.TrimSpace(method))
	switch {
	case m == http.MethodPost && path == apiMountPrefix+"/incidents":
		return "incident.create"
	case m == http.MethodPatch && strings.HasPrefix(path, apiMountPrefix+"/incidents/") && strings.Contains(path, "/steps/"):
		return "incident.step.update"
	case m == http.MethodPost && strings.HasPrefix(path, apiMountPrefix+"/incidents/") && strings.HasSuffix(path, "/resolve"):
		return "incident.resolve"
	case m == http.MethodPost && strings.HasPrefix(path, apiMountPrefix+"/incidents/") && strings.HasSuffix(path, "/postmortem"):
		return "postmortem.generate"
	case m == http.MethodPost && path == apiMountPrefix+"/remediation/propose":
		return "remediation.propose"
	case m == http.MethodPost && strings.HasPrefix(path, apiMountPrefix+"/remediation/") && strings.HasSuffix(path, "/gitops"):
		return "remediation.gitops.generate"
	case m == http.MethodPost && strings.HasPrefix(path, apiMountPrefix+"/remediation/") && strings.HasSuffix(path, "/approve"):
		return "remediation.approve"
	case m == http.MethodPost && strings.HasPrefix(path, apiMountPrefix+"/remediation/") && strings.HasSuffix(path, "/execute"):
		return "remediation.execute"
	case m == http.MethodPost && strings.HasPrefix(path, apiMountPrefix+"/remediation/") && strings.HasSuffix(path, "/reject"):
		return "remediation.reject"
	case m == http.MethodPost && path == apiMountPrefix+"/memory/runbooks":
		return "memory.runbook.create"
	case m == http.MethodPut && strings.HasPrefix(path, apiMountPrefix+"/memory/runbooks/"):
		return "memory.runbook.update"
	case m == http.MethodPost && path == apiMountPrefix+"/memory/fixes":
		return "memory.fix.record"
	case m == http.MethodPost && path == apiMountPrefix+"/risk-guard/analyze":
		return "riskguard.analyze"
	case m == http.MethodPost && path == apiMountPrefix+"/ghost/simulations":
		return "ghost.simulate"
	case m == http.MethodGet && path == apiMountPrefix+"/rightsizing":
		return "rightsizing.view"
	case m == http.MethodPost && path == apiMountPrefix+"/alerts/lifecycle":
		return "alert.lifecycle.update"
	case m == http.MethodPost && path == apiMountPrefix+"/pods":
		return "pod.create"
	case m == http.MethodPost && strings.HasSuffix(path, "/restart") && strings.Contains(path, apiMountPrefix+"/pods/"):
		return "pod.restart"
	case m == http.MethodDelete && strings.Contains(path, apiMountPrefix+"/pods/"):
		return "pod.delete"
	case m == http.MethodPost && strings.HasSuffix(path, "/cordon"):
		return "node.cordon"
	case m == http.MethodPost && strings.HasSuffix(path, "/uncordon"):
		return "node.uncordon"
	case m == http.MethodPost && strings.HasSuffix(path, "/drain"):
		return "node.drain"
	case m == http.MethodPut && strings.HasSuffix(path, "/yaml"):
		return "resource.apply"
	case m == http.MethodPost && strings.HasSuffix(path, "/scale"):
		return "resource.scale"
	case m == http.MethodPost && strings.HasSuffix(path, "/rollback"):
		return "resource.rollback"
	case m == http.MethodPost && strings.HasSuffix(path, "/restart"):
		return "resource.restart"
	case m == http.MethodPost && path == apiMountPrefix+"/assistant":
		return "assistant.ask"
	case m == http.MethodPost && path == apiMountPrefix+"/assistant/references/feedback":
		return "assistant.reference.feedback"
	default:
		route := strings.TrimSpace(path)
		if route == "" {
			route = "/"
		}
		return strings.ToLower(m) + " " + route
	}
}
