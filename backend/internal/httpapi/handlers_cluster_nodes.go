package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"kubelens-backend/internal/apperrors"
	"kubelens-backend/internal/auth"
	"kubelens-backend/internal/model"
)

type nodeMaintenanceReader interface {
	DrainNodePreview(ctx context.Context, name string) (model.NodeDrainPreview, error)
}

type nodeMaintenanceWriter interface {
	UncordonNode(ctx context.Context, name string) (model.ActionResult, error)
	DrainNode(ctx context.Context, name string, force bool) (model.ActionResult, error)
}

type nodeScopeReader interface {
	NodePods(ctx context.Context, name string) ([]model.PodSummary, error)
	NodeEvents(ctx context.Context, name string) ([]model.K8sEvent, error)
}

type NodeController struct {
	cluster                    ClusterReader
	audit                      *auditLog
	now                        func() time.Time
	decodeJSONBody             func(*http.Request, any) error
	invalidatePredictionsCache func()
}

func NewNodeController(
	cluster ClusterReader,
	audit *auditLog,
	now func() time.Time,
	decode func(*http.Request, any) error,
	invalidatePredictionsCache func(),
) *NodeController {
	if now == nil {
		now = time.Now
	}
	if decode == nil {
		decode = decodeJSONBody
	}
	if invalidatePredictionsCache == nil {
		invalidatePredictionsCache = func() {}
	}
	return &NodeController{
		cluster:                    cluster,
		audit:                      audit,
		now:                        now,
		decodeJSONBody:             decode,
		invalidatePredictionsCache: invalidatePredictionsCache,
	}
}

func (nc *NodeController) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", nc.handleNodes)
	r.Post("/{name}/cordon", nc.handleCordonNode)
	r.Post("/{name}/uncordon", nc.handleUncordonNode)
	r.Get("/{name}/drain/preview", nc.handleNodeDrainPreview)
	r.Post("/{name}/drain", nc.handleDrainNode)
	r.Get("/{name}/pods", nc.handleNodePods)
	r.Get("/{name}/events", nc.handleNodeEvents)
	r.Get("/{name}", nc.handleNodeDetail)
	return r
}

func (nc *NodeController) handleNodes(w http.ResponseWriter, r *http.Request) {
	_, nodes := nc.cluster.Snapshot(r.Context())
	writeJSON(w, http.StatusOK, nodes)
}

func (nc *NodeController) handleNodeDetail(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	node, err := nc.cluster.NodeDetail(r.Context(), name)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Node not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to fetch node details")
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (nc *NodeController) handleNodePods(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "node name is required")
		return
	}

	if provider, ok := nc.cluster.(nodeScopeReader); ok {
		pods, err := provider.NodePods(r.Context(), name)
		if err != nil {
			if errors.Is(err, apperrors.ErrNotFound) {
				writeError(w, http.StatusNotFound, "Node not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "Failed to fetch node pods")
			return
		}
		writeJSON(w, http.StatusOK, pods)
		return
	}

	pods, _ := nc.cluster.Snapshot(r.Context())
	out := make([]model.PodSummary, 0, len(pods))
	for _, pod := range pods {
		if pod.NodeName == name {
			out = append(out, pod)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (nc *NodeController) handleNodeEvents(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "node name is required")
		return
	}

	if provider, ok := nc.cluster.(nodeScopeReader); ok {
		events, err := provider.NodeEvents(r.Context(), name)
		if err != nil {
			if errors.Is(err, apperrors.ErrNotFound) {
				writeError(w, http.StatusNotFound, "Node not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "Failed to fetch node events")
			return
		}
		writeJSON(w, http.StatusOK, events)
		return
	}

	events := nc.cluster.ListClusterEvents(r.Context())
	out := make([]model.K8sEvent, 0, len(events))
	for _, event := range events {
		if !strings.EqualFold(event.ResourceKind, "Node") {
			continue
		}
		if event.Resource != name {
			continue
		}
		out = append(out, event)
	}
	writeJSON(w, http.StatusOK, out)
}

func (nc *NodeController) handleCordonNode(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	result, err := nc.cluster.CordonNode(r.Context(), name)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Node not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	nc.invalidatePredictionsCache()
	writeJSON(w, http.StatusOK, result)
}

func (nc *NodeController) handleUncordonNode(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	provider, ok := nc.cluster.(nodeMaintenanceWriter)
	if !ok {
		writeError(w, http.StatusNotImplemented, "uncordon is not supported by the active cluster provider")
		return
	}

	result, err := provider.UncordonNode(r.Context(), name)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Node not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	nc.invalidatePredictionsCache()
	writeJSON(w, http.StatusOK, result)
}

func (nc *NodeController) handleNodeDrainPreview(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	provider, ok := nc.cluster.(nodeMaintenanceReader)
	if !ok {
		writeError(w, http.StatusNotImplemented, "node drain preview is not supported by the active cluster provider")
		return
	}

	preview, err := provider.DrainNodePreview(r.Context(), name)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Node not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (nc *NodeController) handleDrainNode(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var req model.NodeDrainRequest
	if err := nc.decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	force := req.Force || queryBool(r, "force")
	forceReason := strings.TrimSpace(req.Reason)
	if force {
		if len(forceReason) > 240 {
			forceReason = forceReason[:240]
		}
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok || principal.Role < auth.RoleAdmin {
			writeError(w, http.StatusForbidden, "force drain requires admin role")
			return
		}
		if forceReason == "" {
			writeError(w, http.StatusBadRequest, "force drain reason is required")
			return
		}
	}

	provider, ok := nc.cluster.(nodeMaintenanceWriter)
	if !ok {
		writeError(w, http.StatusNotImplemented, "node drain is not supported by the active cluster provider")
		return
	}

	result, err := provider.DrainNode(r.Context(), name, force)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Node not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if force && nc.audit != nil {
		entry := model.AuditEntry{
			Timestamp: nc.now().UTC().Format(time.RFC3339),
			RequestID: middleware.GetReqID(r.Context()),
			Method:    r.Method,
			Path:      sanitizeAuditPath(r.URL.Path),
			Action:    fmt.Sprintf("node.drain.force_override reason=%q", forceReason),
			Status:    http.StatusOK,
			ClientIP:  sanitizeClientIP(r.RemoteAddr),
			Success:   true,
		}
		if principal, ok := auth.PrincipalFromContext(r.Context()); ok {
			entry.User = principal.User
			entry.Role = auth.RoleLabel(principal.Role)
		}
		nc.audit.append(entry)
	}
	nc.invalidatePredictionsCache()
	writeJSON(w, http.StatusOK, result)
}
