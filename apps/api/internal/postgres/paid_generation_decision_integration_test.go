package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/paidgeneration"
)

func TestPaidGenerationGuardRecordsPrivacySafeAllowAndQuotaDenyDecisions(t *testing.T) {
	pool := integrationPool(t)
	guard := NewPaidGenerationGuard(pool)
	now := time.Date(2026, 9, 10, 18, 0, 0, 0, time.UTC)
	guard.now = func() time.Time { return now }
	ownerID := uuid.New()
	projectID := uuid.New()
	firstRequestID := uuid.New()
	deniedRequestID := uuid.New()
	policy := paidgeneration.Policy{MaxRequests: 1, Window: time.Hour, MaxInFlight: 1, LeaseDuration: time.Minute}

	reservation, err := guard.Reserve(context.Background(), ownerID, projectID, paidgeneration.OperationImage, firstRequestID, policy)
	if err != nil {
		t.Fatalf("reserve allowed request: %v", err)
	}
	if err := guard.Release(context.Background(), reservation); err != nil {
		t.Fatalf("release allowed request: %v", err)
	}

	if _, err := guard.Reserve(context.Background(), ownerID, projectID, paidgeneration.OperationImage, deniedRequestID, policy); !errors.Is(err, paidgeneration.ErrQuotaExceeded) {
		t.Fatalf("expected quota denial, got %v", err)
	}

	rows, err := pool.Query(context.Background(), `
		SELECT request_id, decision, reason, decided_at
		FROM paid_generation_decisions
		WHERE owner_id = $1 AND project_id = $2 AND operation_kind = $3
		ORDER BY id
	`, ownerID, projectID, string(paidgeneration.OperationImage))
	if err != nil {
		t.Fatalf("query paid generation decisions: %v", err)
	}
	defer rows.Close()

	type decision struct {
		requestID uuid.UUID
		decision  string
		reason    string
		decidedAt time.Time
	}
	var got []decision
	for rows.Next() {
		var item decision
		if err := rows.Scan(&item.requestID, &item.decision, &item.reason, &item.decidedAt); err != nil {
			t.Fatalf("scan paid generation decision: %v", err)
		}
		got = append(got, item)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate paid generation decisions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected two audit decisions, got %d", len(got))
	}
	if got[0].requestID != firstRequestID || got[0].decision != "allowed" || got[0].reason != "new" || !got[0].decidedAt.Equal(now) {
		t.Fatalf("unexpected allow decision: %+v", got[0])
	}
	if got[1].requestID != deniedRequestID || got[1].decision != "denied" || got[1].reason != "quota" || !got[1].decidedAt.Equal(now) {
		t.Fatalf("unexpected quota decision: %+v", got[1])
	}
}

func TestPaidGenerationGuardRecordsConcurrencyDenial(t *testing.T) {
	pool := integrationPool(t)
	guard := NewPaidGenerationGuard(pool)
	now := time.Date(2026, 9, 10, 18, 30, 0, 0, time.UTC)
	guard.now = func() time.Time { return now }
	ownerID := uuid.New()
	projectID := uuid.New()
	policy := paidgeneration.Policy{MaxRequests: 10, Window: time.Hour, MaxInFlight: 1, LeaseDuration: time.Minute}

	if _, err := guard.Reserve(context.Background(), ownerID, projectID, paidgeneration.OperationVideo, uuid.New(), policy); err != nil {
		t.Fatalf("reserve first request: %v", err)
	}
	deniedRequestID := uuid.New()
	if _, err := guard.Reserve(context.Background(), ownerID, projectID, paidgeneration.OperationVideo, deniedRequestID, policy); !errors.Is(err, paidgeneration.ErrConcurrencyExceeded) {
		t.Fatalf("expected concurrency denial, got %v", err)
	}

	var decision, reason string
	if err := pool.QueryRow(context.Background(), `
		SELECT decision, reason
		FROM paid_generation_decisions
		WHERE owner_id = $1 AND project_id = $2 AND operation_kind = $3 AND request_id = $4
		ORDER BY id DESC
		LIMIT 1
	`, ownerID, projectID, string(paidgeneration.OperationVideo), deniedRequestID).Scan(&decision, &reason); err != nil {
		t.Fatalf("load concurrency denial decision: %v", err)
	}
	if decision != "denied" || reason != "concurrency" {
		t.Fatalf("unexpected concurrency decision: decision=%q reason=%q", decision, reason)
	}
}
