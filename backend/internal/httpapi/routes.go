// Package httpapi exposes the HTTP transport for KubeLens backend services.
package httpapi

import "strings"

const (
	apiMountPrefix = "/api"

	apiAuthLoginPath   = apiMountPrefix + "/auth/login"
	apiAuthLogoutPath  = apiMountPrefix + "/auth/logout"
	apiAuthSessionPath = apiMountPrefix + "/auth/session"

	apiHealthzPath         = apiMountPrefix + "/healthz"
	apiReadyzPath          = apiMountPrefix + "/readyz"
	apiOpenAPIPath         = apiMountPrefix + "/openapi.yaml"
	apiStreamPrefix        = apiMountPrefix + "/stream"
	apiPodLogsStreamSuffix = "/logs/stream"
)

// isAPIPath reports whether a request path targets the API mount.
func isAPIPath(path string) bool {
	trimmed := strings.TrimSpace(path)
	return trimmed == apiMountPrefix || strings.HasPrefix(trimmed, apiMountPrefix+"/")
}
