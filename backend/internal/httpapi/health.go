package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"kubelens-backend/internal/model"
	"kubelens-backend/internal/redact"
)

const readinessClusterTimeout = 2 * time.Second

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"timestamp": s.now().UTC().Format(time.RFC3339),
		"version":   s.buildInfo.Version,
		"commit":    s.buildInfo.Commit,
	})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	checks := make([]model.HealthCheck, 0, 3)
	overallOK := true

	clusterCheck := s.clusterReadinessCheck(r.Context())
	checks = append(checks, clusterCheck)
	if !clusterCheck.OK {
		overallOK = false
	}

	predictorCheck := s.predictorReadinessCheck()
	checks = append(checks, predictorCheck)
	if !predictorCheck.OK {
		overallOK = false
	}

	authOK := !(s.runtime.Mode == "prod" && !s.runtime.AuthEnabled)
	authMessage := "configured"
	if !authOK {
		authMessage = "prod-requires-auth"
	}
	authCheck := model.HealthCheck{
		Name:    "auth",
		OK:      authOK,
		Message: authMessage,
	}
	checks = append(checks, authCheck)

	status := "ok"
	httpStatus := http.StatusOK
	if !overallOK {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
		s.logger.WarnContext(r.Context(), "readiness_degraded",
			"request_id", middleware.GetReqID(r.Context()),
			"cluster_ok", clusterCheck.OK,
			"predictor_ok", predictorCheck.OK,
			"predictor_message", predictorCheck.Message,
		)
	}

	writeJSON(w, httpStatus, model.HealthStatus{
		Status:    status,
		Timestamp: s.now().UTC().Format(time.RFC3339),
		Checks:    checks,
		Build:     s.buildInfo,
	})
}

func (s *Server) handleEnterpriseReadyz(w http.ResponseWriter, r *http.Request) {
	checks := make([]model.HealthCheck, 0, 6)
	overallOK := true
	appendCheck := func(check model.HealthCheck) {
		checks = append(checks, check)
		if !check.OK {
			overallOK = false
		}
	}

	appendCheck(s.clusterReadinessCheck(r.Context()))
	appendCheck(s.predictorReadinessCheck())
	appendCheck(model.HealthCheck{
		Name:    "auth",
		OK:      s.runtime.Mode != "prod" || s.runtime.AuthEnabled,
		Message: boolMessage(s.runtime.Mode != "prod" || s.runtime.AuthEnabled, "configured", "prod-requires-auth"),
	})
	appendCheck(model.HealthCheck{
		Name:    "write-gate",
		OK:      !s.runtime.WriteActionsEnabled || s.runtime.AuthEnabled,
		Message: boolMessage(!s.runtime.WriteActionsEnabled || s.runtime.AuthEnabled, "guarded", "writes-require-auth"),
	})
	appendCheck(model.HealthCheck{
		Name:    "storage",
		OK:      s.runtime.EnterpriseStorage,
		Message: enterpriseStorageMessage(s.runtime.Mode, s.runtime.DatabaseDriver, s.runtime.EnterpriseStorage),
	})
	appendCheck(model.HealthCheck{
		Name:    "audit",
		OK:      s.audit != nil,
		Message: boolMessage(s.audit != nil, "available", "unavailable"),
	})

	status := "ok"
	httpStatus := http.StatusOK
	if !overallOK {
		status = "not-ready"
		httpStatus = http.StatusServiceUnavailable
	}

	writeJSON(w, httpStatus, model.HealthStatus{
		Status:    status,
		Timestamp: s.now().UTC().Format(time.RFC3339),
		Checks:    checks,
		Build:     s.buildInfo,
	})
}

func (s *Server) handleProductionReadyz(w http.ResponseWriter, r *http.Request) {
	status := s.productionReadinessStatus(r.Context())
	httpStatus := http.StatusOK
	if len(status.Blockers) > 0 {
		httpStatus = http.StatusServiceUnavailable
	}
	writeJSON(w, httpStatus, status)
}

func (s *Server) clusterReadinessCheck(parent context.Context) model.HealthCheck {
	if !s.cluster.IsRealCluster() {
		return model.HealthCheck{
			Name:    "cluster",
			OK:      true,
			Message: "mock-mode",
		}
	}

	ctx, cancel := context.WithTimeout(parent, readinessClusterTimeout)
	defer cancel()

	_, _ = s.cluster.Snapshot(ctx)
	if ctx.Err() == nil {
		return model.HealthCheck{
			Name:    "cluster",
			OK:      true,
			Message: "reachable",
		}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return model.HealthCheck{
			Name:    "cluster",
			OK:      false,
			Message: "timeout",
		}
	}
	return model.HealthCheck{
		Name:    "cluster",
		OK:      false,
		Message: "cancelled",
	}
}

func (s *Server) ghostReadinessCheck() model.HealthCheck {
	runtime := s.runtimeSnapshot()
	if !runtime.GhostEnabled {
		return model.HealthCheck{
			Name:    "ghost",
			OK:      true,
			Message: "disabled",
		}
	}
	if s.ghostClient == nil {
		return model.HealthCheck{
			Name:    "ghost",
			OK:      true,
			Message: "fallback-engine",
		}
	}

	state := s.ghostHealthSnapshot()
	check := model.HealthCheck{
		Name:    "ghost",
		OK:      true,
		Message: "healthy",
	}
	if !state.lastSuccess.IsZero() {
		check.LastSuccess = state.lastSuccess.UTC().Format(time.RFC3339)
	}
	if !state.lastFailure.IsZero() {
		check.LastFailure = state.lastFailure.UTC().Format(time.RFC3339)
	}
	if state.lastFailure.After(state.lastSuccess) {
		check.OK = false
		if state.lastError != "" {
			check.Message = state.lastError
		} else {
			check.Message = "unavailable"
		}
	}
	return check
}

func (s *Server) predictorReadinessCheck() model.HealthCheck {
	state := s.predictorHealthSnapshot()
	if !state.enabled {
		return model.HealthCheck{
			Name:    "predictor",
			OK:      true,
			Message: "disabled",
		}
	}

	check := model.HealthCheck{
		Name:    "predictor",
		OK:      true,
		Message: "healthy",
	}
	if !state.lastSuccess.IsZero() {
		check.LastSuccess = state.lastSuccess.UTC().Format(time.RFC3339)
	}
	if !state.lastFailure.IsZero() {
		check.LastFailure = state.lastFailure.UTC().Format(time.RFC3339)
	}
	if state.lastFailure.After(state.lastSuccess) {
		check.OK = false
		if state.lastError != "" {
			check.Message = state.lastError
		} else {
			check.Message = "unavailable"
		}
	}
	return check
}

func (s *Server) runtimeSnapshot() model.RuntimeStatus {
	runtime := s.runtime
	predictorState := s.predictorHealthSnapshot()
	if !runtime.PredictorEnabled {
		runtime.PredictorHealthy = true
		runtime.PredictorLastError = ""
	} else {
		runtime.PredictorHealthy = !predictorState.lastFailure.After(predictorState.lastSuccess)
		runtime.PredictorLastError = redact.SensitiveText(predictorState.lastError)
	}

	ghostState := s.ghostHealthSnapshot()
	if !runtime.GhostEnabled {
		runtime.GhostHealthy = true
		runtime.GhostLastError = ""
	} else {
		runtime.GhostHealthy = !ghostState.lastFailure.After(ghostState.lastSuccess)
		runtime.GhostLastError = redact.SensitiveText(ghostState.lastError)
	}
	return runtime
}

func (s *Server) recordPredictorSuccess() {
	s.predictorHealthMu.Lock()
	s.predictorHealth.lastSuccess = s.now()
	s.predictorHealth.lastError = ""
	s.predictorHealthMu.Unlock()
}

func (s *Server) recordPredictorFailure(err error) {
	if err == nil {
		return
	}
	s.predictorHealthMu.Lock()
	s.predictorHealth.lastFailure = s.now()
	s.predictorHealth.lastError = redact.Error(err)
	s.predictorHealthMu.Unlock()
}

func (s *Server) predictorHealthSnapshot() predictorHealthState {
	s.predictorHealthMu.RLock()
	defer s.predictorHealthMu.RUnlock()
	return s.predictorHealth
}

func (s *Server) recordGhostSuccess() {
	s.ghostHealthMu.Lock()
	s.ghostHealth.lastSuccess = s.now()
	s.ghostHealth.lastError = ""
	s.ghostHealthMu.Unlock()
}

func (s *Server) recordGhostFailure(err error) {
	if err == nil {
		return
	}
	s.ghostHealthMu.Lock()
	s.ghostHealth.lastFailure = s.now()
	s.ghostHealth.lastError = redact.Error(err)
	s.ghostHealthMu.Unlock()
}

func (s *Server) ghostHealthSnapshot() predictorHealthState {
	s.ghostHealthMu.RLock()
	defer s.ghostHealthMu.RUnlock()
	return s.ghostHealth
}

func boolMessage(ok bool, okMessage string, failMessage string) string {
	if ok {
		return okMessage
	}
	return failMessage
}

func enterpriseStorageMessage(mode string, driver string, durable bool) string {
	if durable {
		if strings.TrimSpace(driver) == "" {
			return "sqlite-durable"
		}
		return strings.TrimSpace(driver) + "-durable"
	}
	if mode == "prod" {
		return "prod-requires-durable-storage"
	}
	if driver == "" {
		return "storage-not-durable"
	}
	return strings.TrimSpace(driver) + "-not-durable"
}
