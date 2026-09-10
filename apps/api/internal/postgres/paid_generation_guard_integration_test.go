package postgres

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/paidgeneration"
)

func TestPaidGenerationGuardExpiredReservationFreesConcurrencySlot(t *testing.T) {
	pool := integrationPool(t)
	guard := NewPaidGenerationGuard(pool)
	now := time.Date(2026, 9, 10, 6, 30, 0, 0, time.UTC)
	guard.now = func() time.Time { return now }
	ownerID := uuid.New()
	projectID := uuid.New()
	policy := paidgeneration.Policy{MaxRequests: 10, Window: time.Hour, MaxInFlight: 1, LeaseDuration: time.Minute}

	first, err := guard.Reserve(context.Background(), ownerID, projectID, paidgeneration.OperationImage, uuid.New(), policy)
	if err != nil {
		t.Fatalf("reserve first request: %v", err)
	}
	if first.LeaseExpiresAt != now.Add(policy.LeaseDuration) {
		t.Fatalf("unexpected first lease expiry: %v", first.LeaseExpiresAt)
	}

	now = now.Add(2 * time.Minute)
	second, err := guard.Reserve(context.Background(), ownerID, projectID, paidgeneration.OperationImage, uuid.New(), policy)
	if err != nil {
		t.Fatalf("expired reservation should not consume concurrency slot: %v", err)
	}
	if second.Replay || second.Recovered {
		t.Fatalf("expected a fresh reservation, got replay=%v recovered=%v", second.Replay, second.Recovered)
	}
}

func TestPaidGenerationGuardRecoversExpiredRequestWithoutDoubleCountingQuota(t *testing.T) {
	pool := integrationPool(t)
	guard := NewPaidGenerationGuard(pool)
	now := time.Date(2026, 9, 10, 7, 0, 0, 0, time.UTC)
	guard.now = func() time.Time { return now }
	ownerID := uuid.New()
	projectID := uuid.New()
	requestID := uuid.New()
	policy := paidgeneration.Policy{MaxRequests: 1, Window: time.Hour, MaxInFlight: 1, LeaseDuration: time.Minute}

	first, err := guard.Reserve(context.Background(), ownerID, projectID, paidgeneration.OperationNarration, requestID, policy)
	if err != nil {
		t.Fatalf("reserve request: %v", err)
	}

	now = now.Add(2 * time.Minute)
	recovered, err := guard.Reserve(context.Background(), ownerID, projectID, paidgeneration.OperationNarration, requestID, policy)
	if err != nil {
		t.Fatalf("recover expired request: %v", err)
	}
	if !recovered.Recovered || recovered.Replay {
		t.Fatalf("expected recovered reservation, got replay=%v recovered=%v", recovered.Replay, recovered.Recovered)
	}
	if recovered.LeaseToken == first.LeaseToken {
		t.Fatal("expected recovered reservation to receive a new fencing token")
	}
	if !recovered.ReservedAt.Equal(first.ReservedAt) {
		t.Fatalf("recovery must preserve original quota timestamp: first=%v recovered=%v", first.ReservedAt, recovered.ReservedAt)
	}

	var rows int
	if err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM paid_generation_reservations
		WHERE owner_id = $1 AND project_id = $2 AND operation_kind = $3 AND request_id = $4
	`, ownerID, projectID, string(paidgeneration.OperationNarration), requestID).Scan(&rows); err != nil {
		t.Fatalf("count reservation rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected one durable quota row after recovery, got %d", rows)
	}

	if _, err := guard.Reserve(context.Background(), ownerID, projectID, paidgeneration.OperationNarration, uuid.New(), policy); !errors.Is(err, paidgeneration.ErrConcurrencyExceeded) && !errors.Is(err, paidgeneration.ErrQuotaExceeded) {
		t.Fatalf("recovered request must still count against the original quota window, got %v", err)
	}
}

func TestPaidGenerationGuardRecoveredLeaseFencesOldHolder(t *testing.T) {
	pool := integrationPool(t)
	guard := NewPaidGenerationGuard(pool)
	now := time.Date(2026, 9, 10, 7, 30, 0, 0, time.UTC)
	guard.now = func() time.Time { return now }
	ownerID := uuid.New()
	projectID := uuid.New()
	requestID := uuid.New()
	policy := paidgeneration.Policy{MaxRequests: 10, Window: time.Hour, MaxInFlight: 1, LeaseDuration: time.Minute}

	oldLease, err := guard.Reserve(context.Background(), ownerID, projectID, paidgeneration.OperationVideo, requestID, policy)
	if err != nil {
		t.Fatalf("reserve old lease: %v", err)
	}
	now = now.Add(2 * time.Minute)
	newLease, err := guard.Reserve(context.Background(), ownerID, projectID, paidgeneration.OperationVideo, requestID, policy)
	if err != nil {
		t.Fatalf("recover lease: %v", err)
	}

	if _, err := guard.Renew(context.Background(), oldLease, policy.LeaseDuration); !errors.Is(err, paidgeneration.ErrLeaseLost) {
		t.Fatalf("expected old holder renewal to be fenced, got %v", err)
	}
	if err := guard.Release(context.Background(), oldLease); err != nil {
		t.Fatalf("stale release should be harmless: %v", err)
	}

	var state string
	var leaseToken uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		SELECT state, lease_token FROM paid_generation_reservations
		WHERE owner_id = $1 AND project_id = $2 AND operation_kind = $3 AND request_id = $4
	`, ownerID, projectID, string(paidgeneration.OperationVideo), requestID).Scan(&state, &leaseToken); err != nil {
		t.Fatalf("read recovered reservation: %v", err)
	}
	if state != "reserved" || leaseToken != newLease.LeaseToken {
		t.Fatalf("stale holder changed recovered lease: state=%s token=%s want=%s", state, leaseToken, newLease.LeaseToken)
	}

	if err := guard.Release(context.Background(), newLease); err != nil {
		t.Fatalf("release current lease: %v", err)
	}
	if err := pool.QueryRow(context.Background(), `
		SELECT state FROM paid_generation_reservations
		WHERE owner_id = $1 AND project_id = $2 AND operation_kind = $3 AND request_id = $4
	`, ownerID, projectID, string(paidgeneration.OperationVideo), requestID).Scan(&state); err != nil {
		t.Fatalf("read released reservation: %v", err)
	}
	if state != "released" {
		t.Fatalf("expected current holder to release reservation, got %s", state)
	}
}

func TestPaidGenerationGuardConcurrentReserveHonorsInFlightLimit(t *testing.T) {
	pool := integrationPool(t)
	guard := NewPaidGenerationGuard(pool)
	now := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	guard.now = func() time.Time { return now }
	ownerID := uuid.New()
	projectID := uuid.New()
	policy := paidgeneration.Policy{MaxRequests: 20, Window: time.Hour, MaxInFlight: 1, LeaseDuration: 5 * time.Minute}

	const workers = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	var successes atomic.Int32
	var rejected atomic.Int32
	errs := make(chan error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-start
			_, err := guard.Reserve(context.Background(), ownerID, projectID, paidgeneration.OperationImage, uuid.New(), policy)
			switch {
			case err == nil:
				successes.Add(1)
			case errors.Is(err, paidgeneration.ErrConcurrencyExceeded):
				rejected.Add(1)
			default:
				errs <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("unexpected reserve error: %v", err)
	}
	if got := successes.Load(); got != 1 {
		t.Fatalf("expected exactly one concurrent reservation, got %d", got)
	}
	if got := rejected.Load(); got != workers-1 {
		t.Fatalf("expected %d concurrency rejections, got %d", workers-1, got)
	}
}
