package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

func parseMode(raw string) Mode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(ModeDev):
		return ModeDev
	case string(ModeProd):
		return ModeProd
	case string(ModeDemo):
		fallthrough
	default:
		return ModeDemo
	}
}

func profileForMode(mode Mode) profile {
	switch mode {
	case ModeDev:
		return profile{
			authEnabled:        false,
			rateLimitEnabled:   true,
			rateLimitRequests:  500,
			rateLimitWindowSec: 60,
			writeActions:       false,
			ragEnabled:         true,
		}
	case ModeProd:
		return profile{
			authEnabled:        true,
			rateLimitEnabled:   true,
			rateLimitRequests:  300,
			rateLimitWindowSec: 60,
			writeActions:       false,
			ragEnabled:         true,
		}
	default:
		return profile{
			authEnabled:        false,
			rateLimitEnabled:   true,
			rateLimitRequests:  300,
			rateLimitWindowSec: 60,
			writeActions:       false,
			ragEnabled:         true,
		}
	}
}

func parseClusterConfig() ClusterConfig {
	return ClusterConfig{
		KubeconfigData: strings.TrimSpace(os.Getenv("KUBECONFIG_DATA")),
		Contexts:       parseClusterContextMap(strings.TrimSpace(os.Getenv("KUBECONFIG_CONTEXTS"))),
	}
}

func parseClusterContextMap(raw string) map[string]string {
	if raw == "" {
		return map[string]string{}
	}

	entries := strings.Split(raw, ",")
	contexts := make(map[string]string, len(entries))
	for _, entry := range entries {
		item := strings.TrimSpace(entry)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		encoded := strings.TrimSpace(parts[1])
		if name == "" || encoded == "" {
			continue
		}
		contexts[name] = encoded
	}
	return contexts
}

func parsePort(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 || value > 65535 {
		return 3000
	}
	return value
}

func parseSecondsAsDuration(raw string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func parseHoursAsDuration(raw string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	hours, err := strconv.Atoi(value)
	if err != nil || hours <= 0 {
		return fallback
	}
	return time.Duration(hours) * time.Hour
}

func parseFloatDefault(raw string, fallback float64) float64 {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseIntDefault(raw string, fallback int) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseBoolDefault(raw string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return fallback
	}

	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseCSV(raw string) []string {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(strings.ToLower(part))
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func parseAuthTokens(raw string) []AuthToken {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	entries := strings.Split(trimmed, ",")
	out := make([]AuthToken, 0, len(entries))
	for _, entry := range entries {
		item := strings.TrimSpace(entry)
		if item == "" {
			continue
		}
		parts := strings.Split(item, ":")
		if len(parts) != 3 {
			continue
		}
		user := strings.TrimSpace(parts[0])
		role := strings.TrimSpace(parts[1])
		token := strings.TrimSpace(parts[2])
		if user == "" || role == "" || token == "" {
			continue
		}
		out = append(out, AuthToken{Token: token, User: user, Role: role})
	}
	return out
}

func defaultIfEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
