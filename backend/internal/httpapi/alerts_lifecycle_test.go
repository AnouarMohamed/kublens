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

	storesql "kubelens-backend/internal/db"
	"kubelens-backend/internal/model"
)

func TestAlertLifecycleEndpoints(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handle, err := storesql.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	t.Cleanup(func() {
		_ = handle.Close()
	})
	server := newServer(
		testClusterReader{},
		nil,
		logger,
		WithSQLiteDB(handle),
		WithAuth(AuthConfig{
			Enabled: true,
			Tokens: []AuthToken{
				{Token: "viewer-token", User: "viewer", Role: "viewer"},
				{Token: "operator-token", User: "operator", Role: "operator"},
			},
		}),
	)
	router := server.Router("")

	t.Run("operator can upsert lifecycle state", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/alerts/lifecycle",
			strings.NewReader(`{"id":"pressure-node-1","node":"node-1","rule":"sustained_pressure","status":"acknowledged"}`),
		)
		req.Header.Set("Authorization", "Bearer operator-token")
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status code = %d, want 200", rr.Code)
		}

		var payload model.NodeAlertLifecycle
		if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
			t.Fatalf("decode lifecycle response: %v", err)
		}
		if payload.Status != model.NodeAlertStatusAcknowledged {
			t.Fatalf("status = %s, want %s", payload.Status, model.NodeAlertStatusAcknowledged)
		}
		if payload.UpdatedBy != "operator" {
			t.Fatalf("updatedBy = %q, want operator", payload.UpdatedBy)
		}
	})

	t.Run("viewer can list lifecycle state", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/alerts/lifecycle", nil)
		req.Header.Set("Authorization", "Bearer viewer-token")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status code = %d, want 200", rr.Code)
		}

		var payload []model.NodeAlertLifecycle
		if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
			t.Fatalf("decode lifecycle list: %v", err)
		}
		if len(payload) == 0 {
			t.Fatal("expected at least one lifecycle item")
		}
		if payload[0].ID != "pressure-node-1" {
			t.Fatalf("id = %q, want pressure-node-1", payload[0].ID)
		}
	})

	t.Run("viewer cannot mutate lifecycle state", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/alerts/lifecycle",
			strings.NewReader(`{"id":"pressure-node-1","node":"node-1","rule":"sustained_pressure","status":"dismissed"}`),
		)
		req.Header.Set("Authorization", "Bearer viewer-token")
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status code = %d, want 403", rr.Code)
		}
	})

	t.Run("invalid snooze payload is rejected", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/alerts/lifecycle",
			strings.NewReader(`{"id":"pressure-node-1","node":"node-1","rule":"sustained_pressure","status":"snoozed","snoozeMinutes":0}`),
		)
		req.Header.Set("Authorization", "Bearer operator-token")
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status code = %d, want 400", rr.Code)
		}
	})
}
