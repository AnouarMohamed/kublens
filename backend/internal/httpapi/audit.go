package httpapi

import (
	"bufio"
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

	"github.com/go-chi/chi/v5/middleware"

	"kubelens-backend/internal/auth"
	"kubelens-backend/internal/events"
	"kubelens-backend/internal/model"
)

const (
	defaultAuditLimit = 120
	maxAuditLimit     = 500
)

type AuditConfig struct {
	MaxItems int
	FilePath string
}

type auditLog struct {
	maxItems int
	counter  atomic.Uint64
	sink     auditSink
	mu       sync.RWMutex
	items    []model.AuditEntry
}

type auditSink interface {
	write(entry model.AuditEntry) error
}

type fileAuditSink struct {
	mu   sync.Mutex
	file *os.File
}

func WithAuditConfig(config AuditConfig) Option {
	return func(s *Server) {
		maxItems := config.MaxItems
		if maxItems <= 0 {
			maxItems = maxAuditLimit
		}
		s.audit = newAuditLog(maxItems, config.FilePath, s.logger)
	}
}

func newAuditLog(maxItems int, filePath string, logger *slog.Logger) *auditLog {
	if maxItems <= 0 {
		maxItems = maxAuditLimit
	}
	log := &auditLog{
		maxItems: maxItems,
		items:    make([]model.AuditEntry, 0, maxItems),
	}

	trimmedPath := strings.TrimSpace(filePath)
	if trimmedPath == "" {
		return log
	}

	sink, err := newFileAuditSink(trimmedPath)
	if err != nil {
		if logger != nil {
			logger.Warn("audit file sink disabled", "path", trimmedPath, "error", err.Error())
		}
		return log
	}

	log.sink = sink
	items, maxID, err := loadAuditEntries(trimmedPath, maxItems)
	if err != nil {
		if logger != nil {
			logger.Warn("audit history load failed", "path", trimmedPath, "error", err.Error())
		}
		return log
	}
	log.items = items
	log.counter.Store(maxID)
	return log
}

func (l *auditLog) append(entry model.AuditEntry) model.AuditEntry {
	entry.ID = strconv.FormatUint(l.counter.Add(1), 10)
	if strings.TrimSpace(entry.Timestamp) == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	l.mu.Lock()
	l.items = append(l.items, entry)
	if overflow := len(l.items) - l.maxItems; overflow > 0 {
		l.items = append([]model.AuditEntry(nil), l.items[overflow:]...)
	}
	sink := l.sink
	l.mu.Unlock()

	if sink != nil {
		_ = sink.write(entry)
	}

	return entry
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

func (s *Server) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	limit := parsePositiveInt(r.URL.Query().Get("limit"), defaultAuditLimit)
	items := s.audit.list(limit)
	writeJSON(w, http.StatusOK, model.AuditLogResponse{
		Total: s.audit.total(),
		Items: items,
	})
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
