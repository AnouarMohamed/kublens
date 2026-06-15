package config

import (
	"strings"

	"kubelens-backend/internal/model"
)

func RuntimeStatus(cfg Config, isRealCluster bool, alertsEnabled bool) model.RuntimeStatus {
	warnings := make([]string, 0, 2)
	if cfg.Mode != ModeProd {
		warnings = append(warnings, "Non-production mode: for development/demo use only.")
	}
	if cfg.DevMode {
		warnings = append(warnings, "DEV_MODE enabled: convenience shortcuts may reduce security guarantees.")
	}

	insecure := cfg.Mode != ModeProd || cfg.DevMode || !cfg.Auth.Enabled

	return model.RuntimeStatus{
		Mode:                string(cfg.Mode),
		DevMode:             cfg.DevMode,
		Insecure:            insecure,
		IsRealCluster:       isRealCluster,
		AuthEnabled:         cfg.Auth.Enabled,
		WriteActionsEnabled: cfg.WriteActionsEnabled,
		PredictorEnabled:    strings.TrimSpace(cfg.Predictor.BaseURL) != "",
		PredictorHealthy:    true,
		GhostEnabled:        cfg.Ghost.Enabled,
		GhostHealthy:        true,
		AssistantEnabled:    cfg.Assistant.Provider != "" && cfg.Assistant.Provider != "none",
		RAGEnabled:          cfg.Assistant.RAGEnabled,
		AlertsEnabled:       alertsEnabled,
		Warnings:            warnings,
	}
}

func anonymousPermissionsFor(cfg Config) []string {
	permissions := []string{"read", "assist", "stream"}
	if cfg.WriteActionsEnabled {
		permissions = append(permissions, "write")
	}
	return permissions
}
