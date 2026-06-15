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
	api.Get("/openapi.yaml", s.handleOpenAPIYAML)
	api.Get("/version", s.handleVersion)
	api.Get("/runtime", s.handleRuntime)

	api.Get("/auth/session", s.handleAuthSession)
	api.Post("/auth/login", s.handleAuthLogin)
	api.Post("/auth/logout", s.handleAuthLogout)

	api.Get("/clusters", s.handleClusters)
	api.Post("/clusters/select", s.handleSelectCluster)
}

func (s *Server) mountClusterRoutes(api chi.Router) {
	api.Get("/cluster-info", s.handleClusterInfo)
	api.Get("/namespaces", s.handleNamespaces)
	api.Get("/pods", s.handlePods)
	api.Post("/pods", s.handleCreatePod)
	api.Get("/nodes", s.handleNodes)
	api.Get("/events", s.handleEvents)
	api.Get("/stats", s.handleStats)
	api.Get("/diagnostics", s.handleDiagnostics)

	api.Get("/resources/{kind}", s.handleResources)
	api.Get("/resources/{kind}/{namespace}/{name}/yaml", s.handleGetResourceYAML)
	api.Put("/resources/{kind}/{namespace}/{name}/yaml", s.handleApplyResourceYAML)
	api.Post("/resources/{kind}/{namespace}/{name}/scale", s.handleScaleResource)
	api.Post("/resources/{kind}/{namespace}/{name}/restart", s.handleRestartResource)
	api.Post("/resources/{kind}/{namespace}/{name}/rollback", s.handleRollbackResource)

	api.Get("/pods/{namespace}/{name}/events", s.handlePodEvents)
	api.Get("/pods/{namespace}/{name}/logs", s.handlePodLogs)
	api.Get("/pods/{namespace}/{name}/logs/stream", s.handlePodLogsStream)
	api.Get("/pods/{namespace}/{name}/describe", s.handlePodDescribe)
	api.Post("/pods/{namespace}/{name}/restart", s.handleRestartPod)
	api.Delete("/pods/{namespace}/{name}", s.handleDeletePod)
	api.Get("/pods/{namespace}/{name}", s.handlePodDetail)

	api.Post("/nodes/{name}/cordon", s.handleCordonNode)
	api.Post("/nodes/{name}/uncordon", s.handleUncordonNode)
	api.Get("/nodes/{name}/drain/preview", s.handleNodeDrainPreview)
	api.Post("/nodes/{name}/drain", s.handleDrainNode)
	api.Get("/nodes/{name}/pods", s.handleNodePods)
	api.Get("/nodes/{name}/events", s.handleNodeEvents)
	api.Get("/nodes/{name}", s.handleNodeDetail)
}

func (s *Server) mountObservabilityRoutes(api chi.Router) {
	api.Get("/metrics", s.handleMetrics)
	api.Get("/metrics/prometheus", s.handlePrometheusMetrics)
	api.Get("/slo", s.handleSLOOverview)
	api.Get("/rightsizing", s.handleRightsizingOverview)
	api.Get("/audit", s.handleAuditLog)
	api.Get("/stream", s.handleStream)
	api.Get("/stream/ws", s.handleStreamWebSocket)
	api.Get("/predictions", s.handlePredictions)
	api.Get("/predictive-incidents", s.handlePredictions) // Backward-compatible alias for older frontend builds.
	api.Get("/ghost/topology", s.handleGhostTopology)
	api.Post("/ghost/simulations", s.handleGhostSimulation)

	api.Post("/alerts/dispatch", s.handleAlertDispatch)
	api.Post("/alerts/test", s.handleAlertTest)
	api.Get("/alerts/lifecycle", s.handleListAlertLifecycle)
	api.Post("/alerts/lifecycle", s.handleUpsertAlertLifecycle)
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
