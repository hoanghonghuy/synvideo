package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/paidgeneration"
)

func TestPaidGenerationGuardReleasedReplayReacquiresConcurrencyLease(t *testing.T) {
	pool := integrationPool(t)
	guard := NewPaidGenerationGuard(pool)
	now := time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)
	guard.now = func() time.Time { return now }
	ownerID := uuid.New()
	projectID := uuid.New()
	requestID := uuid.New()
	policy := paidgeneration.Policy{MaxRequests: 10, Window: time.Hour, MaxInFlight: 1, LeaseDuration: time.Minute}

	first, err := guard.Reserve(context.Background(), ownerID, projectID, paidgeneration.OperationImage, requestID, policy)
	if err != nil {
		t.Fatalf("reserve original request: %v", err)
	}
	if err := guard.Release(context.Background(), first); err != nil {
		t.Fatalf("release original request: %v", err)
	}

	replay, err := guard.Reserve(context.Background(), ownerID, projectID, paidgeneration.OperationImage, requestID, policy)
	if err != nil {
		t.Fatalf("reacquire released replay: %v", err)
	}
	if !replay.Replay || replay.Recovered {
		t.Fatalf("expected released request replay, got replay=%v recovered=%v", replay.Replay, replay.Recovered)
	}
	if replay.LeaseToken == first.LeaseToken {
		t.Fatal("released replay must receive a fresh fencing token")
	}
	if !replay.ReservedAt.Equal(first.ReservedAt) {
		t.Fatalf("replay must preserve original quota timestamp: first=%v replay=%v", first.ReservedAt, replay.ReservedAt)
	}

	if _, err := guard.Reserve(context.Background(), ownerID, projectID, paidgeneration.OperationImage, uuid.New(), policy); !errors.Is(err, paidgeneration.ErrConcurrencyExceeded) {
		t.Fatalf("replayed request must occupy the in-flight slot, got %v", err)
	}

	renewed, err := guard.Renew(context.Background(), replay, policy.LeaseDuration)
	if err != nil {
		t.Fatalf("replayed reservation must be renewable: %v", err)
	}
	if !renewed.LeaseExpiresAt.After(replay.LeaseExpiresAt) && !renewed.LeaseExpiresAt.Equal(replay.LeaseExpiresAt) {
		t.Fatalf("unexpected renewed lease expiry: before=%v after=%v", replay.LeaseExpiresAt, renewed.LeaseExpiresAt)
	}
	if err := guard.Release(context.Background(), renewed); err != nil {
		t.Fatalf("replayed reservation must be releasable: %v", err)
	}

	if _, err := guard.Reserve(context.Background(), ownerID, projectID, paidgeneration.OperationImage, uuid.New(), policy); err != nil {
		t.Fatalf("released replay should free in-flight capacity: %v", err)
	}
}
