package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kubelens-backend/internal/model"
	"kubelens-backend/internal/state"
)

func BenchmarkHandleAssistant(b *testing.B) {
	cases := []struct {
		name    string
		message string
	}{
		{name: "health_intent", message: "show cluster health"},
		{name: "diagnose_intent", message: "diagnose payment-gateway"},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			cluster := benchmarkClusterReader{
				pods: []model.PodSummary{
					{
						Name:      "payment-gateway-7f8d9a0b-12345",
						Namespace: "production",
						Status:    model.PodStatusFailed,
						Restarts:  5,
					},
					{
						Name:      "checkout-api-12345",
						Namespace: "production",
						Status:    model.PodStatusRunning,
						Restarts:  0,
					},
				},
				nodes: []model.NodeSummary{
					{Name: "node-1", Status: model.NodeStatusReady},
					{Name: "node-2", Status: model.NodeStatusNotReady},
				},
				events: []model.K8sEvent{{Reason: "BackOff"}},
				logs:   "ERROR dependency dial tcp 10.0.0.12:5432 connection timeout",
			}

			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			server := newServer(cluster, time.Now, logger)
			router := server.Router("")
			body := `{"message":"` + tc.message + `"}`

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				req := httptest.NewRequest(http.MethodPost, "/api/assistant", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, req)
				if rr.Code != http.StatusOK {
					b.Fatalf("unexpected status code: %d", rr.Code)
				}
			}
		})
	}
}

type benchmarkClusterReader struct {
	pods   []model.PodSummary
	nodes  []model.NodeSummary
	events []model.K8sEvent
	logs   string
}

func (b benchmarkClusterReader) IsRealCluster() bool { return true }

func (b benchmarkClusterReader) Snapshot(context.Context) ([]model.PodSummary, []model.NodeSummary) {
	return b.pods, b.nodes
}

func (b benchmarkClusterReader) ListNamespaces(context.Context) []string {
	return []string{"production"}
}

func (b benchmarkClusterReader) ListResources(context.Context, string) ([]model.ResourceRecord, error) {
	return []model.ResourceRecord{{ID: "1", Name: "sample", Status: "ok", Age: "1m"}}, nil
}

func (b benchmarkClusterReader) ListClusterEvents(context.Context) []model.K8sEvent {
	return []model.K8sEvent{{Reason: "BackOff", Type: "Warning", Age: "1m", From: "kubelet", Message: "sample"}}
}

func (b benchmarkClusterReader) GetResourceYAML(context.Context, string, string, string) (string, error) {
	return "apiVersion: apps/v1\nkind: Deployment", nil
}

func (b benchmarkClusterReader) ApplyResourceYAML(context.Context, string, string, string, string) (model.ActionResult, error) {
	return model.ActionResult{Success: true, Message: "applied"}, nil
}

func (b benchmarkClusterReader) ScaleResource(context.Context, string, string, string, int32) (model.ActionResult, error) {
	return model.ActionResult{Success: true, Message: "scaled"}, nil
}

func (b benchmarkClusterReader) RestartResource(context.Context, string, string, string) (model.ActionResult, error) {
	return model.ActionResult{Success: true, Message: "restarted"}, nil
}

func (b benchmarkClusterReader) RollbackResource(context.Context, string, string, string) (model.ActionResult, error) {
	return model.ActionResult{Success: true, Message: "rolled back"}, nil
}

func (b benchmarkClusterReader) PodEvents(context.Context, string, string) []model.K8sEvent {
	return b.events
}

func (b benchmarkClusterReader) PodLogs(context.Context, string, string, string, int) string {
	return b.logs
}

func (b benchmarkClusterReader) StreamPodLogs(context.Context, string, string, string, int, bool) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(b.logs)), nil
}

func (b benchmarkClusterReader) PodDetail(context.Context, string, string) (model.PodDetail, error) {
	return model.PodDetail{}, nil
}

func (b benchmarkClusterReader) NodeDetail(context.Context, string) (model.NodeDetail, error) {
	return model.NodeDetail{}, nil
}

func (b benchmarkClusterReader) CreatePod(context.Context, model.PodCreateRequest) (model.ActionResult, error) {
	return model.ActionResult{Success: true, Message: "created"}, nil
}

func (b benchmarkClusterReader) RestartPod(context.Context, string, string) (model.ActionResult, error) {
	return model.ActionResult{Success: true, Message: "restarted"}, nil
}

func (b benchmarkClusterReader) DeletePod(context.Context, string, string) (model.ActionResult, error) {
	return model.ActionResult{Success: true, Message: "deleted"}, nil
}

func (b benchmarkClusterReader) CordonNode(context.Context, string) (model.ActionResult, error) {
	return model.ActionResult{Success: true, Message: "cordoned"}, nil
}

func (b benchmarkClusterReader) StateSnapshot(context.Context) (state.ClusterState, bool) {
	return state.ClusterState{}, false
}
