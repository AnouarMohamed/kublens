package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kubelens-backend/internal/model"
)

func TestGhostSimulationIsAnnotatedAndStored(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := newServer(testClusterReader{}, nil, logger)
	router := server.Router("")

	simReq := httptest.NewRequest(
		http.MethodPost,
		"/api/ghost/simulations",
		strings.NewReader(`{"action":"node_drain","nodeName":"node-1","horizonSeconds":900}`),
	)
	simReq.Header.Set("Content-Type", "application/json")
	simResp := httptest.NewRecorder()
	router.ServeHTTP(simResp, simReq)
	if simResp.Code != http.StatusOK {
		t.Fatalf("simulation status = %d, want 200", simResp.Code)
	}

	var result model.GhostSimulationResult
	if err := json.NewDecoder(simResp.Body).Decode(&result); err != nil {
		t.Fatalf("decode simulation result: %v", err)
	}
	if result.Engine != "in-memory" {
		t.Fatalf("engine = %q, want in-memory", result.Engine)
	}
	if result.TopologyHash == "" {
		t.Fatal("expected topology hash")
	}
	if result.Confidence <= 0 || result.Confidence > 100 {
		t.Fatalf("confidence = %d, want 1..100", result.Confidence)
	}
	if len(result.Limitations) == 0 {
		t.Fatal("expected simulation limitations")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/ghost/simulations?limit=10", nil)
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("simulation list status = %d, want 200", listResp.Code)
	}
	var list model.GhostSimulationListResponse
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatalf("decode simulation list: %v", err)
	}
	if len(list.Items) == 0 || list.Items[0].ID != result.ID {
		t.Fatalf("expected stored simulation %q in list: %+v", result.ID, list.Items)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/ghost/simulations/"+result.ID, nil)
	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("simulation get status = %d, want 200", getResp.Code)
	}
	var record model.GhostSimulationRecord
	if err := json.NewDecoder(getResp.Body).Decode(&record); err != nil {
		t.Fatalf("decode simulation record: %v", err)
	}
	if record.Result.TopologyHash != result.TopologyHash {
		t.Fatalf("record topology hash = %q, want %q", record.Result.TopologyHash, result.TopologyHash)
	}
}
