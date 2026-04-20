package httpapi

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"kubelens-backend/internal/model"
)

const (
	sloLatencyThresholdMs = 750.0
)

func (s *Server) handleSLOOverview(w http.ResponseWriter, r *http.Request) {
	snapshot := s.metrics.snapshot()
	stats := s.currentClusterStats(r.Context())

	incidents := []model.Incident{}
	if s.incidents != nil {
		items, err := listIncidentsWithContext(r.Context(), s.incidents)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load incidents for slo overview")
			return
		}
		incidents = items
	}

	overview := buildSLOOverview(snapshot, stats, incidents, s.now())
	writeJSON(w, http.StatusOK, overview)
}

func buildSLOOverview(
	snapshot metricsSnapshot,
	stats model.ClusterStats,
	incidents []model.Incident,
	now time.Time,
) model.SLOOverview {
	openIncidents := 0
	criticalOpenIncidents := 0
	acknowledgedOpenIncidents := 0
	for _, item := range incidents {
		if item.Status != model.IncidentStatusOpen {
			continue
		}
		openIncidents++
		if strings.EqualFold(strings.TrimSpace(item.Severity), string(model.SeverityCritical)) {
			criticalOpenIncidents++
		}
		if incidentAcknowledged(item) {
			acknowledgedOpenIncidents++
		}
	}

	availabilityCurrent := 100.0
	if snapshot.TotalRequests > 0 {
		availabilityCurrent = 100 - (float64(snapshot.TotalErrors)/float64(snapshot.TotalRequests))*100
	}
	availabilitySignals := []model.SLOSignal{
		{Label: "Requests", Value: fmt.Sprintf("%d", snapshot.TotalRequests), Tone: "neutral"},
		{Label: "5xx errors", Value: fmt.Sprintf("%d", snapshot.TotalErrors), Tone: toneForCount(snapshot.TotalErrors)},
		{Label: "Avg latency", Value: fmt.Sprintf("%.1fms", snapshot.AvgLatencyMs), Tone: toneForLatency(snapshot.AvgLatencyMs)},
	}

	fastRequestShare, slowRouteCount := computeFastRouteShare(snapshot.Routes, sloLatencyThresholdMs)
	latencySignals := []model.SLOSignal{
		{Label: "Fast request share", Value: fmt.Sprintf("%.1f%%", fastRequestShare), Tone: toneForPercent(fastRequestShare, 95)},
		{Label: "Slow routes", Value: fmt.Sprintf("%d", slowRouteCount), Tone: toneForCount(uint64(slowRouteCount))},
		{Label: "Max latency", Value: fmt.Sprintf("%.1fms", snapshot.MaxLatencyMs), Tone: toneForLatency(snapshot.MaxLatencyMs)},
	}

	totalUnits := stats.Pods.Total + stats.Nodes.Total
	readyUnits := stats.Pods.Running + stats.Nodes.Ready
	readinessCurrent := 100.0
	if totalUnits > 0 {
		readinessCurrent = (float64(readyUnits) / float64(totalUnits)) * 100
	}
	readinessSignals := []model.SLOSignal{
		{
			Label: "Ready nodes",
			Value: fmt.Sprintf("%d/%d", stats.Nodes.Ready, stats.Nodes.Total),
			Tone:  toneForRatio(stats.Nodes.Ready, stats.Nodes.Total),
		},
		{
			Label: "Running pods",
			Value: fmt.Sprintf("%d/%d", stats.Pods.Running, stats.Pods.Total),
			Tone:  toneForRatio(stats.Pods.Running, stats.Pods.Total),
		},
		{
			Label: "Pending/failed pods",
			Value: fmt.Sprintf("%d", stats.Pods.Pending+stats.Pods.Failed),
			Tone:  toneForCount(uint64(stats.Pods.Pending + stats.Pods.Failed)),
		},
	}

	incidentResponseCurrent := 100.0
	if openIncidents > 0 {
		incidentResponseCurrent = (float64(acknowledgedOpenIncidents) / float64(openIncidents)) * 100
	}
	incidentSignals := []model.SLOSignal{
		{Label: "Open incidents", Value: fmt.Sprintf("%d", openIncidents), Tone: toneForCount(uint64(openIncidents))},
		{Label: "Critical open", Value: fmt.Sprintf("%d", criticalOpenIncidents), Tone: toneForCount(uint64(criticalOpenIncidents))},
		{
			Label: "Acknowledged",
			Value: fmt.Sprintf("%d/%d", acknowledgedOpenIncidents, openIncidents),
			Tone:  toneForRatio(acknowledgedOpenIncidents, openIncidents),
		},
	}

	objectives := []model.SLOObjective{
		buildSLOObjective(
			"api-availability",
			"API Availability",
			"Service health",
			"Protect the request error budget from sustained 5xx failures.",
			"runtime window",
			99.9,
			availabilityCurrent,
			availabilitySignals,
		),
		buildSLOObjective(
			"latency-budget",
			"Latency Budget",
			"User experience",
			fmt.Sprintf("Keep request-weighted route latency within %.0fms for the active traffic mix.", sloLatencyThresholdMs),
			"runtime window",
			95.0,
			fastRequestShare,
			latencySignals,
		),
		buildSLOObjective(
			"cluster-readiness",
			"Cluster Readiness",
			"Platform stability",
			"Keep runnable workload and node readiness above the rollout safety threshold.",
			"live snapshot",
			99.0,
			readinessCurrent,
			readinessSignals,
		),
		buildSLOObjective(
			"incident-response",
			"Incident Response",
			"Operational execution",
			"Keep open incidents actively acknowledged so the runbook queue does not stall.",
			"live workflow",
			95.0,
			incidentResponseCurrent,
			incidentSignals,
		),
	}

	healthy := 0
	atRisk := 0
	breached := 0
	alerts := make([]string, 0, 4)
	for _, objective := range objectives {
		switch objective.Status {
		case model.SLOStatusHealthy:
			healthy++
		case model.SLOStatusAtRisk:
			atRisk++
		case model.SLOStatusBreached:
			breached++
			alerts = append(alerts, fmt.Sprintf("%s is breaching its current error budget.", objective.Name))
		}
	}
	if criticalOpenIncidents > 0 {
		alerts = append(alerts, fmt.Sprintf("%d critical incidents remain open in the active control window.", criticalOpenIncidents))
	}
	if len(alerts) == 0 {
		alerts = append(alerts, "All tracked objectives are operating within their current guardrails.")
	}

	sort.SliceStable(objectives, func(i, j int) bool {
		return sloSeverityRank(objectives[i].Status) > sloSeverityRank(objectives[j].Status)
	})

	return model.SLOOverview{
		GeneratedAt:        now.UTC().Format(time.RFC3339),
		Summary:            fmt.Sprintf("%d healthy, %d at risk, %d breached objectives.", healthy, atRisk, breached),
		HealthyObjectives:  healthy,
		AtRiskObjectives:   atRisk,
		BreachedObjectives: breached,
		Alerts:             alerts,
		Objectives:         objectives,
	}
}

func buildSLOObjective(
	id string,
	name string,
	category string,
	summary string,
	window string,
	targetPercent float64,
	currentPercent float64,
	signals []model.SLOSignal,
) model.SLOObjective {
	currentPercent = clampPercent(currentPercent)
	targetPercent = clampPercent(targetPercent)

	allowedBadPercent := math.Max(100-targetPercent, 0.1)
	currentBadPercent := math.Max(0, 100-currentPercent)
	errorBudgetUsedPercent := clampPercent((currentBadPercent / allowedBadPercent) * 100)
	budgetRemainingPercent := clampPercent(100 - errorBudgetUsedPercent)
	burnRate := currentBadPercent / allowedBadPercent

	status := model.SLOStatusHealthy
	switch {
	case currentPercent < targetPercent && burnRate >= 1:
		status = model.SLOStatusBreached
	case burnRate >= 0.5 || currentPercent < targetPercent:
		status = model.SLOStatusAtRisk
	}

	return model.SLOObjective{
		ID:                     id,
		Name:                   name,
		Category:               category,
		Summary:                summary,
		Window:                 window,
		Status:                 status,
		TargetValue:            fmt.Sprintf("%.1f%%", targetPercent),
		CurrentValue:           fmt.Sprintf("%.1f%%", currentPercent),
		TargetPercent:          roundToSingleDecimal(targetPercent),
		CurrentPercent:         roundToSingleDecimal(currentPercent),
		ErrorBudgetUsedPercent: roundToSingleDecimal(errorBudgetUsedPercent),
		BudgetRemainingPercent: roundToSingleDecimal(budgetRemainingPercent),
		BurnRate:               roundToSingleDecimal(burnRate),
		Signals:                append([]model.SLOSignal(nil), signals...),
	}
}

func computeFastRouteShare(routes []routeMetricsSummary, thresholdMs float64) (float64, int) {
	var totalRequests uint64
	var fastRequests uint64
	slowRoutes := 0

	for _, route := range routes {
		totalRequests += route.Requests
		if route.AvgLatencyMs <= thresholdMs {
			fastRequests += route.Requests
		} else if route.Requests > 0 {
			slowRoutes++
		}
	}

	if totalRequests == 0 {
		return 100, 0
	}
	return (float64(fastRequests) / float64(totalRequests)) * 100, slowRoutes
}

func incidentAcknowledged(item model.Incident) bool {
	for _, entry := range item.Timeline {
		if entry.Kind == model.TimelineEntryKindAction {
			return true
		}
	}
	for _, step := range item.Runbook {
		if step.Status != model.RunbookStepStatusPending {
			return true
		}
	}
	return false
}

func clampPercent(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 100:
		return 100
	default:
		return value
	}
}

func roundToSingleDecimal(value float64) float64 {
	return math.Round(value*10) / 10
}

func toneForCount(value uint64) string {
	switch {
	case value == 0:
		return "healthy"
	case value == 1:
		return "warning"
	default:
		return "critical"
	}
}

func toneForLatency(latencyMs float64) string {
	switch {
	case latencyMs <= 250:
		return "healthy"
	case latencyMs <= sloLatencyThresholdMs:
		return "warning"
	default:
		return "critical"
	}
}

func toneForPercent(current float64, target float64) string {
	switch {
	case current >= target:
		return "healthy"
	case current >= target-5:
		return "warning"
	default:
		return "critical"
	}
}

func toneForRatio(good int, total int) string {
	if total <= 0 {
		return "healthy"
	}
	return toneForPercent((float64(good)/float64(total))*100, 95)
}

func sloSeverityRank(status model.SLOStatus) int {
	switch status {
	case model.SLOStatusBreached:
		return 3
	case model.SLOStatusAtRisk:
		return 2
	default:
		return 1
	}
}
