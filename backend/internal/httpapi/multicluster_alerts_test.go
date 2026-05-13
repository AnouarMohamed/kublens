package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kubelens-backend/internal/model"
	"kubelens-backend/internal/state"
)

func TestClusterSelectionSwitchesActiveContext(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithClusterContexts(ClusterContextsConfig{
			Default: "default",
			Readers: map[string]ClusterReader{
				"default": testClusterReader{},
				"staging": altClusterReader{},
			},
		}),
	)
	router := server.Router("")

	listReq := httptest.NewRequest(http.MethodGet, "/api/clusters", nil)
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("cluster list status = %d, want 200", listResp.Code)
	}

	selectReq := httptest.NewRequest(http.MethodPost, "/api/clusters/select", strings.NewReader(`{"name":"staging"}`))
	selectReq.Header.Set("Content-Type", "application/json")
	selectResp := httptest.NewRecorder()
	router.ServeHTTP(selectResp, selectReq)
	if selectResp.Code != http.StatusOK {
		t.Fatalf("cluster select status = %d, want 200", selectResp.Code)
	}
	cookies := selectResp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected cluster selection cookie")
	}

	infoReq := httptest.NewRequest(http.MethodGet, "/api/cluster-info", nil)
	infoReq.AddCookie(cookies[0])
	infoResp := httptest.NewRecorder()
	router.ServeHTTP(infoResp, infoReq)
	if infoResp.Code != http.StatusOK {
		t.Fatalf("cluster info status = %d, want 200", infoResp.Code)
	}

	var payload model.ClusterInfo
	if err := json.NewDecoder(infoResp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode cluster info: %v", err)
	}
	if payload.IsRealCluster {
		t.Fatal("expected staging cluster to report mock mode")
	}
}

func TestClusterSelectionCookieSecureWithForwardedProto(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithClusterContexts(ClusterContextsConfig{
			Default: "default",
			Readers: map[string]ClusterReader{
				"default": testClusterReader{},
				"staging": altClusterReader{},
			},
		}),
	)
	router := server.Router("")

	selectReq := httptest.NewRequest(http.MethodPost, "/api/clusters/select", strings.NewReader(`{"name":"staging"}`))
	selectReq.Header.Set("Content-Type", "application/json")
	selectReq.Header.Set("X-Forwarded-Proto", "https")
	selectResp := httptest.NewRecorder()
	router.ServeHTTP(selectResp, selectReq)
	if selectResp.Code != http.StatusOK {
		t.Fatalf("cluster select status = %d, want 200", selectResp.Code)
	}
	cookies := selectResp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected cluster selection cookie")
	}
	if !cookies[0].Secure {
		t.Fatal("expected secure cluster selection cookie when request is forwarded over https")
	}
}

func TestAlertDispatchEndpoint(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithWriteActionsEnabled(true),
		WithAuth(AuthConfig{
			Enabled: true,
			Tokens: []AuthToken{
				{Token: "operator-token", User: "operator", Role: "operator"},
			},
		}),
		WithAlertDispatcher(testAlertDispatcher{enabled: true}),
	)
	router := server.Router("")

	req := httptest.NewRequest(http.MethodPost, "/api/alerts/dispatch", strings.NewReader(`{"title":"test","message":"hello","severity":"warning"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer operator-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("alert dispatch status = %d, want 200", rr.Code)
	}

	var payload model.AlertDispatchResponse
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode alert dispatch: %v", err)
	}
	if !payload.Success {
		t.Fatal("expected dispatch success")
	}
}

type altClusterReader struct{}

func (altClusterReader) IsRealCluster() bool { return false }

func (altClusterReader) Snapshot(context.Context) ([]model.PodSummary, []model.NodeSummary) {
	return []model.PodSummary{{Name: "mock-pod", Namespace: "default", Status: model.PodStatusRunning}}, []model.NodeSummary{{Name: "mock-node", Status: model.NodeStatusReady}}
}

func (altClusterReader) ListNamespaces(context.Context) []string {
	return []string{"default"}
}

func (altClusterReader) ListResources(context.Context, string) ([]model.ResourceRecord, error) {
	return []model.ResourceRecord{{ID: "m1", Name: "mock", Status: "ok", Age: "1m"}}, nil
}

func (altClusterReader) ListClusterEvents(context.Context) []model.K8sEvent {
	return []model.K8sEvent{{Reason: "Scheduled", Type: "Normal", Message: "scheduled"}}
}

func (altClusterReader) GetResourceYAML(context.Context, string, string, string) (string, error) {
	return "kind: Deployment", nil
}

func (altClusterReader) ApplyResourceYAML(context.Context, string, string, string, string) (model.ActionResult, error) {
	return model.ActionResult{Success: true, Message: "ok"}, nil
}

func (altClusterReader) ScaleResource(context.Context, string, string, string, int32) (model.ActionResult, error) {
	return model.ActionResult{Success: true, Message: "ok"}, nil
}

func (altClusterReader) RestartResource(context.Context, string, string, string) (model.ActionResult, error) {
	return model.ActionResult{Success: true, Message: "ok"}, nil
}

func (altClusterReader) RollbackResource(context.Context, string, string, string) (model.ActionResult, error) {
	return model.ActionResult{Success: true, Message: "ok"}, nil
}

func (altClusterReader) PodEvents(context.Context, string, string) []model.K8sEvent {
	return nil
}

func (altClusterReader) PodLogs(context.Context, string, string, string, int) string {
	return ""
}

func (altClusterReader) StreamPodLogs(context.Context, string, string, string, int, bool) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (altClusterReader) PodDetail(context.Context, string, string) (model.PodDetail, error) {
	return model.PodDetail{}, nil
}

func (altClusterReader) NodeDetail(context.Context, string) (model.NodeDetail, error) {
	return model.NodeDetail{}, nil
}

func (altClusterReader) CreatePod(context.Context, model.PodCreateRequest) (model.ActionResult, error) {
	return model.ActionResult{Success: true, Message: "ok"}, nil
}

func (altClusterReader) RestartPod(context.Context, string, string) (model.ActionResult, error) {
	return model.ActionResult{Success: true, Message: "ok"}, nil
}

func (altClusterReader) DeletePod(context.Context, string, string) (model.ActionResult, error) {
	return model.ActionResult{Success: true, Message: "ok"}, nil
}

func (altClusterReader) CordonNode(context.Context, string) (model.ActionResult, error) {
	return model.ActionResult{Success: true, Message: "ok"}, nil
}

func (altClusterReader) StateSnapshot(context.Context) (state.ClusterState, bool) {
	return state.ClusterState{}, false
}

type testAlertDispatcher struct {
	enabled bool
}

func (d testAlertDispatcher) Enabled() bool {
	return d.enabled
}

func (d testAlertDispatcher) Dispatch(context.Context, model.AlertDispatchRequest) model.AlertDispatchResponse {
	return model.AlertDispatchResponse{
		Success: true,
		Results: []model.AlertChannelResult{{Channel: "test", Success: true}},
	}
}
