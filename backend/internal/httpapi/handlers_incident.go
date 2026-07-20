package httpapi

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"kubelens-backend/internal/incident"
	"kubelens-backend/internal/model"
	"kubelens-backend/internal/postmortem"
	"kubelens-backend/internal/redact"
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

func (s *Server) handleGetIncidentReplay(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(w, http.StatusOK, incident.BuildReplay(value, s.now))
}

func (s *Server) handleGetIncidentEvidence(w http.ResponseWriter, r *http.Request) {
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

	var postmortemItem *model.Postmortem
	if s.postmortems != nil {
		if item, found, err := getPostmortemByIncidentWithContext(r.Context(), s.postmortems, value.ID); err == nil && found {
			postmortemItem = &item
		}
	}

	bundle := incident.BuildEvidenceBundle(
		value,
		s.relatedAuditEntriesForIncident(value),
		s.relatedRemediationsForIncident(r.Context(), value),
		postmortemItem,
		s.now,
	)
	writeJSON(w, http.StatusOK, bundle)
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
		s.logger.Warn("incident prediction fallback", "error", redact.Error(err))
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

func (s *Server) relatedAuditEntriesForIncident(value model.Incident) []model.AuditEntry {
	if s.audit == nil {
		return nil
	}

	startedAt := parseIncidentTimelineTime(value.OpenedAt, s.now())
	endedAt := startedAt
	if strings.TrimSpace(value.ResolvedAt) != "" {
		endedAt = parseIncidentTimelineTime(value.ResolvedAt, s.now())
	} else {
		endedAt = s.now().UTC()
	}

	tokens := incidentResourceTokens(value)
	windowStart := startedAt.Add(-10 * time.Minute)
	windowEnd := endedAt.Add(10 * time.Minute)

	items := s.audit.list(maxAuditLimit)
	matched := make([]model.AuditEntry, 0, 16)
	fallbackWindow := make([]model.AuditEntry, 0, 16)
	for _, entry := range items {
		at := parseIncidentTimelineTime(entry.Timestamp, endedAt)
		if at.Before(windowStart) || at.After(windowEnd) {
			continue
		}

		fallbackWindow = append(fallbackWindow, entry)
		haystack := strings.ToLower(strings.TrimSpace(entry.Path + " " + entry.Action + " " + entry.Route))
		if strings.HasPrefix(strings.TrimSpace(entry.Path), "/api/incidents") || strings.HasPrefix(strings.TrimSpace(entry.Path), "/api/remediation") {
			matched = append(matched, entry)
			continue
		}
		if containsIncidentToken(haystack, tokens) {
			matched = append(matched, entry)
		}
	}

	if len(matched) == 0 {
		matched = fallbackWindow
	}

	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].Timestamp < matched[j].Timestamp
	})
	if len(matched) > 20 {
		matched = matched[len(matched)-20:]
	}
	out := make([]model.AuditEntry, len(matched))
	copy(out, matched)
	return out
}

func (s *Server) relatedRemediationsForIncident(ctx context.Context, value model.Incident) []model.RemediationProposal {
	if s.remediations == nil {
		return nil
	}

	items, err := listRemediationsWithContext(ctx, s.remediations)
	if err != nil {
		return nil
	}

	idSet := make(map[string]struct{}, len(value.AssociatedRemediationIDs))
	for _, id := range value.AssociatedRemediationIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		idSet[trimmed] = struct{}{}
	}
	resourceSet := make(map[string]struct{}, len(value.AffectedResources))
	for _, resource := range value.AffectedResources {
		resourceSet[strings.ToLower(strings.TrimSpace(resource))] = struct{}{}
	}

	out := make([]model.RemediationProposal, 0, len(items))
	for _, item := range items {
		if _, ok := idSet[strings.TrimSpace(item.ID)]; ok {
			out = append(out, item)
			continue
		}
		resourceKey := strings.ToLower(strings.TrimSpace(item.Resource))
		if ns := strings.TrimSpace(item.Namespace); ns != "" {
			resourceKey = strings.ToLower(ns + "/" + strings.TrimSpace(item.Resource))
		}
		if _, ok := resourceSet[resourceKey]; ok {
			out = append(out, item)
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt < out[j].CreatedAt
	})
	return out
}

func incidentResourceTokens(value model.Incident) []string {
	seen := map[string]struct{}{}
	tokens := make([]string, 0, len(value.AffectedResources)*2)
	add := func(raw string) {
		trimmed := strings.ToLower(strings.TrimSpace(raw))
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		tokens = append(tokens, trimmed)
	}

	add(value.ID)
	for _, resource := range value.AffectedResources {
		add(resource)
		parts := strings.Split(strings.TrimSpace(resource), "/")
		if len(parts) == 2 {
			add(parts[0])
			add(parts[1])
		}
	}
	return tokens
}

func containsIncidentToken(haystack string, tokens []string) bool {
	for _, token := range tokens {
		if token != "" && strings.Contains(haystack, token) {
			return true
		}
	}
	return false
}

func parseIncidentTimelineTime(raw string, fallback time.Time) time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return fallback.UTC()
	}
	return parsed.UTC()
}
