package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	storesql "kubelens-backend/internal/db"
	"kubelens-backend/internal/model"
	"kubelens-backend/internal/remediation"
)

func TestHealthzAndReadyzInMockMode(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(testClusterReader{}, nil, logger)
	router := server.Router("")

	healthReq := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	healthResp := httptest.NewRecorder()
	router.ServeHTTP(healthResp, healthReq)
	if healthResp.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", healthResp.Code)
	}

	readyReq := httptest.NewRequest(http.MethodGet, "/api/readyz", nil)
	readyResp := httptest.NewRecorder()
	router.ServeHTTP(readyResp, readyReq)
	if readyResp.Code != http.StatusOK {
		t.Fatalf("readyz status = %d, want 200", readyResp.Code)
	}

	var payload model.HealthStatus
	if err := json.NewDecoder(readyResp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode readyz payload: %v", err)
	}
	if payload.Status != "ok" {
		t.Fatalf("status = %q, want ok", payload.Status)
	}
}

func TestPredictorFailureIsVisibleInRuntimeAndReadyz(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(testClusterReader{}, nil, logger, WithPredictionsTTL(time.Minute))
	server.predictor = &flakyPredictionProvider{
		err: errors.New("predictor timeout"),
	}
	server.predictorHealth.enabled = true
	server.runtime.PredictorEnabled = true
	router := server.Router("")

	predictionsReq := httptest.NewRequest(http.MethodGet, "/api/predictions?force=1", nil)
	predictionsResp := httptest.NewRecorder()
	router.ServeHTTP(predictionsResp, predictionsReq)
	if predictionsResp.Code != http.StatusOK {
		t.Fatalf("predictions status = %d, want 200", predictionsResp.Code)
	}

	runtimeReq := httptest.NewRequest(http.MethodGet, "/api/runtime", nil)
	runtimeResp := httptest.NewRecorder()
	router.ServeHTTP(runtimeResp, runtimeReq)
	if runtimeResp.Code != http.StatusOK {
		t.Fatalf("runtime status = %d, want 200", runtimeResp.Code)
	}

	var runtimePayload model.RuntimeStatus
	if err := json.NewDecoder(runtimeResp.Body).Decode(&runtimePayload); err != nil {
		t.Fatalf("decode runtime payload: %v", err)
	}
	if runtimePayload.PredictorHealthy {
		t.Fatal("predictorHealthy should be false after predictor failure")
	}
	if !strings.Contains(runtimePayload.PredictorLastError, "predictor timeout") {
		t.Fatalf("predictorLastError = %q, want timeout message", runtimePayload.PredictorLastError)
	}

	readyReq := httptest.NewRequest(http.MethodGet, "/api/readyz", nil)
	readyResp := httptest.NewRecorder()
	router.ServeHTTP(readyResp, readyReq)
	if readyResp.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz status = %d, want 503", readyResp.Code)
	}
}

func TestEnterpriseReadinessRequiresDurableStorageInProd(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithRuntimeStatus(model.RuntimeStatus{
			Mode:                "prod",
			AuthEnabled:         true,
			WriteActionsEnabled: false,
			DatabaseDriver:      "sqlite",
			EnterpriseStorage:   false,
			PredictorHealthy:    true,
			PredictorMode:       "deterministic",
		}),
	)
	router := server.Router("")

	req := httptest.NewRequest(http.MethodGet, "/api/readiness/enterprise", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("enterprise readiness status = %d, want 503", rr.Code)
	}

	var payload model.HealthStatus
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode enterprise readiness payload: %v", err)
	}
	if payload.Status != "not-ready" {
		t.Fatalf("status = %q, want not-ready", payload.Status)
	}
	if !hasHealthCheck(payload.Checks, "storage", "prod-requires-durable-storage") {
		t.Fatalf("expected storage check to require durable storage: %+v", payload.Checks)
	}
}

func TestEnterpriseReadinessAcceptsDurableSQLiteInProd(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithRuntimeStatus(model.RuntimeStatus{
			Mode:                "prod",
			AuthEnabled:         true,
			WriteActionsEnabled: false,
			DatabaseDriver:      "sqlite",
			EnterpriseStorage:   true,
			PredictorHealthy:    true,
			PredictorMode:       "deterministic",
		}),
	)
	router := server.Router("")

	req := httptest.NewRequest(http.MethodGet, "/api/readiness/enterprise", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("enterprise readiness status = %d, want 200", rr.Code)
	}

	var payload model.HealthStatus
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode enterprise readiness payload: %v", err)
	}
	if !hasHealthCheck(payload.Checks, "storage", "sqlite-durable") {
		t.Fatalf("expected durable sqlite storage check: %+v", payload.Checks)
	}
}

func TestProductionReadinessReportsBlockers(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(testClusterReader{}, nil, logger)
	router := server.Router("")

	req := httptest.NewRequest(http.MethodGet, "/api/readiness/production", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("production readiness status = %d, want 503", rr.Code)
	}

	var payload model.ProductionReadinessStatus
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode production readiness payload: %v", err)
	}
	if payload.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", payload.Status)
	}
	if !hasProductionIssue(payload.Blockers, "mode") {
		t.Fatalf("expected mode blocker: %+v", payload.Blockers)
	}
	if !hasProductionIssue(payload.Blockers, "memory-store") {
		t.Fatalf("expected memory-store blocker: %+v", payload.Blockers)
	}
	if !hasProductionIssue(payload.Blockers, "audit-store") {
		t.Fatalf("expected audit-store blocker: %+v", payload.Blockers)
	}
}

func TestProductionReadinessAcceptsProductionPosture(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	db, err := storesql.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithRuntimeStatus(model.RuntimeStatus{
			Mode:                "prod",
			IsRealCluster:       true,
			AuthEnabled:         true,
			WriteActionsEnabled: true,
			DatabaseDriver:      "sqlite",
			DatabaseMigrations:  true,
			EnterpriseStorage:   true,
			MemoryStore:         "sql",
			MemoryDurable:       true,
			AuditStore:          "sql",
			AuditDurable:        true,
			AuditSigned:         true,
			PredictorEnabled:    true,
			PredictorHealthy:    true,
			PredictorMode:       "blended",
			GhostEnabled:        true,
			GhostHealthy:        true,
			AlertsEnabled:       true,
		}),
		WithAuditConfig(AuditConfig{
			MaxItems:   10,
			Store:      "sql",
			SigningKey: "audit-secret",
			SQLDB:      db,
			Dialect:    storesql.DialectSQLite,
		}),
	)
	server.predictorHealth.enabled = true
	server.ghostClient = healthyGhostClient{}
	server.ghostHealth.enabled = true
	router := server.Router("")

	req := httptest.NewRequest(http.MethodGet, "/api/readiness/production", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("production readiness status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	var payload model.ProductionReadinessStatus
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode production readiness payload: %v", err)
	}
	if payload.Status != "ready" {
		t.Fatalf("status = %q, want ready; blockers=%+v warnings=%+v", payload.Status, payload.Blockers, payload.Warnings)
	}
	if len(payload.Blockers) != 0 || len(payload.Warnings) != 0 {
		t.Fatalf("unexpected readiness issues: blockers=%+v warnings=%+v", payload.Blockers, payload.Warnings)
	}
	if !payload.Stores.MemoryDurable || !payload.Stores.AuditDurable || !payload.Stores.AuditSigned {
		t.Fatalf("unexpected store posture: %+v", payload.Stores)
	}
}

func TestPredictorModelHealthFallsBackWhenProviderUnavailable(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithRuntimeStatus(model.RuntimeStatus{
			PredictorEnabled:   true,
			PredictorHealthy:   false,
			PredictorMode:      "shadow",
			PredictorLastError: "predictor unavailable",
		}),
	)
	server.predictorHealth.enabled = true
	server.predictorHealth.lastFailure = time.Now()
	server.predictorHealth.lastError = "predictor unavailable"
	router := server.Router("")

	req := httptest.NewRequest(http.MethodGet, "/api/predictor/model", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("predictor model status = %d, want 200", rr.Code)
	}

	var payload model.PredictorModelHealth
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode predictor model health: %v", err)
	}
	if payload.Source != "backend" {
		t.Fatalf("source = %q, want backend", payload.Source)
	}
	if payload.Mode != "shadow" {
		t.Fatalf("mode = %q, want shadow", payload.Mode)
	}
	if payload.LastError != "predictor unavailable" {
		t.Fatalf("lastError = %q", payload.LastError)
	}
}

func TestExperimentalStatusDisabledByDefault(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(testClusterReader{}, nil, logger)
	router := server.Router("")

	req := httptest.NewRequest(http.MethodGet, "/api/experimental", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("experimental status = %d, want 200", rr.Code)
	}

	var payload model.ExperimentalStatus
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode experimental status: %v", err)
	}
	if len(payload.Features) != 3 {
		t.Fatalf("features = %d, want 3", len(payload.Features))
	}
	for _, feature := range payload.Features {
		if feature.Enabled {
			t.Fatalf("feature %s should be disabled by default", feature.Name)
		}
		if !feature.Experimental {
			t.Fatalf("feature %s should be marked experimental", feature.Name)
		}
	}
}

func TestExperimentalNodeTelemetryCompatibilityReport(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithExperimentalConfig(ExperimentalConfig{EBPFTelemetryEnabled: true}),
	)
	router := server.Router("")

	req := httptest.NewRequest(http.MethodGet, "/api/experimental/ebpf/nodes", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("node telemetry status = %d, want 200", rr.Code)
	}

	var payload model.NodeTelemetryReport
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode node telemetry: %v", err)
	}
	if !payload.Enabled || !payload.Experimental {
		t.Fatalf("expected enabled experimental telemetry report: %+v", payload)
	}
	if payload.AgentConnected {
		t.Fatal("compatibility report should not claim an eBPF agent is connected")
	}
	if len(payload.Nodes) == 0 {
		t.Fatal("expected node telemetry items")
	}
}

func TestExperimentalNodeTelemetryIngestRequiresEnabledGate(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithAuth(AuthConfig{
			Enabled: true,
			Tokens:  []AuthToken{{Token: "operator-token", User: "agent", Role: "operator"}},
		}),
	)
	router := server.Router("")

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/experimental/ebpf/nodes",
		bytes.NewBufferString(`{"agentId":"agent-a","nodes":[{"node":"node-a","status":"Ready","cpuUsage":"10%","memoryUsage":"20%","warningEvents":0,"pressureSignals":[],"observedWorkload":3}]}`),
	)
	req.Header.Set("Authorization", "Bearer operator-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("node telemetry ingest status = %d, want 403", rr.Code)
	}
}

func TestExperimentalNodeTelemetryIngestFeedsReport(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithAuth(AuthConfig{
			Enabled: true,
			Tokens:  []AuthToken{{Token: "operator-token", User: "agent", Role: "operator"}},
		}),
		WithExperimentalConfig(ExperimentalConfig{EBPFTelemetryEnabled: true}),
	)
	router := server.Router("")

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/experimental/ebpf/nodes",
		bytes.NewBufferString(`{"agentId":"agent-a","nodes":[{"node":"node-a","status":"NotReady","cpuUsage":"95%","memoryUsage":"91%","warningEvents":2,"pressureSignals":[],"observedWorkload":7}]}`),
	)
	req.Header.Set("Authorization", "Bearer operator-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("node telemetry ingest status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	var posted model.NodeTelemetryReport
	if err := json.NewDecoder(rr.Body).Decode(&posted); err != nil {
		t.Fatalf("decode posted node telemetry: %v", err)
	}
	if !posted.AgentConnected || posted.Source != "ebpf-agent" || posted.LastReceivedAt == "" {
		t.Fatalf("unexpected posted telemetry report: %+v", posted)
	}
	if len(posted.Nodes) != 1 || len(posted.Nodes[0].PressureSignals) < 3 {
		t.Fatalf("expected derived pressure signals, got: %+v", posted.Nodes)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/experimental/ebpf/nodes", nil)
	getReq.Header.Set("Authorization", "Bearer operator-token")
	getRR := httptest.NewRecorder()
	router.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("node telemetry report status = %d, want 200", getRR.Code)
	}

	var fetched model.NodeTelemetryReport
	if err := json.NewDecoder(getRR.Body).Decode(&fetched); err != nil {
		t.Fatalf("decode fetched node telemetry: %v", err)
	}
	if !fetched.AgentConnected || fetched.Source != "ebpf-agent" {
		t.Fatalf("GET should use recent agent telemetry: %+v", fetched)
	}
}

func TestProductionReadinessWarnsWhenEBPFTelemetryHasNoAgentData(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithExperimentalConfig(ExperimentalConfig{EBPFTelemetryEnabled: true}),
	)

	status := server.productionReadinessStatus(context.Background())
	found := false
	for _, warning := range status.Warnings {
		if warning.Key == "ebpf-telemetry" && warning.Message == "no-recent-agent-telemetry" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ebpf telemetry warning, got: %+v", status.Warnings)
	}
}

func TestExperimentalFleetDriftProposeCreatesReviewProposal(t *testing.T) {
	db, err := storesql.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	defaultReader := driftClusterReader{
		namespaces: []string{"production"},
		pods:       []model.PodSummary{{Name: "api", Namespace: "production", Status: model.PodStatusRunning}},
		nodes:      []model.NodeSummary{{Name: "node-a", Status: model.NodeStatusReady}},
	}
	stagingReader := driftClusterReader{
		namespaces: []string{"production", "payments"},
		pods: []model.PodSummary{
			{Name: "api", Namespace: "production", Status: model.PodStatusRunning},
			{Name: "worker", Namespace: "payments", Status: model.PodStatusRunning},
		},
		nodes: []model.NodeSummary{{Name: "node-a", Status: model.NodeStatusReady}},
	}
	server := newServer(
		defaultReader,
		nil,
		logger,
		WithAuth(AuthConfig{
			Enabled: true,
			Tokens:  []AuthToken{{Token: "operator-token", User: "operator", Role: "operator"}},
		}),
		WithRemediationStore(remediation.NewStore(db, remediation.DefaultStoreLimit, nil)),
		WithExperimentalConfig(ExperimentalConfig{FleetDriftEnabled: true}),
		WithClusterContexts(ClusterContextsConfig{
			Default: "prod",
			Readers: map[string]ClusterReader{
				"prod":    defaultReader,
				"staging": stagingReader,
			},
		}),
	)
	router := server.Router("")

	req := httptest.NewRequest(http.MethodPost, "/api/experimental/fleet-drift/propose", nil)
	req.Header.Set("Authorization", "Bearer operator-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("fleet drift propose status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	var payload model.FleetDriftProposalReport
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode fleet proposal report: %v", err)
	}
	if len(payload.Proposals) != 1 {
		t.Fatalf("proposals = %d, want 1: %+v", len(payload.Proposals), payload)
	}
	if payload.Proposals[0].Kind != model.RemediationKindFleetDriftReview {
		t.Fatalf("kind = %q, want fleet drift review", payload.Proposals[0].Kind)
	}
	if payload.Proposals[0].Resource != "staging" {
		t.Fatalf("resource = %q, want staging", payload.Proposals[0].Resource)
	}
}

func TestAutonomousRemediationPolicyBlocksWhenWriteGateDisabled(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithExperimentalConfig(ExperimentalConfig{AutonomousRemediationEnabled: true}),
	)
	router := server.Router("")

	req := httptest.NewRequest(http.MethodPost, "/api/experimental/autonomous-remediation/propose", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("autonomous remediation status = %d, want 403", rr.Code)
	}
}

func TestOpenAPISpecEndpoint(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(testClusterReader{}, nil, logger)
	router := server.Router("")

	req := httptest.NewRequest(http.MethodGet, "/api/openapi.yaml", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "openapi: 3.0.3") {
		t.Fatal("expected openapi version marker in spec payload")
	}
}

func hasHealthCheck(checks []model.HealthCheck, name string, message string) bool {
	for _, check := range checks {
		if check.Name == name && check.Message == message {
			return true
		}
	}
	return false
}

func TestPrometheusMetricsEndpoint(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(testClusterReader{}, nil, logger)
	router := server.Router("")

	// Generate some traffic before scraping.
	for _, path := range []string{"/api/pods", "/api/nodes", "/api/runtime"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, rr.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/metrics/prometheus", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "kubelens_http_requests_total") {
		t.Fatal("missing kubelens_http_requests_total metric")
	}
	if !strings.Contains(body, "kubelens_runtime_predictor_healthy") {
		t.Fatal("missing kubelens_runtime_predictor_healthy metric")
	}
	if !strings.Contains(body, "kubelens_runtime_memory_durable") {
		t.Fatal("missing kubelens_runtime_memory_durable metric")
	}
	if !strings.Contains(body, "kubelens_audit_sink_failures_total") {
		t.Fatal("missing kubelens_audit_sink_failures_total metric")
	}
}

type flakyPredictionProvider struct {
	err      error
	response model.PredictionsResult
}

func (f *flakyPredictionProvider) Predict(_ context.Context, _ predictorRequest) (model.PredictionsResult, error) {
	if f.err != nil {
		return model.PredictionsResult{}, f.err
	}
	return f.response, nil
}

type healthyGhostClient struct{}

func (healthyGhostClient) Simulate(
	context.Context,
	model.GhostSimulationRequest,
	model.GhostTopology,
) (model.GhostSimulationResult, error) {
	return model.GhostSimulationResult{}, nil
}

type driftClusterReader struct {
	testClusterReader
	pods       []model.PodSummary
	nodes      []model.NodeSummary
	namespaces []string
}

func (r driftClusterReader) Snapshot(context.Context) ([]model.PodSummary, []model.NodeSummary) {
	return append([]model.PodSummary(nil), r.pods...), append([]model.NodeSummary(nil), r.nodes...)
}

func (r driftClusterReader) ListNamespaces(context.Context) []string {
	return append([]string(nil), r.namespaces...)
}

func hasProductionIssue(issues []model.ProductionReadinessIssue, key string) bool {
	for _, issue := range issues {
		if issue.Key == key {
			return true
		}
	}
	return false
}
