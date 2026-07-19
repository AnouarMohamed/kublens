package httpapi

import "github.com/go-chi/chi/v5"

func (s *Server) mountAPIRoutes(api chi.Router) {
	s.mountSystemRoutes(api)
	s.mountClusterRoutes(api)
	s.mountObservabilityRoutes(api)
	s.mountOpsRoutes(api)
}

func (s *Server) mountSystemRoutes(api chi.Router) {
	api.Get("/healthz", s.handleHealthz)
	api.Get("/readyz", s.handleReadyz)
	api.Get("/readiness/enterprise", s.handleEnterpriseReadyz)
	api.Get("/readiness/production", s.handleProductionReadyz)
	api.Get("/openapi.yaml", s.handleOpenAPIYAML)
	api.Get("/version", s.handleVersion)
	api.Get("/runtime", s.handleRuntime)
	api.Get("/experimental", s.handleExperimentalStatus)
	api.Get("/experimental/ebpf/nodes", s.handleExperimentalNodeTelemetry)
	api.Get("/experimental/fleet-drift", s.handleExperimentalFleetDrift)
	api.Post("/experimental/autonomous-remediation/propose", s.handleAutonomousRemediationPropose)

	api.Get("/auth/session", s.handleAuthSession)
	api.Post("/auth/login", s.handleAuthLogin)
	api.Post("/auth/logout", s.handleAuthLogout)

	api.Get("/clusters", s.handleClusters)
	api.Post("/clusters/select", s.handleSelectCluster)
}

func (s *Server) mountClusterRoutes(api chi.Router) {
	api.Get("/cluster-info", s.handleClusterInfo)
	api.Get("/namespaces", s.handleNamespaces)
	api.Get("/events", s.handleEvents)
	api.Get("/stats", s.handleStats)
	api.Get("/diagnostics", s.handleDiagnostics)

	api.Mount("/pods", NewPodController(s.cluster, s.logger, s.decodeJSONBody, s.invalidatePredictionsCache).Routes())
	api.Mount("/nodes", NewNodeController(s.cluster, s.audit, s.now, s.decodeJSONBody, s.invalidatePredictionsCache).Routes())
	api.Mount("/resources", NewResourceController(
		s.cluster,
		s.audit,
		s.now,
		s.decodeJSONBody,
		s.evaluateManifestRisk,
		s.invalidatePredictionsCache,
	).Routes())
}

func (s *Server) mountObservabilityRoutes(api chi.Router) {
	api.Mount("/metrics", NewMetricsController(s.metrics, s.docs, s.runtimeSnapshot, s.audit.posture).Routes())
	api.Mount("/slo", NewSLOController(s.metrics, s.incidents, s.currentClusterStats, s.now).Routes())
	api.Mount("/rightsizing", NewRightsizingController(s.cluster, s.now).Routes())
	api.Mount("/audit", NewAuditController(s.audit).Routes())
	api.Mount("/stream", NewStreamController(
		s.cluster,
		s.eventBus,
		s.now,
		s.currentClusterStats,
		s.auth.trustedCSRFDomains,
	).Routes())
	api.Mount("/ghost", NewGhostController(
		s.cluster,
		s.ghostClient,
		s.ghostRuns,
		s.logger,
		s.now,
		s.decodeJSONBody,
		s.recordGhostSuccess,
		s.recordGhostFailure,
	).Routes())

	predictions := NewPredictionController(
		s.cluster,
		s.predictor,
		s.logger,
		s.now,
		s.predictionsFromCache,
		s.storePredictions,
		s.recordPredictorSuccess,
		s.recordPredictorFailure,
	)
	api.Get("/predictions", predictions.handlePredictions)
	api.Get("/predictor/model", s.handlePredictorModelHealth)
	api.Get("/predictive-incidents", predictions.handlePredictions) // Backward-compatible alias for older frontend builds.

	api.Mount("/alerts", NewAlertController(s.alerts, s.alertLifecycle, s.decodeJSONBody).Routes())
}

func (s *Server) mountOpsRoutes(api chi.Router) {
	api.Post("/assistant", s.handleAssistant)
	api.Post("/assistant/references/feedback", s.handleAssistantReferenceFeedback)
	api.Get("/rag/telemetry", s.handleRAGTelemetry)

	api.Post("/incidents", s.handleCreateIncident)
	api.Get("/incidents", s.handleListIncidents)
	api.Get("/incidents/{id}", s.handleGetIncident)
	api.Get("/incidents/{id}/replay", s.handleGetIncidentReplay)
	api.Get("/incidents/{id}/evidence", s.handleGetIncidentEvidence)
	api.Patch("/incidents/{id}/steps/{step}", s.handlePatchIncidentStep)
	api.Post("/incidents/{id}/resolve", s.handleResolveIncident)
	api.Post("/incidents/{id}/postmortem", s.handleGeneratePostmortem)

	api.Get("/postmortems", s.handleListPostmortems)
	api.Get("/postmortems/{id}", s.handleGetPostmortem)

	api.Post("/remediation/propose", s.handleProposeRemediation)
	api.Get("/remediation", s.handleListRemediation)
	api.Get("/remediation/{id}/gitops", s.handleGetRemediationGitOps)
	api.Post("/remediation/{id}/gitops", s.handleGenerateRemediationGitOps)
	api.Post("/remediation/{id}/approve", s.handleApproveRemediation)
	api.Post("/remediation/{id}/execute", s.handleExecuteRemediation)
	api.Post("/remediation/{id}/reject", s.handleRejectRemediation)

	api.Get("/memory/runbooks", s.handleMemoryRunbooks)
	api.Post("/memory/runbooks", s.handleCreateMemoryRunbook)
	api.Put("/memory/runbooks/{id}", s.handleUpdateMemoryRunbook)
	api.Get("/memory/fixes", s.handleListMemoryFixes)
	api.Post("/memory/fixes", s.handleRecordMemoryFix)

	api.Post("/risk-guard/analyze", s.handleAnalyzeRiskGuard)
}
