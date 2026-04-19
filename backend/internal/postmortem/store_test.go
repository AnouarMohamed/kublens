package postmortem

import (
	"context"
	"strings"
	"testing"
	"time"

	storesql "kubelens-backend/internal/db"
	"kubelens-backend/internal/model"
)

func TestStoreCreateAndConflict(t *testing.T) {
	store := newTestStore(t, 50, func() time.Time {
		return time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	})

	first, err := store.Create(model.Postmortem{
		IncidentID:    "inc-1",
		IncidentTitle: "incident one",
		Method:        model.PostmortemMethodTemplate,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if strings.TrimSpace(first.ID) == "" {
		t.Fatal("postmortem ID should be assigned")
	}

	_, err = store.Create(model.Postmortem{
		IncidentID:    "inc-1",
		IncidentTitle: "incident one duplicate",
		Method:        model.PostmortemMethodTemplate,
	})
	if err == nil {
		t.Fatal("expected duplicate incident conflict")
	}
	if !strings.Contains(err.Error(), "postmortem already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStoreEvictsOldest(t *testing.T) {
	store := newTestStore(t, 2, time.Now)
	first, _ := store.Create(model.Postmortem{IncidentID: "inc-1", IncidentTitle: "one"})
	second, _ := store.Create(model.Postmortem{IncidentID: "inc-2", IncidentTitle: "two"})
	third, _ := store.Create(model.Postmortem{IncidentID: "inc-3", IncidentTitle: "three"})

	if _, ok := store.Get(first.ID); ok {
		t.Fatalf("expected first postmortem %s to be evicted", first.ID)
	}
	if _, ok := store.Get(second.ID); !ok {
		t.Fatalf("expected second postmortem %s", second.ID)
	}
	if _, ok := store.Get(third.ID); !ok {
		t.Fatalf("expected third postmortem %s", third.ID)
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
