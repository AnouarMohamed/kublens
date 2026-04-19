package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"kubelens-backend/internal/incident"
	"kubelens-backend/internal/model"
	"kubelens-backend/internal/postmortem"
)

const chatOpsTimeout = 10 * time.Second

func (s *Server) handleCreateIncident(w http.ResponseWriter, r *http.Request) {
	if s.incidents == nil {
		writeError(w, http.StatusServiceUnavailable, "incident store is not configured")
		return
	}

	pods, nodes := s.cluster.Snapshot(r.Context())
	events := s.cluster.ListClusterEvents(r.Context())
	diag := s.mapDiagnosticsReport(s.runDiagnostics(r.Context()))
	predictions := s.collectPredictionsForIncident(r.Context(), pods, nodes, events)
	newIncident := incident.BuildIncident(r.Context(), diag, events, pods, predictions, s.now)
	created, err := createIncidentWithContext(r.Context(), s.incidents, newIncident)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist incident")
		return
	}

	s.notifyChatOps(func(ctx context.Context) {
		if s.chatops != nil {
			s.chatops.NotifyIncident(ctx, created)
		}
	})

	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleListIncidents(w http.ResponseWriter, r *http.Request) {
	if s.incidents == nil {
		writeJSON(w, http.StatusOK, []model.Incident{})
		return
	}
	items, err := listIncidentsWithContext(r.Context(), s.incidents)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list incidents")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleGetIncident(w http.ResponseWriter, r *http.Request) {
	if s.incidents == nil {
		writeError(w, http.StatusServiceUnavailable, "incident store is not configured")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	value, ok, err := getIncidentWithContext(r.Context(), s.incidents, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load incident")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "incident not found")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) handlePatchIncidentStep(w http.ResponseWriter, r *http.Request) {
	if s.incidents == nil {
		writeError(w, http.StatusServiceUnavailable, "incident store is not configured")
		return
	}

	incidentID := strings.TrimSpace(chi.URLParam(r, "id"))
	stepID := strings.TrimSpace(chi.URLParam(r, "step"))
	if incidentID == "" || stepID == "" {
		writeError(w, http.StatusBadRequest, "incident id and step id are required")
		return
	}

	var req model.IncidentStepStatusPatch
	if err := s.decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := patchIncidentStepWithContext(r.Context(), s.incidents, incidentID, stepID, req.Status)
	if err != nil {
		switch {
		case errors.Is(err, incident.ErrIncidentNotFound):
			writeError(w, http.StatusNotFound, "incident not found")
		case errors.Is(err, incident.ErrStepNotFound):
			writeError(w, http.StatusNotFound, "runbook step not found")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleResolveIncident(w http.ResponseWriter, r *http.Request) {
	if s.incidents == nil {
		writeError(w, http.StatusServiceUnavailable, "incident store is not configured")
		return
	}

	incidentID := strings.TrimSpace(chi.URLParam(r, "id"))
	if incidentID == "" {
		writeError(w, http.StatusBadRequest, "incident id is required")
		return
	}

	resolved, err := resolveIncidentWithContext(r.Context(), s.incidents, incidentID)
	if err != nil {
		if errors.Is(err, incident.ErrIncidentNotFound) {
			writeError(w, http.StatusNotFound, "incident not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.notifyChatOps(func(ctx context.Context) {
		if s.chatops != nil {
			statusUpdate := resolved
			statusUpdate.Severity = "resolved"
			s.chatops.NotifyIncident(ctx, statusUpdate)
		}
	})

	writeJSON(w, http.StatusOK, resolved)
}

func (s *Server) handleGeneratePostmortem(w http.ResponseWriter, r *http.Request) {
	if s.incidents == nil || s.postmortems == nil {
		writeError(w, http.StatusServiceUnavailable, "postmortem generation is not configured")
		return
	}

	incidentID := strings.TrimSpace(chi.URLParam(r, "id"))
	if incidentID == "" {
		writeError(w, http.StatusBadRequest, "incident id is required")
		return
	}

	value, ok, err := getIncidentWithContext(r.Context(), s.incidents, incidentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load incident")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "incident not found")
		return
	}
	if value.Status != model.IncidentStatusResolved {
		writeError(w, http.StatusBadRequest, "incident must be resolved before generating postmortem")
		return
	}

	if existing, exists, err := getPostmortemByIncidentWithContext(r.Context(), s.postmortems, incidentID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load postmortem")
		return
	} else if exists {
		writeError(w, http.StatusConflict, "postmortem already exists for incident: "+existing.ID)
		return
	}

	generated := postmortem.Generate(r.Context(), value, s.ai, s.now)
	created, err := createPostmortemWithContext(r.Context(), s.postmortems, generated)
	if err != nil {
		if errors.Is(err, postmortem.ErrPostmortemExists) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.notifyChatOps(func(ctx context.Context) {
		if s.chatops != nil {
			s.chatops.NotifyPostmortem(ctx, created)
		}
	})

	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleListPostmortems(w http.ResponseWriter, r *http.Request) {
	if s.postmortems == nil {
		writeJSON(w, http.StatusOK, []model.Postmortem{})
		return
	}
	items, err := listPostmortemsWithContext(r.Context(), s.postmortems)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list postmortems")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleGetPostmortem(w http.ResponseWriter, r *http.Request) {
	if s.postmortems == nil {
		writeError(w, http.StatusServiceUnavailable, "postmortem store is not configured")
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	value, ok, err := getPostmortemWithContext(r.Context(), s.postmortems, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load postmortem")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "postmortem not found")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) collectPredictionsForIncident(
	ctx context.Context,
	pods []model.PodSummary,
	nodes []model.NodeSummary,
	events []model.K8sEvent,
) model.PredictionsResult {
	request := predictorRequest{
		Pods:      pods,
		Nodes:     nodes,
		Events:    events,
		Timestamp: s.now().UTC().Format(time.RFC3339),
	}
	if s.predictor != nil {
		predictions, err := s.predictor.Predict(ctx, request)
		if err == nil {
			s.recordPredictorSuccess()
			return predictions
		}
		s.recordPredictorFailure(err)
		s.logger.Warn("incident prediction fallback", "error", err.Error())
	}
	return buildLocalPredictions(pods, nodes, events, s.now())
}

func (s *Server) notifyChatOps(run func(context.Context)) {
	if run == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), chatOpsTimeout)
		defer cancel()
		run(ctx)
	}()
}
