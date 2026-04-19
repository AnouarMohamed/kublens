package incident

import (
	"context"
	"strings"
	"testing"
	"time"

	storesql "kubelens-backend/internal/db"
	"kubelens-backend/internal/model"
)

func TestIncidentStoreStepTransitions(t *testing.T) {
	now := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	store := newTestStore(t, 50, func() time.Time { return now })
	created := store.Create(model.Incident{
		Title:    "test incident",
		Severity: "warning",
		Runbook: []model.RunbookStep{
			{ID: "step-1", Title: "Step 1", Description: "x", Command: "echo", Status: model.RunbookStepStatusPending},
			{ID: "step-2", Title: "Verify cluster health", Description: "y", Command: "kubectl get pods -A", Status: model.RunbookStepStatusPending, Mandatory: true},
		},
	})

	updated, err := store.PatchStepStatus(created.ID, "step-1", model.RunbookStepStatusInProgress)
	if err != nil {
		t.Fatalf("pending->in_progress should be valid: %v", err)
	}
	if updated.Runbook[0].Status != model.RunbookStepStatusInProgress {
		t.Fatalf("step status = %s, want in_progress", updated.Runbook[0].Status)
	}

	if _, err := store.PatchStepStatus(created.ID, "step-1", model.RunbookStepStatusPending); err == nil {
		t.Fatal("in_progress->pending should be invalid")
	} else if !strings.Contains(err.Error(), "invalid status transition") {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := store.PatchStepStatus(created.ID, "step-2", model.RunbookStepStatusSkipped); err == nil {
		t.Fatal("mandatory final step skip should fail")
	} else if err.Error() != "final verification step cannot be skipped" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIncidentStoreListNewestFirst(t *testing.T) {
	index := 0
	times := []time.Time{
		time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC),
		time.Date(2026, time.March, 10, 12, 1, 0, 0, time.UTC),
	}
	store := newTestStore(t, 50, func() time.Time {
		if index >= len(times) {
			return times[len(times)-1]
		}
		current := times[index]
		index++
		return current
	})

	first := store.Create(model.Incident{Title: "first", Severity: "warning"})
	second := store.Create(model.Incident{Title: "second", Severity: "critical"})
	list := store.List()
	if len(list) != 2 {
		t.Fatalf("list length = %d, want 2", len(list))
	}
	if list[0].ID != second.ID || list[1].ID != first.ID {
		t.Fatalf("list ordering incorrect: %#v", list)
	}
}

func TestIncidentStoreEvictsOldest(t *testing.T) {
	store := newTestStore(t, 2, time.Now)
	first := store.Create(model.Incident{Title: "first", Severity: "warning"})
	second := store.Create(model.Incident{Title: "second", Severity: "warning"})
	third := store.Create(model.Incident{Title: "third", Severity: "warning"})

	list := store.List()
	if len(list) != 2 {
		t.Fatalf("list length = %d, want 2", len(list))
	}
	if _, ok := store.Get(first.ID); ok {
		t.Fatalf("expected first incident %s to be evicted", first.ID)
	}
	if _, ok := store.Get(second.ID); !ok {
		t.Fatalf("expected second incident %s to remain", second.ID)
	}
	if _, ok := store.Get(third.ID); !ok {
		t.Fatalf("expected third incident %s to remain", third.ID)
	}
}

func TestIncidentStoreResolveRequiresCompletedRunbook(t *testing.T) {
	now := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	store := newTestStore(t, 50, func() time.Time { return now })
	created := store.Create(model.Incident{
		Title:    "test incident",
		Severity: "warning",
		Runbook: []model.RunbookStep{
			{ID: "step-1", Title: "Step 1", Description: "x", Status: model.RunbookStepStatusPending},
			{ID: "step-2", Title: "Verify cluster health", Description: "y", Status: model.RunbookStepStatusPending, Mandatory: true},
		},
	})

	if _, err := store.Resolve(created.ID); err == nil {
		t.Fatal("resolve should fail when runbook steps are still pending")
	} else if !strings.Contains(err.Error(), "cannot be resolved") {
		t.Fatalf("unexpected resolve error: %v", err)
	}

	if _, err := store.PatchStepStatus(created.ID, "step-1", model.RunbookStepStatusInProgress); err != nil {
		t.Fatalf("step-1 pending->in_progress failed: %v", err)
	}
	if _, err := store.PatchStepStatus(created.ID, "step-1", model.RunbookStepStatusDone); err != nil {
		t.Fatalf("step-1 in_progress->done failed: %v", err)
	}
	if _, err := store.PatchStepStatus(created.ID, "step-2", model.RunbookStepStatusInProgress); err != nil {
		t.Fatalf("step-2 pending->in_progress failed: %v", err)
	}
	if _, err := store.PatchStepStatus(created.ID, "step-2", model.RunbookStepStatusDone); err != nil {
		t.Fatalf("step-2 in_progress->done failed: %v", err)
	}

	resolved, err := store.Resolve(created.ID)
	if err != nil {
		t.Fatalf("resolve should succeed once runbook is complete: %v", err)
	}
	if resolved.Status != model.IncidentStatusResolved {
		t.Fatalf("status = %s, want resolved", resolved.Status)
	}
}

func newTestStore(t *testing.T, maxItems int, now func() time.Time) *Store {
	t.Helper()

	handle, err := storesql.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	t.Cleanup(func() {
		_ = handle.Close()
	})

	return NewStore(handle, maxItems, now)
}
