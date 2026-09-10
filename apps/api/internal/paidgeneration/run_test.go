package paidgeneration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type blockingReleaseGuard struct{}

func (blockingReleaseGuard) Reserve(_ context.Context, ownerID, projectID uuid.UUID, operation Operation, requestID uuid.UUID, policy Policy) (Reservation, error) {
	now := time.Now()
	return Reservation{
		OwnerID:        ownerID,
		ProjectID:      projectID,
		Operation:      operation,
		RequestID:      requestID,
		ReservedAt:     now,
		LeaseToken:     uuid.New(),
		LeaseExpiresAt: now.Add(policy.LeaseDuration),
	}, nil
}

func (blockingReleaseGuard) Renew(_ context.Context, reservation Reservation, extension time.Duration) (Reservation, error) {
	reservation.LeaseExpiresAt = time.Now().Add(extension)
	return reservation, nil
}

func (blockingReleaseGuard) Release(ctx context.Context, _ Reservation) error {
	<-ctx.Done()
	return ctx.Err()
}

type failingReleaseGuard struct {
	releaseErr error
}

func (g failingReleaseGuard) Reserve(_ context.Context, ownerID, projectID uuid.UUID, operation Operation, requestID uuid.UUID, policy Policy) (Reservation, error) {
	now := time.Now()
	return Reservation{
		OwnerID:        ownerID,
		ProjectID:      projectID,
		Operation:      operation,
		RequestID:      requestID,
		ReservedAt:     now,
		LeaseToken:     uuid.New(),
		LeaseExpiresAt: now.Add(policy.LeaseDuration),
	}, nil
}

func (g failingReleaseGuard) Renew(_ context.Context, reservation Reservation, extension time.Duration) (Reservation, error) {
	reservation.LeaseExpiresAt = time.Now().Add(extension)
	return reservation, nil
}

func (g failingReleaseGuard) Release(context.Context, Reservation) error {
	return g.releaseErr
}

func testPolicy() Policy {
	return Policy{
		MaxRequests:   5,
		Window:        time.Hour,
		MaxInFlight:   1,
		LeaseDuration: time.Minute,
	}
}

func TestRunBoundsIndependentReleaseCleanup(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	requestID := uuid.New()
	budget := 25 * time.Millisecond

	started := time.Now()
	err := runWithCleanupTimeout(
		context.Background(),
		blockingReleaseGuard{},
		ownerID,
		projectID,
		OperationImage,
		requestID,
		testPolicy(),
		budget,
		func(context.Context) error { return nil },
	)
	elapsed := time.Since(started)

	if !errors.Is(err, ErrGuardUnavailable) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected bounded cleanup failure, got %v", err)
	}
	if elapsed < budget {
		t.Fatalf("cleanup returned before timeout budget: elapsed=%s budget=%s", elapsed, budget)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("cleanup exceeded bounded return expectation: elapsed=%s", elapsed)
	}
}

func TestRunPreservesWorkErrorWhenReleaseAlsoFails(t *testing.T) {
	workErr := errors.New("provider failed")
	releaseErr := errors.New("release failed")

	err := runWithCleanupTimeout(
		context.Background(),
		failingReleaseGuard{releaseErr: releaseErr},
		uuid.New(),
		uuid.New(),
		OperationNarration,
		uuid.New(),
		testPolicy(),
		25*time.Millisecond,
		func(context.Context) error { return workErr },
	)

	if !errors.Is(err, workErr) {
		t.Fatalf("expected primary work error, got %v", err)
	}
	if errors.Is(err, releaseErr) || errors.Is(err, ErrGuardUnavailable) {
		t.Fatalf("cleanup error must not replace successful detection of work failure: %v", err)
	}
}
