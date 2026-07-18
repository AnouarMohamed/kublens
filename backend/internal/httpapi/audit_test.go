package httpapi

import (
	"testing"
	"time"

	"kubelens-backend/internal/model"
)

func TestAuditVerificationChecksHMACSignature(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	log := newAuditLog(10, "", "audit-secret", nil)

	entry := log.append(model.AuditEntry{
		Timestamp:  now.Format(time.RFC3339),
		Method:     "POST",
		Path:       "/api/remediation/1/approve",
		Action:     "remediation.approve",
		Status:     200,
		DurationMs: 4,
		Success:    true,
	})
	if entry.Hash == "" {
		t.Fatal("expected audit hash")
	}
	if entry.Signature == "" {
		t.Fatal("expected audit signature")
	}

	verification, ok := log.verify(entry.ID, now)
	if !ok {
		t.Fatal("expected verification result")
	}
	if !verification.OK {
		t.Fatalf("verification failed: %+v", verification)
	}
	if verification.Signature != entry.Signature {
		t.Fatalf("verification signature = %q, want %q", verification.Signature, entry.Signature)
	}

	log.mu.Lock()
	log.items[0].Signature = "tampered"
	log.mu.Unlock()

	verification, ok = log.verify(entry.ID, now)
	if !ok {
		t.Fatal("expected tampered verification result")
	}
	if verification.OK {
		t.Fatal("tampered signature should fail verification")
	}
	if verification.Message != "signature-mismatch" {
		t.Fatalf("message = %q, want signature-mismatch", verification.Message)
	}
}
