package httpapi

import (
	"context"
	"testing"
	"time"

	storesql "kubelens-backend/internal/db"
	"kubelens-backend/internal/model"
)

func TestSQLNodeTelemetryStorePersistsSamples(t *testing.T) {
	db, err := storesql.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	store := newSQLNodeTelemetryStore(db, storesql.DialectSQLite, 10, time.Hour)
	store.Save(nodeTelemetrySample{
		agentID:    "agent-a",
		source:     "ebpf-agent",
		capturedAt: now.Add(-time.Second).Format(time.RFC3339),
		receivedAt: now,
		nodes: []model.NodeTelemetryItem{{
			Node:             "node-a",
			Status:           "Ready",
			CPUUsage:         "92%",
			MemoryUsage:      "35%",
			PressureSignals:  []string{"cpu-pressure"},
			ObservedWorkload: 8,
		}},
	}, func() time.Time { return now })

	reloaded := newSQLNodeTelemetryStore(db, storesql.DialectSQLite, 10, time.Hour)
	samples := reloaded.Recent(now.Add(time.Minute))
	if len(samples) != 1 {
		t.Fatalf("samples = %d, want 1", len(samples))
	}
	if samples[0].agentID != "agent-a" || len(samples[0].nodes) != 1 || samples[0].nodes[0].Node != "node-a" {
		t.Fatalf("unexpected sample: %+v", samples[0])
	}
}

func TestSQLNodeTelemetryStorePrunesExpiredSamples(t *testing.T) {
	db, err := storesql.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	store := newSQLNodeTelemetryStore(db, storesql.DialectSQLite, 10, time.Minute)
	store.Save(nodeTelemetrySample{
		agentID:    "agent-a",
		source:     "ebpf-agent",
		capturedAt: now.Add(-2 * time.Minute).Format(time.RFC3339),
		receivedAt: now.Add(-2 * time.Minute),
		nodes: []model.NodeTelemetryItem{{
			Node:   "node-a",
			Status: "Ready",
		}},
	}, func() time.Time { return now.Add(-2 * time.Minute) })

	if samples := store.Recent(now); len(samples) != 0 {
		t.Fatalf("samples = %d, want 0 after prune", len(samples))
	}
}

func TestSQLNodeTelemetryStoreReportsSaveErrors(t *testing.T) {
	db, err := storesql.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close sqlite: %v", err)
	}

	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	store := newSQLNodeTelemetryStore(db, storesql.DialectSQLite, 10, time.Minute)
	err = store.Save(nodeTelemetrySample{
		agentID:    "agent-a",
		source:     "ebpf-agent",
		capturedAt: now.Format(time.RFC3339),
		receivedAt: now,
		nodes: []model.NodeTelemetryItem{{
			Node:   "node-a",
			Status: "Ready",
		}},
	}, func() time.Time { return now })
	if err == nil {
		t.Fatal("expected save error after database close")
	}
}
