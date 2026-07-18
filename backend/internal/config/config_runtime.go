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
	if cfg.Mode == ModeProd && cfg.Database.Driver == "sqlite" && !sqliteStorageDurable(cfg.Database.SQLitePath) {
		warnings = append(warnings, "Production mode requires a durable DB_PATH for SQLite storage.")
	}
	if cfg.Database.Driver == "postgres" && strings.TrimSpace(cfg.Database.URL) == "" {
		warnings = append(warnings, "Postgres storage requires DATABASE_URL.")
	}
	if cfg.Mode == ModeProd && cfg.Memory.Store != "sql" {
		warnings = append(warnings, "Production mode should use MEMORY_STORE=sql for durable team memory.")
	}
	if cfg.Mode == ModeProd && cfg.Audit.Store != "sql" {
		warnings = append(warnings, "Production mode should use AUDIT_STORE=sql for durable audit records.")
	}
	if cfg.Mode == ModeProd && strings.TrimSpace(cfg.Audit.SigningKey) == "" {
		warnings = append(warnings, "Production mode should configure AUDIT_SIGNING_KEY for tamper-evident audit signatures.")
	}
	if cfg.Predictor.Mode == "shadow" {
		warnings = append(warnings, "Predictor shadow mode emits ML scores without changing final risk.")
	}

	insecure := cfg.Mode != ModeProd || cfg.DevMode || !cfg.Auth.Enabled

	return model.RuntimeStatus{
		Mode:                string(cfg.Mode),
		DevMode:             cfg.DevMode,
		Insecure:            insecure,
		IsRealCluster:       isRealCluster,
		AuthEnabled:         cfg.Auth.Enabled,
		WriteActionsEnabled: cfg.WriteActionsEnabled,
		DatabaseDriver:      cfg.Database.Driver,
		DatabaseMigrations:  cfg.Database.MigrationsAuto,
		EnterpriseStorage: cfg.Database.Driver == "postgres" ||
			(cfg.Database.Driver == "sqlite" && sqliteStorageDurable(cfg.Database.SQLitePath)),
		MemoryStore:      cfg.Memory.Store,
		MemoryDurable:    cfg.Memory.Store == "sql",
		AuditStore:       cfg.Audit.Store,
		AuditDurable:     cfg.Audit.Store == "sql" || cfg.Audit.Store == "file",
		AuditSigned:      strings.TrimSpace(cfg.Audit.SigningKey) != "",
		PredictorEnabled: strings.TrimSpace(cfg.Predictor.BaseURL) != "",
		PredictorHealthy: true,
		PredictorMode:    cfg.Predictor.Mode,
		GhostEnabled:     cfg.Ghost.Enabled,
		GhostHealthy:     true,
		AssistantEnabled: cfg.Assistant.Provider != "" && cfg.Assistant.Provider != "none",
		RAGEnabled:       cfg.Assistant.RAGEnabled,
		AlertsEnabled:    alertsEnabled,
		Warnings:         warnings,
	}
}

func anonymousPermissionsFor(cfg Config) []string {
	permissions := []string{"read", "assist", "stream"}
	if cfg.WriteActionsEnabled {
		permissions = append(permissions, "write")
	}
	return permissions
}
