package httpapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kubelens-backend/internal/model"
)

const (
	productionReadinessReady    = "ready"
	productionReadinessDegraded = "degraded"
	productionReadinessBlocked  = "blocked"
)

func (s *Server) productionReadinessStatus(ctx context.Context) model.ProductionReadinessStatus {
	runtime := s.runtimeSnapshot()
	audit := s.audit.posture()
	cluster := s.clusterReadinessCheck(ctx)
	predictor := s.predictorReadinessCheck()
	ghost := s.ghostReadinessCheck()

	stores := model.ProductionStorePosture{
		DatabaseDriver:     strings.TrimSpace(runtime.DatabaseDriver),
		EnterpriseStorage:  runtime.EnterpriseStorage,
		MigrationsEnabled:  runtime.DatabaseMigrations,
		MemoryStore:        strings.TrimSpace(runtime.MemoryStore),
		MemoryDurable:      runtime.MemoryDurable,
		AuditStore:         firstNonEmptyString(audit.Store, runtime.AuditStore),
		AuditDurable:       audit.Durable,
		AuditSigned:        audit.Signed,
		AuditSinkFailures:  audit.Failures,
		AuditSinkLastError: audit.LastError,
	}
	if stores.DatabaseDriver == "" {
		stores.DatabaseDriver = "unknown"
	}
	if stores.MemoryStore == "" {
		stores.MemoryStore = "unknown"
	}
	if stores.AuditStore == "" {
		stores.AuditStore = "unavailable"
	}

	checks := make([]model.ProductionReadinessCheck, 0, 13)
	blockers := make([]model.ProductionReadinessIssue, 0, 8)
	warnings := make([]model.ProductionReadinessIssue, 0, 6)

	addCheck := func(name string, ok bool, severity string, message string, recommendation string) {
		checks = append(checks, model.ProductionReadinessCheck{
			Name:     name,
			OK:       ok,
			Severity: severity,
			Message:  message,
		})
		if ok {
			return
		}
		issue := model.ProductionReadinessIssue{
			Key:            name,
			Severity:       severity,
			Message:        message,
			Recommendation: recommendation,
		}
		if severity == "blocker" {
			blockers = append(blockers, issue)
			return
		}
		warnings = append(warnings, issue)
	}

	addCheck(
		"mode",
		runtime.Mode == "prod",
		"blocker",
		boolMessage(runtime.Mode == "prod", "prod", "not-prod"),
		"Set APP_MODE=prod for production deployments.",
	)
	addCheck(
		"cluster",
		runtime.IsRealCluster && cluster.OK,
		"blocker",
		productionClusterMessage(runtime, cluster),
		"Provide a production kubeconfig and verify the API server is reachable.",
	)
	addCheck(
		"auth",
		runtime.AuthEnabled,
		"blocker",
		boolMessage(runtime.AuthEnabled, "enabled", "disabled"),
		"Enable AUTH_ENABLED and configure static tokens or OIDC.",
	)
	addCheck(
		"write-gate",
		!runtime.WriteActionsEnabled || runtime.AuthEnabled,
		"blocker",
		boolMessage(!runtime.WriteActionsEnabled || runtime.AuthEnabled, "guarded", "writes-require-auth"),
		"Keep write actions behind authenticated operator or admin sessions.",
	)
	addCheck(
		"database",
		stores.EnterpriseStorage,
		"blocker",
		enterpriseStorageMessage(runtime.Mode, stores.DatabaseDriver, stores.EnterpriseStorage),
		"Use Postgres or durable file-backed SQLite storage.",
	)
	addCheck(
		"database-migrations",
		stores.MigrationsEnabled,
		"blocker",
		boolMessage(stores.MigrationsEnabled, "auto", "disabled"),
		"Enable DATABASE_MIGRATIONS_AUTO=true or run migrations before serving traffic.",
	)
	addCheck(
		"memory-store",
		stores.MemoryDurable && stores.MemoryStore == "sql",
		"blocker",
		productionMemoryMessage(stores),
		"Set MEMORY_STORE=sql so runbooks and fix patterns survive restarts.",
	)
	addCheck(
		"audit-store",
		stores.AuditDurable && stores.AuditSinkFailures == 0,
		"blocker",
		productionAuditMessage(stores),
		"Set AUDIT_STORE=sql and confirm audit_entries writes are succeeding.",
	)
	addCheck(
		"audit-signing",
		stores.AuditSigned,
		"blocker",
		boolMessage(stores.AuditSigned, "signed", "unsigned"),
		"Set AUDIT_SIGNING_KEY for tamper-evident audit verification.",
	)
	addCheck(
		"predictor",
		runtime.PredictorEnabled && predictor.OK,
		"warning",
		productionDependencyMessage(runtime.PredictorEnabled, predictor),
		"Configure PREDICTOR_BASE_URL and monitor /api/predictor/model before relying on ML risk signals.",
	)
	addCheck(
		"ghost",
		runtime.GhostEnabled && ghost.OK && ghost.Message == "healthy",
		"warning",
		productionDependencyMessage(runtime.GhostEnabled, ghost),
		"Run the Ghost engine and configure GHOST_ENGINE_ADDR for production-grade simulations.",
	)
	addCheck(
		"alerts",
		runtime.AlertsEnabled,
		"warning",
		boolMessage(runtime.AlertsEnabled, "enabled", "disabled"),
		"Configure Alertmanager, Slack, or PagerDuty routing for incident notifications.",
	)
	addCheck(
		"experimental-features",
		!s.experimental.EBPFTelemetryEnabled &&
			!s.experimental.FleetDriftEnabled &&
			!s.experimental.AutonomousRemediationEnabled,
		"warning",
		productionExperimentalMessage(s.experimental),
		"Keep experimental features disabled unless they have an explicit rollout and rollback plan.",
	)
	if s.experimental.EBPFTelemetryEnabled {
		telemetryReport, agentConnected := s.nodeTelemetryReportFromAgents()
		message := "agent-connected"
		if !agentConnected {
			message = "no-recent-agent-telemetry"
		} else if strings.TrimSpace(telemetryReport.LastReceivedAt) != "" {
			message = "last-received " + telemetryReport.LastReceivedAt
		}
		addCheck(
			"ebpf-telemetry",
			agentConnected,
			"warning",
			message,
			"Verify node agents are deployed, authenticated, and posting telemetry before relying on deep telemetry signals.",
		)
	}

	status := productionReadinessReady
	if len(blockers) > 0 {
		status = productionReadinessBlocked
	} else if len(warnings) > 0 {
		status = productionReadinessDegraded
	}

	return model.ProductionReadinessStatus{
		Status:      status,
		GeneratedAt: s.now().UTC().Format(time.RFC3339),
		Summary:     productionReadinessSummary(status, len(blockers), len(warnings)),
		Mode:        runtime.Mode,
		Blockers:    blockers,
		Warnings:    warnings,
		Checks:      checks,
		Stores:      stores,
		Dependencies: model.ProductionDependencyPosture{
			Cluster: productionDependencyStatus(runtime.IsRealCluster, runtime.IsRealCluster && cluster.OK, cluster),
			Predictor: productionDependencyStatus(
				runtime.PredictorEnabled,
				runtime.PredictorEnabled && predictor.OK,
				predictor,
			),
			Ghost: productionDependencyStatus(
				runtime.GhostEnabled,
				runtime.GhostEnabled && ghost.OK && ghost.Message == "healthy",
				ghost,
			),
			Alerts: model.ProductionDependencyStatus{
				Enabled: runtime.AlertsEnabled,
				Healthy: runtime.AlertsEnabled,
				Message: boolMessage(runtime.AlertsEnabled, "enabled", "disabled"),
			},
		},
		Runbooks: []model.ProductionRunbookLink{
			{Title: "Production readiness", Path: "/docs/runbooks/production-readiness.md"},
			{Title: "Backup and restore", Path: "/docs/runbooks/backup-restore.md"},
			{Title: "Failed migration", Path: "/docs/runbooks/failed-migration.md"},
			{Title: "Write gate and rollback", Path: "/docs/runbooks/write-gate.md"},
		},
		Build: s.buildInfo,
	}
}

func productionClusterMessage(runtime model.RuntimeStatus, check model.HealthCheck) string {
	if !runtime.IsRealCluster {
		return "mock-mode"
	}
	return check.Message
}

func productionMemoryMessage(stores model.ProductionStorePosture) string {
	if stores.MemoryDurable && stores.MemoryStore == "sql" {
		return "sql-durable"
	}
	if stores.MemoryStore == "" || stores.MemoryStore == "unknown" {
		return "memory-store-unknown"
	}
	return stores.MemoryStore + "-not-durable"
}

func productionAuditMessage(stores model.ProductionStorePosture) string {
	if stores.AuditSinkFailures > 0 {
		if strings.TrimSpace(stores.AuditSinkLastError) != "" {
			return "sink-failing: " + stores.AuditSinkLastError
		}
		return "sink-failing"
	}
	if stores.AuditDurable {
		return stores.AuditStore + "-durable"
	}
	if stores.AuditStore == "" {
		return "audit-store-unavailable"
	}
	return stores.AuditStore + "-not-durable"
}

func productionDependencyMessage(enabled bool, check model.HealthCheck) string {
	if strings.TrimSpace(check.Message) != "" {
		return check.Message
	}
	if !enabled {
		return "disabled"
	}
	return boolMessage(check.OK, "healthy", "unavailable")
}

func productionExperimentalMessage(config ExperimentalConfig) string {
	enabled := make([]string, 0, 3)
	if config.EBPFTelemetryEnabled {
		enabled = append(enabled, "ebpf-telemetry")
	}
	if config.FleetDriftEnabled {
		enabled = append(enabled, "fleet-drift")
	}
	if config.AutonomousRemediationEnabled {
		enabled = append(enabled, "autonomous-remediation")
	}
	if len(enabled) == 0 {
		return "disabled"
	}
	return "enabled: " + strings.Join(enabled, ",")
}

func productionReadinessSummary(status string, blockers int, warnings int) string {
	switch status {
	case productionReadinessBlocked:
		return fmt.Sprintf("%s before production rollout.", pluralize(blockers, "blocker"))
	case productionReadinessDegraded:
		return fmt.Sprintf("Production baseline met with %s.", pluralize(warnings, "warning"))
	default:
		return "Production baseline met."
	}
}

func productionDependencyStatus(
	enabled bool,
	healthy bool,
	check model.HealthCheck,
) model.ProductionDependencyStatus {
	return model.ProductionDependencyStatus{
		Enabled:     enabled,
		Healthy:     healthy,
		Message:     productionDependencyMessage(enabled, check),
		LastSuccess: check.LastSuccess,
		LastFailure: check.LastFailure,
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func pluralize(count int, singular string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %ss", count, singular)
}
