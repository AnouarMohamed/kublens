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
	"time"

	"kubelens-backend/internal/model"
	"nhooyr.io/websocket"
)

func TestRateLimiterBlocksExcessRequests(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithRateLimit(RateLimitConfig{
			Enabled:  true,
			Requests: 1,
			Window:   time.Minute,
		}),
	)
	router := server.Router("")

	first := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	first.RemoteAddr = "10.0.0.1:1234"
	firstResp := httptest.NewRecorder()
	router.ServeHTTP(firstResp, first)
	if firstResp.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", firstResp.Code)
	}

	second := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	second.RemoteAddr = "10.0.0.1:7777"
	secondResp := httptest.NewRecorder()
	router.ServeHTTP(secondResp, second)
	if secondResp.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", secondResp.Code)
	}
}

func TestRateLimiterCanonicalizesHostPortForSameIP(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithRateLimit(RateLimitConfig{
			Enabled:  true,
			Requests: 1,
			Window:   time.Minute,
		}),
	)
	router := server.Router("")

	first := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	first.RemoteAddr = "10.0.0.9:1111"
	firstResp := httptest.NewRecorder()
	router.ServeHTTP(firstResp, first)
	if firstResp.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", firstResp.Code)
	}

	second := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	second.RemoteAddr = "10.0.0.9:2222"
	secondResp := httptest.NewRecorder()
	router.ServeHTTP(secondResp, second)
	if secondResp.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", secondResp.Code)
	}
}

func TestRateLimiterDoesNotTrustForwardedForHeader(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithRateLimit(RateLimitConfig{
			Enabled:  true,
			Requests: 1,
			Window:   time.Minute,
		}),
	)
	router := server.Router("")

	first := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	first.RemoteAddr = "10.0.0.20:1111"
	first.Header.Set("X-Forwarded-For", "198.51.100.10")
	firstResp := httptest.NewRecorder()
	router.ServeHTTP(firstResp, first)
	if firstResp.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", firstResp.Code)
	}

	second := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	second.RemoteAddr = "10.0.0.20:2222"
	second.Header.Set("X-Forwarded-For", "203.0.113.42")
	secondResp := httptest.NewRecorder()
	router.ServeHTTP(secondResp, second)
	if secondResp.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", secondResp.Code)
	}
}

func TestRateLimiterTrustsForwardedForFromAllowedProxyCIDR(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithTrustedProxyCIDRs([]string{"10.0.0.0/8"}),
		WithRateLimit(RateLimitConfig{
			Enabled:  true,
			Requests: 1,
			Window:   time.Minute,
		}),
	)
	router := server.Router("")

	first := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	first.RemoteAddr = "10.7.1.4:1111"
	first.Header.Set("X-Forwarded-For", "198.51.100.10")
	firstResp := httptest.NewRecorder()
	router.ServeHTTP(firstResp, first)
	if firstResp.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", firstResp.Code)
	}

	second := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	second.RemoteAddr = "10.7.1.4:2222"
	second.Header.Set("X-Forwarded-For", "203.0.113.42")
	secondResp := httptest.NewRecorder()
	router.ServeHTTP(secondResp, second)
	if secondResp.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200 when forwarded client ip differs", secondResp.Code)
	}
}

func TestErrorPayloadShapeConsistency(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithAuth(AuthConfig{
			Enabled: true,
			Tokens: []AuthToken{
				{Token: "viewer-token", User: "viewer", Role: "viewer"},
			},
		}),
		WithWriteActionsEnabled(true),
	)
	router := server.Router("")

	req := httptest.NewRequest(http.MethodPost, "/api/pods", strings.NewReader(`{"namespace":"default","name":"demo","image":"nginx:latest"}`))
	req.Header.Set("Authorization", "Bearer viewer-token")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if len(payload) != 1 {
		t.Fatalf("payload keys = %d, want 1", len(payload))
	}
	value, ok := payload["error"].(string)
	if !ok || strings.TrimSpace(value) == "" {
		t.Fatalf("expected non-empty error field, got: %#v", payload)
	}
}

func TestMutationBlockedWhenWriteActionsDisabled(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithAuth(AuthConfig{
			Enabled: true,
			Tokens: []AuthToken{
				{Token: "operator-token", User: "operator", Role: "operator"},
			},
		}),
		WithWriteActionsEnabled(false),
	)
	router := server.Router("")

	req := httptest.NewRequest(http.MethodPost, "/api/pods", strings.NewReader(`{"namespace":"default","name":"demo","image":"nginx:latest"}`))
	req.Header.Set("Authorization", "Bearer operator-token")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
}

func TestRuntimeEndpoint(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithRuntimeStatus(model.RuntimeStatus{
			Mode:                "demo",
			Insecure:            true,
			AuthEnabled:         false,
			WriteActionsEnabled: false,
			PredictorHealthy:    true,
		}),
	)
	router := server.Router("")

	req := httptest.NewRequest(http.MethodGet, "/api/runtime", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var payload model.RuntimeStatus
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode runtime payload: %v", err)
	}
	if payload.Mode != "demo" {
		t.Fatalf("mode = %s, want demo", payload.Mode)
	}
}

func TestSecurityHeadersPresentOnAPIResponses(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(testClusterReader{}, nil, logger)
	router := server.Router("")

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	csp := strings.TrimSpace(rr.Header().Get("Content-Security-Policy"))
	if csp == "" {
		t.Fatal("expected Content-Security-Policy header")
	}
	if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("Content-Security-Policy allows inline scripts: %q", csp)
	}
	if got := rr.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q, want DENY", got)
	}
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rr.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("Strict-Transport-Security = %q, want empty on non-HTTPS request", got)
	}
}

func TestSecurityHeadersSetHSTSForSecureRequests(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(testClusterReader{}, nil, logger)
	router := server.Router("")

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	if got := rr.Header().Get("Strict-Transport-Security"); got != defaultHSTSHeaderValue {
		t.Fatalf("Strict-Transport-Security = %q, want %q", got, defaultHSTSHeaderValue)
	}
}

func TestWebSocketRejectsCrossOriginUpgrade(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(testClusterReader{}, nil, logger)
	httpServer := httptest.NewServer(server.Router(""))
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/stream/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	headers := http.Header{}
	headers.Set("Origin", "https://evil.example")
	conn, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: headers})
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}
	if err == nil {
		t.Fatal("expected websocket dial to fail for cross-origin request")
	}
	if resp == nil {
		t.Fatal("expected HTTP response for rejected websocket upgrade")
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestWebSocketAllowsSameOriginUpgrade(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(testClusterReader{}, nil, logger)
	httpServer := httptest.NewServer(server.Router(""))
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/stream/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	headers := http.Header{}
	headers.Set("Origin", httpServer.URL)
	conn, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		if resp != nil {
			t.Fatalf("websocket dial failed: %v (status=%d)", err, resp.StatusCode)
		}
		t.Fatalf("websocket dial failed: %v", err)
	}
	_ = conn.Close(websocket.StatusNormalClosure, "")
}
