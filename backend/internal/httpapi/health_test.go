package httpapi

import (
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

	"kubelens-backend/internal/model"
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

func TestEnterpriseReadinessRequiresPostgresInProd(t *testing.T) {
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
	if !hasHealthCheck(payload.Checks, "storage", "prod-requires-postgres") {
		t.Fatalf("expected storage check to require postgres: %+v", payload.Checks)
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
