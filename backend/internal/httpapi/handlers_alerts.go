package httpapi

import (
	"net/http"
	"strings"

	"kubelens-backend/internal/auth"
	"kubelens-backend/internal/model"
)

func (s *Server) handleAlertDispatch(w http.ResponseWriter, r *http.Request) {
	if s.alerts == nil || !s.alerts.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "alert integrations are not configured")
		return
	}

	var req model.AlertDispatchRequest
	if err := s.decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Message = strings.TrimSpace(req.Message)
	if req.Title == "" || req.Message == "" {
		writeError(w, http.StatusBadRequest, "title and message are required")
		return
	}

	result := s.alerts.Dispatch(r.Context(), req)
	status := http.StatusOK
	if !result.Success {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, result)
}

func (s *Server) handleAlertTest(w http.ResponseWriter, r *http.Request) {
	if s.alerts == nil || !s.alerts.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "alert integrations are not configured")
		return
	}

	req := model.AlertDispatchRequest{
		Title:    "KubeLens test alert",
		Message:  "This is a test alert from KubeLens diagnostics.",
		Severity: "warning",
		Source:   "kubelens",
		Tags:     []string{"test", "diagnostics"},
	}

	result := s.alerts.Dispatch(r.Context(), req)
	status := http.StatusOK
	if !result.Success {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, result)
}

func (s *Server) handleListAlertLifecycle(w http.ResponseWriter, r *http.Request) {
	if s.alertLifecycle == nil {
		writeJSON(w, http.StatusOK, []model.NodeAlertLifecycle{})
		return
	}
	writeJSON(w, http.StatusOK, s.alertLifecycle.List(r.Context()))
}

func (s *Server) handleUpsertAlertLifecycle(w http.ResponseWriter, r *http.Request) {
	if s.alertLifecycle == nil {
		writeError(w, http.StatusServiceUnavailable, "alert lifecycle store is unavailable")
		return
	}

	var req model.NodeAlertLifecycleUpdateRequest
	if err := s.decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	actor := "unknown"
	if principal, ok := auth.PrincipalFromContext(r.Context()); ok {
		actor = strings.TrimSpace(principal.User)
	}

	item, err := s.alertLifecycle.Upsert(r.Context(), req, actor)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid alert lifecycle request")
		return
	}

	writeJSON(w, http.StatusOK, item)
}
