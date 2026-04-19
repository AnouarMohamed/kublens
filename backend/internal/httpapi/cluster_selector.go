package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"

	"kubelens-backend/internal/model"
	"kubelens-backend/internal/state"
)

const clusterCookieName = "kubelens_cluster"

type ClusterContextsConfig struct {
	Default string
	Readers map[string]ClusterReader
}

type clusterContextKey struct{}

type clusterSelector interface {
	ClusterReader
	DefaultName() string
	Names() []string
	WithClusterName(ctx context.Context, name string) context.Context
	ClusterName(ctx context.Context) string
	ClusterInfo(name string) (model.ClusterContext, bool)
}

type routedCluster struct {
	defaultName string
	readers     map[string]ClusterReader
}

func WithClusterContexts(config ClusterContextsConfig) Option {
	return func(s *Server) {
		selector := newRoutedCluster(config.Default, config.Readers, s.cluster)
		s.cluster = selector
	}
}

func newRoutedCluster(defaultName string, readers map[string]ClusterReader, fallback ClusterReader) *routedCluster {
	out := &routedCluster{
		defaultName: strings.TrimSpace(defaultName),
		readers:     make(map[string]ClusterReader, len(readers)+1),
	}

	for name, reader := range readers {
		normalized := strings.TrimSpace(name)
		if normalized == "" || reader == nil {
			continue
		}
		out.readers[normalized] = reader
	}

	if fallback != nil {
		if out.defaultName == "" {
			out.defaultName = "default"
		}
		if _, exists := out.readers[out.defaultName]; !exists {
			out.readers[out.defaultName] = fallback
		}
	}

	if out.defaultName == "" {
		for name := range out.readers {
			out.defaultName = name
			break
		}
	}

	return out
}

func (r *routedCluster) DefaultName() string {
	return r.defaultName
}

func (r *routedCluster) Names() []string {
	names := make([]string, 0, len(r.readers))
	for name := range r.readers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (r *routedCluster) WithClusterName(ctx context.Context, name string) context.Context {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ctx
	}
	if _, ok := r.readers[trimmed]; !ok {
		return ctx
	}
	return context.WithValue(ctx, clusterContextKey{}, trimmed)
}

func (r *routedCluster) ClusterName(ctx context.Context) string {
	if value, ok := ctx.Value(clusterContextKey{}).(string); ok {
		if _, exists := r.readers[value]; exists {
			return value
		}
	}
	return r.defaultName
}

func (r *routedCluster) ClusterInfo(name string) (model.ClusterContext, bool) {
	reader, ok := r.readers[name]
	if !ok {
		return model.ClusterContext{}, false
	}
	return model.ClusterContext{
		Name:          name,
		IsRealCluster: reader.IsRealCluster(),
	}, true
}

func (r *routedCluster) selectReader(ctx context.Context) ClusterReader {
	if len(r.readers) == 0 {
		return nil
	}

	name := r.ClusterName(ctx)
	if reader, ok := r.readers[name]; ok {
		return reader
	}
	return r.readers[r.defaultName]
}

func (r *routedCluster) IsRealCluster() bool {
	reader := r.readers[r.defaultName]
	if reader == nil {
		return false
	}
	return reader.IsRealCluster()
}

func (r *routedCluster) Snapshot(ctx context.Context) ([]model.PodSummary, []model.NodeSummary) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return nil, nil
	}
	return reader.Snapshot(ctx)
}

func (r *routedCluster) ListNamespaces(ctx context.Context) []string {
	reader := r.selectReader(ctx)
	if reader == nil {
		return nil
	}
	return reader.ListNamespaces(ctx)
}

func (r *routedCluster) ListResources(ctx context.Context, kind string) ([]model.ResourceRecord, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return nil, nil
	}
	return reader.ListResources(ctx, kind)
}

func (r *routedCluster) ListClusterEvents(ctx context.Context) []model.K8sEvent {
	reader := r.selectReader(ctx)
	if reader == nil {
		return nil
	}
	return reader.ListClusterEvents(ctx)
}

func (r *routedCluster) GetResourceYAML(ctx context.Context, kind, namespace, name string) (string, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return "", nil
	}
	return reader.GetResourceYAML(ctx, kind, namespace, name)
}

func (r *routedCluster) ApplyResourceYAML(ctx context.Context, kind, namespace, name, manifestYAML string) (model.ActionResult, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return model.ActionResult{}, nil
	}
	return reader.ApplyResourceYAML(ctx, kind, namespace, name, manifestYAML)
}

func (r *routedCluster) ScaleResource(ctx context.Context, kind, namespace, name string, replicas int32) (model.ActionResult, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return model.ActionResult{}, nil
	}
	return reader.ScaleResource(ctx, kind, namespace, name, replicas)
}

func (r *routedCluster) RestartResource(ctx context.Context, kind, namespace, name string) (model.ActionResult, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return model.ActionResult{}, nil
	}
	return reader.RestartResource(ctx, kind, namespace, name)
}

func (r *routedCluster) RollbackResource(ctx context.Context, kind, namespace, name string) (model.ActionResult, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return model.ActionResult{}, nil
	}
	return reader.RollbackResource(ctx, kind, namespace, name)
}

func (r *routedCluster) PodEvents(ctx context.Context, namespace, name string) []model.K8sEvent {
	reader := r.selectReader(ctx)
	if reader == nil {
		return nil
	}
	return reader.PodEvents(ctx, namespace, name)
}

func (r *routedCluster) PodLogs(ctx context.Context, namespace, name, container string, lines int) string {
	reader := r.selectReader(ctx)
	if reader == nil {
		return ""
	}
	return reader.PodLogs(ctx, namespace, name, container, lines)
}

func (r *routedCluster) StreamPodLogs(
	ctx context.Context,
	namespace,
	name,
	container string,
	tailLines int,
	follow bool,
) (io.ReadCloser, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return nil, nil
	}
	return reader.StreamPodLogs(ctx, namespace, name, container, tailLines, follow)
}

func (r *routedCluster) PodDetail(ctx context.Context, namespace, name string) (model.PodDetail, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return model.PodDetail{}, nil
	}
	return reader.PodDetail(ctx, namespace, name)
}

func (r *routedCluster) NodeDetail(ctx context.Context, name string) (model.NodeDetail, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return model.NodeDetail{}, nil
	}
	return reader.NodeDetail(ctx, name)
}

func (r *routedCluster) CreatePod(ctx context.Context, req model.PodCreateRequest) (model.ActionResult, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return model.ActionResult{}, nil
	}
	return reader.CreatePod(ctx, req)
}

func (r *routedCluster) RestartPod(ctx context.Context, namespace, name string) (model.ActionResult, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return model.ActionResult{}, nil
	}
	return reader.RestartPod(ctx, namespace, name)
}

func (r *routedCluster) DeletePod(ctx context.Context, namespace, name string) (model.ActionResult, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return model.ActionResult{}, nil
	}
	return reader.DeletePod(ctx, namespace, name)
}

func (r *routedCluster) CordonNode(ctx context.Context, name string) (model.ActionResult, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return model.ActionResult{}, nil
	}
	return reader.CordonNode(ctx, name)
}

func (r *routedCluster) UncordonNode(ctx context.Context, name string) (model.ActionResult, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return model.ActionResult{}, errors.New("cluster reader is unavailable")
	}
	provider, ok := reader.(nodeMaintenanceWriter)
	if !ok {
		return model.ActionResult{}, errors.New("node maintenance actions are not supported by the selected cluster")
	}
	return provider.UncordonNode(ctx, name)
}

func (r *routedCluster) DrainNodePreview(ctx context.Context, name string) (model.NodeDrainPreview, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return model.NodeDrainPreview{}, errors.New("cluster reader is unavailable")
	}
	provider, ok := reader.(nodeMaintenanceReader)
	if !ok {
		return model.NodeDrainPreview{}, errors.New("node drain preview is not supported by the selected cluster")
	}
	return provider.DrainNodePreview(ctx, name)
}

func (r *routedCluster) DrainNode(ctx context.Context, name string, force bool) (model.ActionResult, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return model.ActionResult{}, errors.New("cluster reader is unavailable")
	}
	provider, ok := reader.(nodeMaintenanceWriter)
	if !ok {
		return model.ActionResult{}, errors.New("node maintenance actions are not supported by the selected cluster")
	}
	return provider.DrainNode(ctx, name, force)
}

func (r *routedCluster) NodePods(ctx context.Context, name string) ([]model.PodSummary, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return nil, errors.New("cluster reader is unavailable")
	}
	provider, ok := reader.(interface {
		NodePods(context.Context, string) ([]model.PodSummary, error)
	})
	if !ok {
		pods, _ := reader.Snapshot(ctx)
		out := make([]model.PodSummary, 0, len(pods))
		for _, pod := range pods {
			if pod.NodeName == name {
				out = append(out, pod)
			}
		}
		return out, nil
	}
	return provider.NodePods(ctx, name)
}

func (r *routedCluster) NodeEvents(ctx context.Context, name string) ([]model.K8sEvent, error) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return nil, errors.New("cluster reader is unavailable")
	}
	provider, ok := reader.(interface {
		NodeEvents(context.Context, string) ([]model.K8sEvent, error)
	})
	if !ok {
		events := reader.ListClusterEvents(ctx)
		out := make([]model.K8sEvent, 0, len(events))
		for _, event := range events {
			if strings.EqualFold(event.ResourceKind, "Node") && event.Resource == name {
				out = append(out, event)
			}
		}
		return out, nil
	}
	return provider.NodeEvents(ctx, name)
}

func (r *routedCluster) StateSnapshot(ctx context.Context) (state.ClusterState, bool) {
	reader := r.selectReader(ctx)
	if reader == nil {
		return state.ClusterState{}, false
	}
	return reader.StateSnapshot(ctx)
}

func (s *Server) clusterMiddleware(next http.Handler) http.Handler {
	selector, ok := s.cluster.(clusterSelector)
	if !ok {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clusterName := strings.TrimSpace(r.Header.Get("X-KubeLens-Cluster"))
		if clusterName == "" {
			if cookie, err := r.Cookie(clusterCookieName); err == nil {
				clusterName = strings.TrimSpace(cookie.Value)
			}
		}
		ctx := selector.WithClusterName(r.Context(), clusterName)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) handleClusters(w http.ResponseWriter, r *http.Request) {
	selector, ok := s.cluster.(clusterSelector)
	if !ok {
		writeJSON(w, http.StatusOK, model.ClusterContextList{
			Selected: "default",
			Items: []model.ClusterContext{
				{Name: "default", IsRealCluster: s.cluster.IsRealCluster()},
			},
		})
		return
	}

	names := selector.Names()
	items := make([]model.ClusterContext, 0, len(names))
	for _, name := range names {
		if info, ok := selector.ClusterInfo(name); ok {
			items = append(items, info)
		}
	}

	writeJSON(w, http.StatusOK, model.ClusterContextList{
		Selected: selector.ClusterName(r.Context()),
		Items:    items,
	})
}

func (s *Server) handleSelectCluster(w http.ResponseWriter, r *http.Request) {
	selector, ok := s.cluster.(clusterSelector)
	if !ok {
		writeError(w, http.StatusBadRequest, "multi-cluster is not configured")
		return
	}

	var req model.ClusterSelectRequest
	if err := s.decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "cluster name is required")
		return
	}
	if _, exists := selector.ClusterInfo(name); !exists {
		writeError(w, http.StatusNotFound, "cluster context not found")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     clusterCookieName,
		Value:    name,
		Path:     "/",
		MaxAge:   24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil,
	})

	writeJSON(w, http.StatusOK, model.ClusterSelectResponse{Selected: name})
}
