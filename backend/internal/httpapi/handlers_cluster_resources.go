package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"kubelens-backend/internal/auth"
	"kubelens-backend/internal/model"
)

type ResourceController struct {
	cluster                    ClusterReader
	audit                      *auditLog
	now                        func() time.Time
	decodeJSONBody             func(*http.Request, any) error
	evaluateManifestRisk       func(string, []model.PodSummary, []model.NodeSummary) model.RiskReport
	invalidatePredictionsCache func()
}

func NewResourceController(
	cluster ClusterReader,
	audit *auditLog,
	now func() time.Time,
	decode func(*http.Request, any) error,
	evaluateManifestRisk func(string, []model.PodSummary, []model.NodeSummary) model.RiskReport,
	invalidatePredictionsCache func(),
) *ResourceController {
	if now == nil {
		now = time.Now
	}
	if decode == nil {
		decode = decodeJSONBody
	}
	if evaluateManifestRisk == nil {
		evaluateManifestRisk = func(string, []model.PodSummary, []model.NodeSummary) model.RiskReport {
			return model.RiskReport{}
		}
	}
	if invalidatePredictionsCache == nil {
		invalidatePredictionsCache = func() {}
	}
	return &ResourceController{
		cluster:                    cluster,
		audit:                      audit,
		now:                        now,
		decodeJSONBody:             decode,
		evaluateManifestRisk:       evaluateManifestRisk,
		invalidatePredictionsCache: invalidatePredictionsCache,
	}
}

func (rc *ResourceController) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{kind}", rc.handleResources)
	r.Get("/{kind}/{namespace}/{name}/yaml", rc.handleGetResourceYAML)
	r.Put("/{kind}/{namespace}/{name}/yaml", rc.handleApplyResourceYAML)
	r.Post("/{kind}/{namespace}/{name}/scale", rc.handleScaleResource)
	r.Post("/{kind}/{namespace}/{name}/restart", rc.handleRestartResource)
	r.Post("/{kind}/{namespace}/{name}/rollback", rc.handleRollbackResource)
	return r
}

func (rc *ResourceController) handleResources(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(chi.URLParam(r, "kind"))
	if kind == "" {
		writeError(w, http.StatusBadRequest, "resource kind is required")
		return
	}

	items, err := rc.cluster.ListResources(r.Context(), kind)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.ResourceList{
		Kind:  kind,
		Items: items,
	})
}

func (rc *ResourceController) handleGetResourceYAML(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(chi.URLParam(r, "kind"))
	namespace := strings.TrimSpace(chi.URLParam(r, "namespace"))
	name := strings.TrimSpace(chi.URLParam(r, "name"))

	yamlText, err := rc.cluster.GetResourceYAML(r.Context(), kind, namespace, name)
	if err != nil {
		handleActionError(w, err, "Resource not found")
		return
	}

	writeJSON(w, http.StatusOK, model.ResourceManifest{YAML: yamlText})
}

func (rc *ResourceController) handleApplyResourceYAML(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(chi.URLParam(r, "kind"))
	namespace := strings.TrimSpace(chi.URLParam(r, "namespace"))
	name := strings.TrimSpace(chi.URLParam(r, "name"))

	var req model.ResourceManifest
	if err := rc.decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	pods, nodes := rc.cluster.Snapshot(r.Context())
	risk := rc.evaluateManifestRisk(req.YAML, pods, nodes)
	force := queryBool(r, "force")
	if risk.Score >= 50 && !force {
		writeJSON(w, http.StatusAccepted, model.ResourceApplyRiskResponse{
			Message:       "Risk guard blocked apply. Review the report and retry with force=true if override is justified.",
			RequiresForce: true,
			Report:        risk,
		})
		return
	}
	if risk.Score >= 50 && force && rc.audit != nil {
		entry := model.AuditEntry{
			Timestamp: rc.now().UTC().Format(time.RFC3339),
			RequestID: middleware.GetReqID(r.Context()),
			Method:    r.Method,
			Path:      sanitizeAuditPath(r.URL.Path),
			Action:    fmt.Sprintf("resource.apply.force_override riskScore=%d", risk.Score),
			Status:    http.StatusOK,
			ClientIP:  sanitizeClientIP(r.RemoteAddr),
			Success:   true,
		}
		if principal, ok := auth.PrincipalFromContext(r.Context()); ok {
			entry.User = principal.User
			entry.Role = auth.RoleLabel(principal.Role)
		}
		rc.audit.append(entry)
	}

	result, err := rc.cluster.ApplyResourceYAML(r.Context(), kind, namespace, name, req.YAML)
	if err != nil {
		handleActionError(w, err, "Resource not found")
		return
	}

	rc.invalidatePredictionsCache()
	writeJSON(w, http.StatusOK, result)
}

func (rc *ResourceController) handleScaleResource(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(chi.URLParam(r, "kind"))
	namespace := strings.TrimSpace(chi.URLParam(r, "namespace"))
	name := strings.TrimSpace(chi.URLParam(r, "name"))

	var req model.ScaleRequest
	if err := rc.decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := rc.cluster.ScaleResource(r.Context(), kind, namespace, name, req.Replicas)
	if err != nil {
		handleActionError(w, err, "Resource not found")
		return
	}

	rc.invalidatePredictionsCache()
	writeJSON(w, http.StatusOK, result)
}

func (rc *ResourceController) handleRestartResource(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(chi.URLParam(r, "kind"))
	namespace := strings.TrimSpace(chi.URLParam(r, "namespace"))
	name := strings.TrimSpace(chi.URLParam(r, "name"))

	result, err := rc.cluster.RestartResource(r.Context(), kind, namespace, name)
	if err != nil {
		handleActionError(w, err, "Resource not found")
		return
	}
	rc.invalidatePredictionsCache()
	writeJSON(w, http.StatusOK, result)
}

func (rc *ResourceController) handleRollbackResource(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(chi.URLParam(r, "kind"))
	namespace := strings.TrimSpace(chi.URLParam(r, "namespace"))
	name := strings.TrimSpace(chi.URLParam(r, "name"))

	result, err := rc.cluster.RollbackResource(r.Context(), kind, namespace, name)
	if err != nil {
		handleActionError(w, err, "Resource not found")
		return
	}
	rc.invalidatePredictionsCache()
	writeJSON(w, http.StatusOK, result)
}
