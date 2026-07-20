package auth

import (
	"net/http"
	"strings"
)

type Role int

const (
	// RoleViewer grants read, stream, and assistant access.
	RoleViewer Role = iota + 1
	// RoleOperator adds mutation capabilities guarded by the write gate.
	RoleOperator
	// RoleAdmin represents administrative access level.
	RoleAdmin
)

// ParseRole converts a role label into a Role value.
func ParseRole(raw string) Role {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "admin":
		return RoleAdmin
	case "operator":
		return RoleOperator
	default:
		return RoleViewer
	}
}

// RoleLabel returns the canonical role label for a Role value.
func RoleLabel(role Role) string {
	switch role {
	case RoleAdmin:
		return "admin"
	case RoleOperator:
		return "operator"
	default:
		return "viewer"
	}
}

// PermissionsForRole expands a role into UI-facing capability strings.
func PermissionsForRole(role Role) []string {
	switch role {
	case RoleAdmin, RoleOperator:
		return []string{"read", "assist", "stream", "write"}
	default:
		return []string{"read", "assist", "stream"}
	}
}

// RequiredRole returns the minimum role required to access an API route.
func RequiredRole(method, path string) Role {
	cleanMethod := strings.ToUpper(strings.TrimSpace(method))
	cleanPath := strings.TrimSpace(path)

	switch {
	// Intentionally viewer-level: these endpoints do not mutate cluster state.
	case cleanMethod == http.MethodPost && cleanPath == "/api/assistant":
		return RoleViewer
	case cleanMethod == http.MethodPost && cleanPath == "/api/assistant/references/feedback":
		return RoleViewer
	case cleanMethod == http.MethodPost && cleanPath == "/api/clusters/select":
		return RoleViewer
	case cleanMethod == http.MethodPost && cleanPath == "/api/incidents":
		return RoleViewer
	case cleanMethod == http.MethodPost && cleanPath == "/api/remediation/propose":
		return RoleViewer
	case cleanMethod == http.MethodPost && strings.HasPrefix(cleanPath, "/api/remediation/") && strings.HasSuffix(cleanPath, "/gitops"):
		return RoleViewer
	case cleanMethod == http.MethodPost && cleanPath == "/api/risk-guard/analyze":
		return RoleViewer
	case cleanMethod == http.MethodPost && cleanPath == "/api/ghost/simulations":
		return RoleViewer
	case cleanMethod == http.MethodPost && strings.HasPrefix(cleanPath, "/api/remediation/") && strings.HasSuffix(cleanPath, "/reject"):
		return RoleViewer
	case cleanMethod == http.MethodPost && cleanPath == "/api/experimental/ebpf/nodes":
		return RoleOperator
	case cleanMethod == http.MethodPost && cleanPath == "/api/experimental/fleet-drift/propose":
		return RoleOperator
	case cleanMethod == http.MethodGet || cleanMethod == http.MethodHead:
		return RoleViewer
	case cleanMethod == http.MethodPost || cleanMethod == http.MethodPut || cleanMethod == http.MethodPatch || cleanMethod == http.MethodDelete:
		return RoleOperator
	default:
		return RoleViewer
	}
}

// RequiresWriteGate reports whether the route is blocked when write actions are disabled.
func RequiresWriteGate(method, path string) bool {
	cleanMethod := strings.ToUpper(strings.TrimSpace(method))
	if cleanMethod != http.MethodPost && cleanMethod != http.MethodPut && cleanMethod != http.MethodPatch && cleanMethod != http.MethodDelete {
		return false
	}

	cleanPath := strings.TrimSpace(path)
	switch {
	case cleanMethod == http.MethodPost && cleanPath == "/api/pods":
		return true
	case cleanMethod == http.MethodDelete && strings.HasPrefix(cleanPath, "/api/pods/"):
		return true
	case cleanMethod == http.MethodPost && strings.HasPrefix(cleanPath, "/api/pods/") && strings.HasSuffix(cleanPath, "/restart"):
		return true
	case cleanMethod == http.MethodPost && strings.HasPrefix(cleanPath, "/api/nodes/") && strings.HasSuffix(cleanPath, "/cordon"):
		return true
	case cleanMethod == http.MethodPost && strings.HasPrefix(cleanPath, "/api/nodes/") && strings.HasSuffix(cleanPath, "/uncordon"):
		return true
	case cleanMethod == http.MethodPost && strings.HasPrefix(cleanPath, "/api/nodes/") && strings.HasSuffix(cleanPath, "/drain"):
		return true
	case cleanMethod == http.MethodPut && strings.HasPrefix(cleanPath, "/api/resources/") && strings.HasSuffix(cleanPath, "/yaml"):
		return true
	case cleanMethod == http.MethodPost && strings.HasPrefix(cleanPath, "/api/resources/") && strings.HasSuffix(cleanPath, "/scale"):
		return true
	case cleanMethod == http.MethodPost && strings.HasPrefix(cleanPath, "/api/resources/") && strings.HasSuffix(cleanPath, "/restart"):
		return true
	case cleanMethod == http.MethodPost && strings.HasPrefix(cleanPath, "/api/resources/") && strings.HasSuffix(cleanPath, "/rollback"):
		return true
	case cleanMethod == http.MethodPost && strings.HasPrefix(cleanPath, "/api/remediation/") && strings.HasSuffix(cleanPath, "/execute"):
		return true
	case cleanMethod == http.MethodPost && cleanPath == "/api/experimental/autonomous-remediation/propose":
		return true
	default:
		return false
	}
}
