package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"kubelens-backend/internal/model"
)

func TestAuthRequiresTokenWhenEnabled(t *testing.T) {
	router := newAuthTestServer().Router("")

	req := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want 401", rr.Code)
	}
}

func TestAuthEnforcesRoles(t *testing.T) {
	router := newAuthTestServer().Router("")

	viewerRead := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	viewerRead.Header.Set("Authorization", "Bearer viewer-token")
	viewerReadResp := httptest.NewRecorder()
	router.ServeHTTP(viewerReadResp, viewerRead)
	if viewerReadResp.Code != http.StatusOK {
		t.Fatalf("viewer read status = %d, want 200", viewerReadResp.Code)
	}

	viewerWrite := httptest.NewRequest(http.MethodPost, "/api/pods", strings.NewReader(`{"namespace":"default","name":"demo","image":"nginx:latest"}`))
	viewerWrite.Header.Set("Authorization", "Bearer viewer-token")
	viewerWrite.Header.Set("Content-Type", "application/json")
	viewerWriteResp := httptest.NewRecorder()
	router.ServeHTTP(viewerWriteResp, viewerWrite)
	if viewerWriteResp.Code != http.StatusForbidden {
		t.Fatalf("viewer write status = %d, want 403", viewerWriteResp.Code)
	}

	operatorWrite := httptest.NewRequest(http.MethodPost, "/api/pods", strings.NewReader(`{"namespace":"default","name":"demo","image":"nginx:latest"}`))
	operatorWrite.Header.Set("Authorization", "Bearer operator-token")
	operatorWrite.Header.Set("Content-Type", "application/json")
	operatorWriteResp := httptest.NewRecorder()
	router.ServeHTTP(operatorWriteResp, operatorWrite)
	if operatorWriteResp.Code != http.StatusOK {
		t.Fatalf("operator write status = %d, want 200", operatorWriteResp.Code)
	}
}

func TestAuditEndpointIncludesAuthFailures(t *testing.T) {
	router := newAuthTestServer().Router("")

	okReq := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	okReq.Header.Set("Authorization", "Bearer viewer-token")
	okResp := httptest.NewRecorder()
	router.ServeHTTP(okResp, okReq)
	if okResp.Code != http.StatusOK {
		t.Fatalf("read status = %d, want 200", okResp.Code)
	}

	failReq := httptest.NewRequest(http.MethodPost, "/api/pods", strings.NewReader(`{"namespace":"default","name":"demo","image":"nginx:latest"}`))
	failReq.Header.Set("Authorization", "Bearer viewer-token")
	failReq.Header.Set("Content-Type", "application/json")
	failResp := httptest.NewRecorder()
	router.ServeHTTP(failResp, failReq)
	if failResp.Code != http.StatusForbidden {
		t.Fatalf("write status = %d, want 403", failResp.Code)
	}

	auditReq := httptest.NewRequest(http.MethodGet, "/api/audit?limit=20", nil)
	auditReq.Header.Set("Authorization", "Bearer admin-token")
	auditResp := httptest.NewRecorder()
	router.ServeHTTP(auditResp, auditReq)
	if auditResp.Code != http.StatusOK {
		t.Fatalf("audit status = %d, want 200", auditResp.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(auditResp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode audit payload: %v", err)
	}
	itemsRaw, ok := payload["items"].([]any)
	if !ok || len(itemsRaw) == 0 {
		t.Fatal("expected audit items")
	}
}

func TestAuditVerificationEndpointVerifiesHashChain(t *testing.T) {
	router := newAuthTestServer().Router("")

	readReq := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	readReq.Header.Set("Authorization", "Bearer viewer-token")
	readResp := httptest.NewRecorder()
	router.ServeHTTP(readResp, readReq)
	if readResp.Code != http.StatusOK {
		t.Fatalf("read status = %d, want 200", readResp.Code)
	}

	auditReq := httptest.NewRequest(http.MethodGet, "/api/audit?limit=10", nil)
	auditReq.Header.Set("Authorization", "Bearer admin-token")
	auditResp := httptest.NewRecorder()
	router.ServeHTTP(auditResp, auditReq)
	if auditResp.Code != http.StatusOK {
		t.Fatalf("audit status = %d, want 200", auditResp.Code)
	}

	var auditPayload model.AuditLogResponse
	if err := json.NewDecoder(auditResp.Body).Decode(&auditPayload); err != nil {
		t.Fatalf("decode audit payload: %v", err)
	}
	if len(auditPayload.Items) == 0 {
		t.Fatal("expected audit items")
	}

	entry := auditPayload.Items[0]
	if strings.TrimSpace(entry.Hash) == "" {
		t.Fatalf("expected audit entry hash: %+v", entry)
	}

	verifyReq := httptest.NewRequest(http.MethodGet, "/api/audit/"+entry.ID+"/verify", nil)
	verifyReq.Header.Set("Authorization", "Bearer admin-token")
	verifyResp := httptest.NewRecorder()
	router.ServeHTTP(verifyResp, verifyReq)
	if verifyResp.Code != http.StatusOK {
		t.Fatalf("verify status = %d, want 200", verifyResp.Code)
	}

	var verification model.AuditVerification
	if err := json.NewDecoder(verifyResp.Body).Decode(&verification); err != nil {
		t.Fatalf("decode verification payload: %v", err)
	}
	if !verification.OK {
		t.Fatalf("verification failed: %+v", verification)
	}
	if verification.Hash != entry.Hash {
		t.Fatalf("verification hash = %q, want %q", verification.Hash, entry.Hash)
	}
}

func TestAuthLoginCreatesCookieSession(t *testing.T) {
	router := newAuthTestServer().Router("")

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"viewer-token"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	router.ServeHTTP(loginResp, loginReq)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", loginResp.Code)
	}

	cookies := loginResp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected auth cookie")
	}

	readReq := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	readReq.AddCookie(cookies[0])
	readResp := httptest.NewRecorder()
	router.ServeHTTP(readResp, readReq)
	if readResp.Code != http.StatusOK {
		t.Fatalf("cookie-auth read status = %d, want 200", readResp.Code)
	}
}

func TestAuthLoginSetsSecureCookieWhenForwardedProtoHTTPS(t *testing.T) {
	router := newAuthTestServer().Router("")

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"viewer-token"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("X-Forwarded-Proto", "https")
	loginResp := httptest.NewRecorder()
	router.ServeHTTP(loginResp, loginReq)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", loginResp.Code)
	}

	cookies := loginResp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected auth cookie")
	}
	if !cookies[0].Secure {
		t.Fatal("expected secure auth cookie when forwarded proto is https")
	}
}

func TestAuthLoginRateLimitsFailedAttempts(t *testing.T) {
	now := time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC)
	currentNow := now

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		func() time.Time { return currentNow },
		logger,
		WithWriteActionsEnabled(true),
		WithAuth(AuthConfig{
			Enabled: true,
			Tokens: []AuthToken{
				{Token: "viewer-token", User: "viewer", Role: "viewer"},
			},
		}),
		WithAuthLoginProtection(AuthLoginProtectionConfig{
			Enabled:       true,
			MaxFailures:   2,
			FailureWindow: 10 * time.Minute,
			Lockout:       2 * time.Minute,
			MaxEntries:    100,
		}),
	)
	router := server.Router("")

	for i := 0; i < 2; i++ {
		loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"bad-token"}`))
		loginReq.Header.Set("Content-Type", "application/json")
		loginReq.RemoteAddr = "10.7.1.4:4321"
		loginResp := httptest.NewRecorder()
		router.ServeHTTP(loginResp, loginReq)
		if i == 0 && loginResp.Code != http.StatusUnauthorized {
			t.Fatalf("first failed login status = %d, want 401", loginResp.Code)
		}
		if i == 1 && loginResp.Code != http.StatusTooManyRequests {
			t.Fatalf("second failed login status = %d, want 429", loginResp.Code)
		}
	}

	lockedReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"viewer-token"}`))
	lockedReq.Header.Set("Content-Type", "application/json")
	lockedReq.RemoteAddr = "10.7.1.4:9999"
	lockedResp := httptest.NewRecorder()
	router.ServeHTTP(lockedResp, lockedReq)
	if lockedResp.Code != http.StatusTooManyRequests {
		t.Fatalf("locked login status = %d, want 429", lockedResp.Code)
	}
	if strings.TrimSpace(lockedResp.Header().Get("Retry-After")) == "" {
		t.Fatal("expected retry-after header for locked login")
	}

	currentNow = currentNow.Add(2*time.Minute + time.Second)

	retryReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"viewer-token"}`))
	retryReq.Header.Set("Content-Type", "application/json")
	retryReq.RemoteAddr = "10.7.1.4:10000"
	retryResp := httptest.NewRecorder()
	router.ServeHTTP(retryResp, retryReq)
	if retryResp.Code != http.StatusOK {
		t.Fatalf("login after lockout expiry status = %d, want 200", retryResp.Code)
	}
}

func TestAuthLoginRateLimitDoesNotTrustForwardedForHeader(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithWriteActionsEnabled(true),
		WithAuth(AuthConfig{
			Enabled: true,
			Tokens: []AuthToken{
				{Token: "viewer-token", User: "viewer", Role: "viewer"},
			},
		}),
		WithAuthLoginProtection(AuthLoginProtectionConfig{
			Enabled:       true,
			MaxFailures:   2,
			FailureWindow: 10 * time.Minute,
			Lockout:       2 * time.Minute,
			MaxEntries:    100,
		}),
	)
	router := server.Router("")

	first := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"bad-token"}`))
	first.Header.Set("Content-Type", "application/json")
	first.Header.Set("X-Forwarded-For", "198.51.100.10")
	first.RemoteAddr = "10.7.1.4:4321"
	firstResp := httptest.NewRecorder()
	router.ServeHTTP(firstResp, first)
	if firstResp.Code != http.StatusUnauthorized {
		t.Fatalf("first failed login status = %d, want 401", firstResp.Code)
	}

	second := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"bad-token"}`))
	second.Header.Set("Content-Type", "application/json")
	second.Header.Set("X-Forwarded-For", "203.0.113.42")
	second.RemoteAddr = "10.7.1.4:9999"
	secondResp := httptest.NewRecorder()
	router.ServeHTTP(secondResp, second)
	if secondResp.Code != http.StatusTooManyRequests {
		t.Fatalf("second failed login status = %d, want 429", secondResp.Code)
	}

	locked := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"viewer-token"}`))
	locked.Header.Set("Content-Type", "application/json")
	locked.Header.Set("X-Forwarded-For", "192.0.2.77")
	locked.RemoteAddr = "10.7.1.4:10000"
	lockedResp := httptest.NewRecorder()
	router.ServeHTTP(lockedResp, locked)
	if lockedResp.Code != http.StatusTooManyRequests {
		t.Fatalf("locked login status = %d, want 429", lockedResp.Code)
	}
}

func TestAuthLoginRateLimitUsesForwardedForFromAllowedProxyCIDR(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithWriteActionsEnabled(true),
		WithTrustedProxyCIDRs([]string{"10.0.0.0/8"}),
		WithAuth(AuthConfig{
			Enabled: true,
			Tokens: []AuthToken{
				{Token: "viewer-token", User: "viewer", Role: "viewer"},
			},
		}),
		WithAuthLoginProtection(AuthLoginProtectionConfig{
			Enabled:       true,
			MaxFailures:   2,
			FailureWindow: 10 * time.Minute,
			Lockout:       2 * time.Minute,
			MaxEntries:    100,
		}),
	)
	router := server.Router("")

	first := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"bad-token"}`))
	first.Header.Set("Content-Type", "application/json")
	first.Header.Set("X-Forwarded-For", "198.51.100.10")
	first.RemoteAddr = "10.7.1.4:4321"
	firstResp := httptest.NewRecorder()
	router.ServeHTTP(firstResp, first)
	if firstResp.Code != http.StatusUnauthorized {
		t.Fatalf("first failed login status = %d, want 401", firstResp.Code)
	}

	second := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"viewer-token"}`))
	second.Header.Set("Content-Type", "application/json")
	second.Header.Set("X-Forwarded-For", "203.0.113.42")
	second.RemoteAddr = "10.7.1.4:9999"
	secondResp := httptest.NewRecorder()
	router.ServeHTTP(secondResp, second)
	if secondResp.Code != http.StatusOK {
		t.Fatalf("second login status = %d, want 200 for different forwarded client ip", secondResp.Code)
	}
}

func TestAuthBlocksHeaderTokenWhenDisabled(t *testing.T) {
	router := newAuthTestServer().Router("")

	req := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	req.Header.Set("X-Auth-Token", "viewer-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want 401", rr.Code)
	}
}

func TestAuthAllowsHeaderTokenWhenEnabled(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithAuth(AuthConfig{
			Enabled:          true,
			AllowHeaderToken: true,
			Tokens: []AuthToken{
				{Token: "viewer-token", User: "viewer", Role: "viewer"},
			},
		}),
	)
	router := server.Router("")

	req := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	req.Header.Set("X-Auth-Token", "viewer-token")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", rr.Code)
	}
}

func TestCookieMutationRequiresSameOrigin(t *testing.T) {
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
	)
	router := server.Router("")

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"operator-token"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	router.ServeHTTP(loginResp, loginReq)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", loginResp.Code)
	}
	cookies := loginResp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected auth cookie")
	}

	mutateReq := httptest.NewRequest(http.MethodPost, "/api/pods", strings.NewReader(`{"namespace":"default","name":"demo","image":"nginx:latest"}`))
	mutateReq.Header.Set("Content-Type", "application/json")
	mutateReq.Header.Set("Origin", "https://evil.example")
	mutateReq.AddCookie(cookies[0])
	mutateResp := httptest.NewRecorder()
	router.ServeHTTP(mutateResp, mutateReq)
	if mutateResp.Code != http.StatusForbidden {
		t.Fatalf("mutation status = %d, want 403", mutateResp.Code)
	}
}

func TestCookieMutationAllowsSameOrigin(t *testing.T) {
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
	)
	router := server.Router("")

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"operator-token"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	router.ServeHTTP(loginResp, loginReq)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", loginResp.Code)
	}
	cookies := loginResp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected auth cookie")
	}

	mutateReq := httptest.NewRequest(http.MethodPost, "/api/pods", strings.NewReader(`{"namespace":"default","name":"demo","image":"nginx:latest"}`))
	mutateReq.Host = "example.com"
	mutateReq.Header.Set("Content-Type", "application/json")
	mutateReq.Header.Set("Origin", "https://example.com")
	mutateReq.AddCookie(cookies[0])
	mutateResp := httptest.NewRecorder()
	router.ServeHTTP(mutateResp, mutateReq)
	if mutateResp.Code != http.StatusOK {
		t.Fatalf("mutation status = %d, want 200", mutateResp.Code)
	}
}

func TestCookieMutationRejectsMissingOriginAndReferer(t *testing.T) {
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
	)
	router := server.Router("")

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"operator-token"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	router.ServeHTTP(loginResp, loginReq)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", loginResp.Code)
	}
	cookies := loginResp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected auth cookie")
	}

	mutateReq := httptest.NewRequest(http.MethodPost, "/api/pods", strings.NewReader(`{"namespace":"default","name":"demo","image":"nginx:latest"}`))
	mutateReq.Header.Set("Content-Type", "application/json")
	mutateReq.AddCookie(cookies[0])
	mutateResp := httptest.NewRecorder()
	router.ServeHTTP(mutateResp, mutateReq)

	if mutateResp.Code != http.StatusForbidden {
		t.Fatalf("mutation status = %d, want 403", mutateResp.Code)
	}
}

func TestStreamRejectsQueryTokenAuthentication(t *testing.T) {
	router := newAuthTestServer().Router("")

	req := httptest.NewRequest(http.MethodGet, "/api/stream?token=viewer-token", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want 401", rr.Code)
	}
}

func TestHealthEndpointsBypassAuth(t *testing.T) {
	router := newAuthTestServer().Router("")

	for _, path := range []string{"/api/healthz", "/api/readyz", "/api/openapi.yaml"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status code = %d, want 200", path, rr.Code)
		}
	}
}

func TestAuditCapturesMutatingActions(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithWriteActionsEnabled(true),
		WithAuth(AuthConfig{
			Enabled: true,
			Tokens: []AuthToken{
				{Token: "admin-token", User: "admin", Role: "admin"},
			},
		}),
	)
	router := server.Router("")

	requests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/pods", body: `{"namespace":"default","name":"demo","image":"nginx:latest"}`},
		{method: http.MethodPost, path: "/api/pods/default/demo/restart"},
		{method: http.MethodDelete, path: "/api/pods/default/demo"},
		{method: http.MethodPost, path: "/api/nodes/node-1/cordon"},
		{method: http.MethodPut, path: "/api/resources/deployments/default/demo/yaml", body: `{"yaml":"apiVersion: apps/v1\nkind: Deployment"}`},
		{method: http.MethodPost, path: "/api/resources/deployments/default/demo/scale", body: `{"replicas":2}`},
		{method: http.MethodPost, path: "/api/resources/deployments/default/demo/restart"},
		{method: http.MethodPost, path: "/api/resources/deployments/default/demo/rollback"},
	}

	for _, item := range requests {
		req := httptest.NewRequest(item.method, item.path, strings.NewReader(item.body))
		req.Header.Set("Authorization", "Bearer admin-token")
		if item.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d, want 200", item.method, item.path, rr.Code)
		}
	}

	auditReq := httptest.NewRequest(http.MethodGet, "/api/audit?limit=100", nil)
	auditReq.Header.Set("Authorization", "Bearer admin-token")
	auditResp := httptest.NewRecorder()
	router.ServeHTTP(auditResp, auditReq)
	if auditResp.Code != http.StatusOK {
		t.Fatalf("audit status = %d, want 200", auditResp.Code)
	}

	var payload model.AuditLogResponse
	if err := json.NewDecoder(auditResp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode audit payload: %v", err)
	}

	actions := make([]string, 0, len(payload.Items))
	for _, item := range payload.Items {
		if item.Success {
			actions = append(actions, item.Action)
		}
	}

	expected := []string{
		"pod.create",
		"pod.restart",
		"pod.delete",
		"node.cordon",
		"resource.apply",
		"resource.scale",
		"resource.restart",
		"resource.rollback",
	}
	for _, action := range expected {
		if !slices.Contains(actions, action) {
			t.Fatalf("expected audit action %q in successful entries: %v", action, actions)
		}
	}
}

func TestAuditSanitizesClientIPAndDoesNotLeakTokens(t *testing.T) {
	router := newAuthTestServer().Router("")

	unauthorized := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	unauthorized.RemoteAddr = "10.9.0.10:9876"
	unauthorized.Header.Set("Authorization", "Bearer super-secret-token")
	unauthorizedResp := httptest.NewRecorder()
	router.ServeHTTP(unauthorizedResp, unauthorized)
	if unauthorizedResp.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want 401", unauthorizedResp.Code)
	}

	auditReq := httptest.NewRequest(http.MethodGet, "/api/audit?limit=20", nil)
	auditReq.Header.Set("Authorization", "Bearer admin-token")
	auditResp := httptest.NewRecorder()
	router.ServeHTTP(auditResp, auditReq)
	if auditResp.Code != http.StatusOK {
		t.Fatalf("audit status = %d, want 200", auditResp.Code)
	}

	var payload model.AuditLogResponse
	if err := json.NewDecoder(auditResp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode audit payload: %v", err)
	}
	if len(payload.Items) == 0 {
		t.Fatal("expected audit items")
	}

	found := false
	for _, item := range payload.Items {
		if item.Path == "/api/pods" && item.Action == "unauthenticated" {
			found = true
			if item.ClientIP != "10.9.0.10" {
				t.Fatalf("client ip = %q, want 10.9.0.10", item.ClientIP)
			}
			serialized, _ := json.Marshal(item)
			if strings.Contains(string(serialized), "super-secret-token") {
				t.Fatalf("audit entry leaked token: %s", string(serialized))
			}
			break
		}
	}
	if !found {
		t.Fatal("expected unauthorized /api/pods audit entry")
	}
}

func newAuthTestServer() *Server {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return newServer(
		testClusterReader{},
		nil,
		logger,
		WithWriteActionsEnabled(true),
		WithAuth(AuthConfig{
			Enabled: true,
			Tokens: []AuthToken{
				{Token: "viewer-token", User: "viewer", Role: "viewer"},
				{Token: "operator-token", User: "operator", Role: "operator"},
				{Token: "admin-token", User: "admin", Role: "admin"},
			},
		}),
	)
}
