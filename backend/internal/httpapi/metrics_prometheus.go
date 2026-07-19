package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"kubelens-backend/internal/model"
)

type MetricsController struct {
	metrics         *requestMetrics
	docs            docsRetriever
	runtimeSnapshot func() model.RuntimeStatus
	auditPosture    func() auditPosture
}

func NewMetricsController(
	metrics *requestMetrics,
	docs docsRetriever,
	runtimeSnapshot func() model.RuntimeStatus,
	auditPostureSnapshot func() auditPosture,
) *MetricsController {
	if runtimeSnapshot == nil {
		runtimeSnapshot = func() model.RuntimeStatus { return model.RuntimeStatus{} }
	}
	if auditPostureSnapshot == nil {
		auditPostureSnapshot = func() auditPosture { return auditPosture{Store: "unavailable"} }
	}
	return &MetricsController{
		metrics:         metrics,
		docs:            docs,
		runtimeSnapshot: runtimeSnapshot,
		auditPosture:    auditPostureSnapshot,
	}
}

func (mc *MetricsController) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", mc.handleMetrics)
	r.Get("/prometheus", mc.handlePrometheusMetrics)
	return r
}

func (mc *MetricsController) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	if mc.metrics == nil {
		writeError(w, http.StatusServiceUnavailable, "request metrics are not configured")
		return
	}

	snap := mc.metrics.snapshot()
	snap.RAG = ragMetricsFromRetriever(mc.docs)
	writeJSON(w, http.StatusOK, snap)
}

func (mc *MetricsController) handlePrometheusMetrics(w http.ResponseWriter, _ *http.Request) {
	if mc.metrics == nil {
		writeError(w, http.StatusServiceUnavailable, "request metrics are not configured")
		return
	}

	snap := mc.metrics.snapshot()
	runtime := mc.runtimeSnapshot()
	audit := mc.auditPosture()
	rag := ragMetricsFromRetriever(mc.docs)
	auditDurable := runtime.AuditDurable
	auditSigned := runtime.AuditSigned
	if audit.Store != "unavailable" {
		auditDurable = audit.Durable
		auditSigned = audit.Signed
	}

	var b strings.Builder
	writePrometheusMetric(&b, "kubelens_http_requests_total", float64(snap.TotalRequests), nil)
	writePrometheusMetric(&b, "kubelens_http_errors_total", float64(snap.TotalErrors), nil)
	writePrometheusMetric(&b, "kubelens_http_inflight_requests", float64(snap.InFlight), nil)
	writePrometheusMetric(&b, "kubelens_http_uptime_seconds", float64(snap.UptimeSeconds), nil)
	writePrometheusMetric(&b, "kubelens_http_latency_avg_ms", snap.AvgLatencyMs, nil)
	writePrometheusMetric(&b, "kubelens_http_latency_max_ms", snap.MaxLatencyMs, nil)

	writePrometheusMetric(&b, "kubelens_runtime_auth_enabled", boolToGauge(runtime.AuthEnabled), nil)
	writePrometheusMetric(&b, "kubelens_runtime_write_actions_enabled", boolToGauge(runtime.WriteActionsEnabled), nil)
	writePrometheusMetric(&b, "kubelens_runtime_enterprise_storage", boolToGauge(runtime.EnterpriseStorage), nil)
	writePrometheusMetric(&b, "kubelens_runtime_database_migrations_enabled", boolToGauge(runtime.DatabaseMigrations), nil)
	writePrometheusMetric(&b, "kubelens_runtime_memory_durable", boolToGauge(runtime.MemoryDurable), nil)
	writePrometheusMetric(&b, "kubelens_runtime_audit_durable", boolToGauge(auditDurable), nil)
	writePrometheusMetric(&b, "kubelens_runtime_audit_signed", boolToGauge(auditSigned), nil)
	writePrometheusMetric(&b, "kubelens_runtime_predictor_enabled", boolToGauge(runtime.PredictorEnabled), nil)
	writePrometheusMetric(&b, "kubelens_runtime_predictor_healthy", boolToGauge(runtime.PredictorHealthy), nil)
	writePrometheusMetric(&b, "kubelens_runtime_ghost_enabled", boolToGauge(runtime.GhostEnabled), nil)
	writePrometheusMetric(&b, "kubelens_runtime_ghost_healthy", boolToGauge(runtime.GhostHealthy), nil)
	writePrometheusMetric(&b, "kubelens_runtime_alerts_enabled", boolToGauge(runtime.AlertsEnabled), nil)
	writePrometheusMetric(&b, "kubelens_audit_sink_failures_total", float64(audit.Failures), nil)
	writePrometheusMetric(&b, "kubelens_rag_enabled", boolToGauge(rag.Enabled), nil)
	writePrometheusMetric(&b, "kubelens_rag_queries_total", float64(rag.TotalQueries), nil)
	writePrometheusMetric(&b, "kubelens_rag_empty_results_total", float64(rag.EmptyResults), nil)
	writePrometheusMetric(&b, "kubelens_rag_hit_rate", rag.HitRate, nil)
	writePrometheusMetric(&b, "kubelens_rag_average_results", rag.AverageResults, nil)
	writePrometheusMetric(&b, "kubelens_rag_feedback_signals_total", float64(rag.FeedbackSignals), nil)
	writePrometheusMetric(&b, "kubelens_rag_feedback_positive_total", float64(rag.PositiveFeedback), nil)
	writePrometheusMetric(&b, "kubelens_rag_feedback_negative_total", float64(rag.NegativeFeedback), nil)

	for _, route := range snap.Routes {
		labels := map[string]string{"route": route.Route}
		writePrometheusMetric(&b, "kubelens_http_route_requests_total", float64(route.Requests), labels)
		writePrometheusMetric(&b, "kubelens_http_route_errors_total", float64(route.Errors), labels)
		writePrometheusMetric(&b, "kubelens_http_route_status_2xx_total", float64(route.Status2xx), labels)
		writePrometheusMetric(&b, "kubelens_http_route_status_3xx_total", float64(route.Status3xx), labels)
		writePrometheusMetric(&b, "kubelens_http_route_status_4xx_total", float64(route.Status4xx), labels)
		writePrometheusMetric(&b, "kubelens_http_route_status_5xx_total", float64(route.Status5xx), labels)
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(b.String()))
}

func writePrometheusMetric(b *strings.Builder, name string, value float64, labels map[string]string) {
	if len(labels) == 0 {
		_, _ = fmt.Fprintf(b, "%s %v\n", name, value)
		return
	}

	first := true
	b.WriteString(name)
	b.WriteString("{")
	for key, rawValue := range labels {
		if !first {
			b.WriteString(",")
		}
		first = false
		b.WriteString(key)
		b.WriteString("=\"")
		b.WriteString(escapePrometheusLabel(rawValue))
		b.WriteString("\"")
	}
	b.WriteString("} ")
	_, _ = fmt.Fprintf(b, "%v\n", value)
}

func escapePrometheusLabel(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", " ")
	return replacer.Replace(value)
}

func boolToGauge(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
