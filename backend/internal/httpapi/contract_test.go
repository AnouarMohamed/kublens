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
)

func TestAPIContractCoreEndpoints(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(testClusterReader{}, nil, logger)
	router := server.Router("")

	tests := []struct {
		name         string
		path         string
		requiredKeys []string
	}{
		{
			name: "healthz",
			path: "/api/healthz",
			requiredKeys: []string{
				"status", "timestamp", "version", "commit",
			},
		},
		{
			name: "readyz",
			path: "/api/readyz",
			requiredKeys: []string{
				"status", "timestamp", "checks", "build",
			},
		},
		{
			name: "enterprise readiness",
			path: "/api/readiness/enterprise",
			requiredKeys: []string{
				"status", "timestamp", "checks", "build",
			},
		},
		{
			name: "version",
			path: "/api/version",
			requiredKeys: []string{
				"version", "commit", "builtAt",
			},
		},
		{
			name: "runtime",
			path: "/api/runtime",
			requiredKeys: []string{
				"mode", "devMode", "insecure", "isRealCluster", "authEnabled",
				"writeActionsEnabled", "databaseDriver", "enterpriseStorage",
				"predictorEnabled", "predictorHealthy", "predictorMode",
				"assistantEnabled", "ragEnabled", "alertsEnabled", "warnings",
			},
		},
		{
			name: "stats",
			path: "/api/stats",
			requiredKeys: []string{
				"pods", "nodes", "cluster",
			},
		},
		{
			name: "diagnostics",
			path: "/api/diagnostics",
			requiredKeys: []string{
				"summary", "timestamp", "criticalIssues", "warningIssues", "healthScore", "issues",
			},
		},
		{
			name: "predictions",
			path: "/api/predictions",
			requiredKeys: []string{
				"source", "generatedAt", "items",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("status code = %d, want 200", rr.Code)
			}

			var payload map[string]any
			if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			assertHasKeys(t, payload, tc.requiredKeys...)
		})
	}
}

func TestAPIContractCollections(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(testClusterReader{}, nil, logger)
	router := server.Router("")

	t.Run("pods list item shape", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status code = %d, want 200", rr.Code)
		}

		var payload []map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(payload) == 0 {
			t.Fatal("expected at least one pod item")
		}
		assertHasKeys(t, payload[0], "name", "namespace", "status", "cpu", "memory", "age", "restarts")
	})

	t.Run("predictions item shape", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/predictions", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status code = %d, want 200", rr.Code)
		}

		var payload map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		assertHasKeys(t, payload, "source", "generatedAt", "items")

		itemsRaw, ok := payload["items"].([]any)
		if !ok || len(itemsRaw) == 0 {
			t.Fatal("expected at least one prediction item")
		}
		first, ok := itemsRaw[0].(map[string]any)
		if !ok {
			t.Fatal("prediction item is not an object")
		}
		assertHasKeys(t, first, "id", "resourceKind", "resource", "riskScore", "confidence", "summary", "recommendation")
	})
}

func TestAPIContractMutatingActionResultShape(t *testing.T) {
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

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "create pod",
			method: http.MethodPost,
			path:   "/api/pods",
			body:   `{"namespace":"default","name":"contract-pod","image":"nginx:latest"}`,
		},
		{
			name:   "restart pod",
			method: http.MethodPost,
			path:   "/api/pods/default/contract-pod/restart",
		},
		{
			name:   "delete pod",
			method: http.MethodDelete,
			path:   "/api/pods/default/contract-pod",
		},
		{
			name:   "cordon node",
			method: http.MethodPost,
			path:   "/api/nodes/node-1/cordon",
		},
		{
			name:   "apply yaml",
			method: http.MethodPut,
			path:   "/api/resources/deployments/default/payment-gateway/yaml",
			body:   `{"yaml":"apiVersion: apps/v1\nkind: Deployment"}`,
		},
		{
			name:   "scale resource",
			method: http.MethodPost,
			path:   "/api/resources/deployments/default/payment-gateway/scale",
			body:   `{"replicas":2}`,
		},
		{
			name:   "restart resource",
			method: http.MethodPost,
			path:   "/api/resources/deployments/default/payment-gateway/restart",
		},
		{
			name:   "rollback resource",
			method: http.MethodPost,
			path:   "/api/resources/deployments/default/payment-gateway/rollback",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer operator-token")
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("status code = %d, want 200", rr.Code)
			}

			var payload map[string]any
			if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			assertHasKeys(t, payload, "success", "message")
		})
	}
}

func TestAPIContractErrorShapeForAuthFailures(t *testing.T) {
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

	tests := []struct {
		name       string
		path       string
		authHeader string
		wantStatus int
	}{
		{
			name:       "missing token",
			path:       "/api/pods",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid token",
			path:       "/api/pods",
			authHeader: "Bearer invalid-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "insufficient role",
			path:       "/api/pods",
			authHeader: "Bearer viewer-token",
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"namespace":"default","name":"x","image":"nginx"}`))
			req.Header.Set("Content-Type", "application/json")
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status code = %d, want %d", rr.Code, tc.wantStatus)
			}

			var payload map[string]any
			if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			assertHasKeys(t, payload, "error")
			if _, ok := payload["success"]; ok {
				t.Fatalf("error payload must not include success key: %#v", payload)
			}
		})
	}
}

func TestAPIContractAuthSessionEdgeCases(t *testing.T) {
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
	)
	router := server.Router("")

	t.Run("session without auth remains unauthenticated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status code = %d, want 200", rr.Code)
		}

		var payload map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		assertHasKeys(t, payload, "enabled", "authenticated", "permissions")
		if payload["enabled"] != true {
			t.Fatalf("enabled = %#v, want true", payload["enabled"])
		}
		if payload["authenticated"] != false {
			t.Fatalf("authenticated = %#v, want false", payload["authenticated"])
		}
		if _, ok := payload["user"]; ok {
			t.Fatalf("expected user to be omitted for unauthenticated session: %#v", payload)
		}
	})

	t.Run("login rejects prefixed bearer token payload", func(t *testing.T) {
		loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"Bearer viewer-token"}`))
		loginReq.Header.Set("Content-Type", "application/json")
		loginResp := httptest.NewRecorder()
		router.ServeHTTP(loginResp, loginReq)
		if loginResp.Code != http.StatusUnauthorized {
			t.Fatalf("status code = %d, want 401", loginResp.Code)
		}

		var payload map[string]any
		if err := json.NewDecoder(loginResp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		assertHasKeys(t, payload, "error")
	})

	t.Run("login session and logout lifecycle contract", func(t *testing.T) {
		loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"viewer-token"}`))
		loginReq.Header.Set("Content-Type", "application/json")
		loginResp := httptest.NewRecorder()
		router.ServeHTTP(loginResp, loginReq)
		if loginResp.Code != http.StatusOK {
			t.Fatalf("login status code = %d, want 200", loginResp.Code)
		}

		var loginPayload map[string]any
		if err := json.NewDecoder(loginResp.Body).Decode(&loginPayload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		assertHasKeys(t, loginPayload, "enabled", "authenticated", "user", "permissions")
		if loginPayload["authenticated"] != true {
			t.Fatalf("authenticated = %#v, want true", loginPayload["authenticated"])
		}

		cookies := loginResp.Result().Cookies()
		if len(cookies) == 0 {
			t.Fatal("expected auth cookie in login response")
		}

		sessionReq := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
		sessionReq.AddCookie(cookies[0])
		sessionResp := httptest.NewRecorder()
		router.ServeHTTP(sessionResp, sessionReq)
		if sessionResp.Code != http.StatusOK {
			t.Fatalf("session status code = %d, want 200", sessionResp.Code)
		}

		var sessionPayload map[string]any
		if err := json.NewDecoder(sessionResp.Body).Decode(&sessionPayload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		assertHasKeys(t, sessionPayload, "enabled", "authenticated", "user", "permissions")
		if sessionPayload["authenticated"] != true {
			t.Fatalf("authenticated = %#v, want true", sessionPayload["authenticated"])
		}
		permsRaw, ok := sessionPayload["permissions"].([]any)
		if !ok {
			t.Fatalf("permissions type = %T, want []any", sessionPayload["permissions"])
		}
		perms := make([]string, 0, len(permsRaw))
		for _, item := range permsRaw {
			if value, ok := item.(string); ok {
				perms = append(perms, value)
			}
		}
		if !slices.Contains(perms, "read") {
			t.Fatalf("permissions = %#v, expected read permission", perms)
		}

		logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", strings.NewReader(`{}`))
		logoutReq.Header.Set("Content-Type", "application/json")
		logoutReq.AddCookie(cookies[0])
		logoutResp := httptest.NewRecorder()
		router.ServeHTTP(logoutResp, logoutReq)
		if logoutResp.Code != http.StatusOK {
			t.Fatalf("logout status code = %d, want 200", logoutResp.Code)
		}

		var logoutPayload map[string]any
		if err := json.NewDecoder(logoutResp.Body).Decode(&logoutPayload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		assertHasKeys(t, logoutPayload, "enabled", "authenticated", "permissions")
		if logoutPayload["authenticated"] != false {
			t.Fatalf("authenticated = %#v, want false", logoutPayload["authenticated"])
		}

		logoutCookies := logoutResp.Result().Cookies()
		if len(logoutCookies) == 0 {
			t.Fatal("expected cleared auth cookie on logout response")
		}
		if logoutCookies[0].MaxAge >= 0 {
			t.Fatalf("logout cookie MaxAge = %d, want negative", logoutCookies[0].MaxAge)
		}

		clearedSessionReq := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
		clearedSessionResp := httptest.NewRecorder()
		router.ServeHTTP(clearedSessionResp, clearedSessionReq)
		if clearedSessionResp.Code != http.StatusOK {
			t.Fatalf("session after logout status code = %d, want 200", clearedSessionResp.Code)
		}

		var clearedSessionPayload map[string]any
		if err := json.NewDecoder(clearedSessionResp.Body).Decode(&clearedSessionPayload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if clearedSessionPayload["authenticated"] != false {
			t.Fatalf("authenticated after logout = %#v, want false", clearedSessionPayload["authenticated"])
		}
	})
}

func assertHasKeys(t *testing.T, payload map[string]any, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing required key %q in payload: %#v", key, payload)
		}
	}
}
