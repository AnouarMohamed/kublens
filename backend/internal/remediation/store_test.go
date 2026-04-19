package remediation

import (
	"context"
	"testing"
	"time"

	storesql "kubelens-backend/internal/db"
	"kubelens-backend/internal/model"
)

func TestStoreLifecycle(t *testing.T) {
	now := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	store := newTestStore(t, 100, func() time.Time { return now })

	saved := store.SaveProposals([]model.RemediationProposal{
		{
			Kind:      model.RemediationKindRestartPod,
			Namespace: "production",
			Resource:  "payment-gateway",
			Reason:    "CrashLoop",
			RiskLevel: "low",
		},
	})
	if len(saved) != 1 {
		t.Fatalf("saved length = %d, want 1", len(saved))
	}

	approved, err := store.Approve(saved[0].ID, "alice")
	if err != nil {
		t.Fatalf("approve error: %v", err)
	}
	if approved.Status != "approved" || approved.ApprovedBy != "alice" {
		t.Fatalf("unexpected approved proposal: %+v", approved)
	}

	executed, err := store.MarkExecuted(saved[0].ID, "bob", "Restart triggered")
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if executed.Status != "executed" || executed.ExecutedBy != "bob" {
		t.Fatalf("unexpected executed proposal: %+v", executed)
	}
}

func TestStoreRejectAndCap(t *testing.T) {
	index := 0
	times := []time.Time{
		time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC),
		time.Date(2026, time.March, 10, 12, 1, 0, 0, time.UTC),
		time.Date(2026, time.March, 10, 12, 2, 0, 0, time.UTC),
	}
	store := newTestStore(t, 2, func() time.Time {
		if index >= len(times) {
			return times[len(times)-1]
		}
		value := times[index]
		index++
		return value
	})

	first := store.SaveProposals([]model.RemediationProposal{{Kind: model.RemediationKindCordonNode, Resource: "node-1", RiskLevel: "high"}})[0]
	second := store.SaveProposals([]model.RemediationProposal{{Kind: model.RemediationKindCordonNode, Resource: "node-2", RiskLevel: "high"}})[0]
	third := store.SaveProposals([]model.RemediationProposal{{Kind: model.RemediationKindCordonNode, Resource: "node-3", RiskLevel: "high"}})[0]

	if _, err := store.Reject(second.ID, "viewer", "Not required now"); err != nil {
		t.Fatalf("reject error: %v", err)
	}

	if _, ok := store.Get(first.ID); ok {
		t.Fatalf("expected first proposal %s to be evicted", first.ID)
	}
	if _, ok := store.Get(second.ID); !ok {
		t.Fatalf("expected second proposal %s", second.ID)
	}
	if _, ok := store.Get(third.ID); !ok {
		t.Fatalf("expected third proposal %s", third.ID)
	}
}

func TestStoreSaveProposalsDedupesActiveItems(t *testing.T) {
	now := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	store := newTestStore(t, 100, func() time.Time { return now })

	first := store.SaveProposals([]model.RemediationProposal{
		{
			Kind:         model.RemediationKindRestartPod,
			Namespace:    "production",
			Resource:     "payment-gateway-1",
			Reason:       "Crash loop detected",
			RiskLevel:    "low",
			DryRunResult: "restart would recreate pod",
		},
	})
	if len(first) != 1 {
		t.Fatalf("first save length = %d, want 1", len(first))
	}

	second := store.SaveProposals([]model.RemediationProposal{
		{
			Kind:         model.RemediationKindRestartPod,
			Namespace:    "production",
			Resource:     "payment-gateway-1",
			Reason:       "Crash loop still active",
			RiskLevel:    "low",
			DryRunResult: "restart would recreate pod again",
		},
	})
	if len(second) != 1 {
		t.Fatalf("second save length = %d, want 1", len(second))
	}
	if second[0].ID != first[0].ID {
		t.Fatalf("expected duplicate proposal id %s, got %s", first[0].ID, second[0].ID)
	}
	if second[0].Reason != "Crash loop still active" {
		t.Fatalf("expected latest reason to be retained, got %q", second[0].Reason)
	}

	list := store.List()
	if len(list) != 1 {
		t.Fatalf("store should contain one active deduped proposal, got %d", len(list))
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
