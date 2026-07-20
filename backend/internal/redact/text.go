package redact

import (
	"regexp"
	"strings"
)

const replacement = "[redacted]"

var (
	bearerTokenPattern = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/\-=]+`)
	keyValuePattern    = regexp.MustCompile(`(?i)(\b(?:authorization|token|password|passwd|secret|api[_-]?key|access[_-]?key|refresh[_-]?token|x-predictor-secret)\b\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;{}]+)`)
	queryValuePattern  = regexp.MustCompile(`(?i)([?&](?:authorization|token|password|passwd|secret|api[_-]?key|access[_-]?key|refresh[_-]?token|x-predictor-secret)=)[^&\s]+`)
	longOpaquePattern  = regexp.MustCompile(`\b[A-Za-z0-9+/_=-]{32,}\b`)
)

// SensitiveText removes common secret shapes from logs and user-visible errors.
func SensitiveText(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	return longOpaquePattern.ReplaceAllString(
		queryValuePattern.ReplaceAllString(
			keyValuePattern.ReplaceAllString(
				bearerTokenPattern.ReplaceAllString(trimmed, "Bearer "+replacement),
				"${1}"+replacement,
			),
			"${1}"+replacement,
		),
		replacement,
	)
}

// Error returns a redacted error string.
func Error(err error) string {
	if err == nil {
		return ""
	}
	return SensitiveText(err.Error())
}
