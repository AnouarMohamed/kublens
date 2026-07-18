package model

type SessionUser struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type AuthSession struct {
	Enabled       bool         `json:"enabled"`
	Authenticated bool         `json:"authenticated"`
	User          *SessionUser `json:"user,omitempty"`
	Permissions   []string     `json:"permissions"`
}

type AuditEntry struct {
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
	Hash         string `json:"hash,omitempty"`
}

type AuditLogResponse struct {
	Total int          `json:"total"`
	Items []AuditEntry `json:"items"`
}

type AuditVerification struct {
	ID           string `json:"id"`
	OK           bool   `json:"ok"`
	Message      string `json:"message"`
	PreviousHash string `json:"previousHash,omitempty"`
	Hash         string `json:"hash,omitempty"`
	VerifiedAt   string `json:"verifiedAt"`
}

type StreamEvent struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Payload   any    `json:"payload"`
}
