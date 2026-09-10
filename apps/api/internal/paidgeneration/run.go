package paidgeneration

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// cleanupTimeout is a server-owned database cleanup budget. Cleanup must be
// independent of caller cancellation so we make a best effort to free capacity,
// but it must also be bounded because lease expiry is the eventual-recovery path
// when the guard backend is unavailable.
const cleanupTimeout = 2 * time.Second

// Run reserves one logical paid operation, keeps its distributed concurrency
// lease alive while work executes, and releases only the in-flight lease after
// work has reached its durable checkpoint. The durable reservation row remains
// the quota/idempotency record for later retries of the same request ID.
func Run(
	ctx context.Context,
	guard Guard,
	ownerID uuid.UUID,
	projectID uuid.UUID,
	operation Operation,
	requestID uuid.UUID,
	policy Policy,
	work func(context.Context) error,
) error {
	return runWithCleanupTimeout(ctx, guard, ownerID, projectID, operation, requestID, policy, cleanupTimeout, work)
}

func runWithCleanupTimeout(
	ctx context.Context,
	guard Guard,
	ownerID uuid.UUID,
	projectID uuid.UUID,
	operation Operation,
	requestID uuid.UUID,
	policy Policy,
	releaseTimeout time.Duration,
	work func(context.Context) error,
) error {
	if guard == nil || work == nil || ownerID == uuid.Nil || projectID == uuid.Nil || requestID == uuid.Nil || !policy.Valid() || releaseTimeout <= 0 {
		return ErrGuardUnavailable
	}

	reservation, err := guard.Reserve(ctx, ownerID, projectID, operation, requestID, policy)
	if err != nil {
		return withPolicyRetryHint(err, policy)
	}

	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	renewErrCh := make(chan error, 1)
	renewDone := make(chan struct{})
	go func() {
		defer close(renewDone)
		interval := policy.LeaseDuration / 2
		if interval <= 0 {
			interval = policy.LeaseDuration
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-workCtx.Done():
				return
			case <-ticker.C:
				renewed, renewErr := guard.Renew(workCtx, reservation, policy.LeaseDuration)
				if renewErr != nil {
					renewErrCh <- renewErr
					cancel()
					return
				}
				reservation = renewed
			}
		}
	}()

	workErr := work(workCtx)
	cancel()
	<-renewDone

	var renewErr error
	select {
	case renewErr = <-renewErrCh:
	default:
	}

	// Do not inherit caller cancellation here: a cancelled request should still
	// try to free its in-flight slot. Bound the independent cleanup so a stalled
	// pool/network cannot pin the worker forever; lease expiry safely recovers the
	// slot if this best-effort release cannot complete in time.
	releaseCtx, releaseCancel := context.WithTimeout(context.Background(), releaseTimeout)
	releaseErr := guard.Release(releaseCtx, reservation)
	releaseCancel()

	// Preserve the primary execution error. Cleanup failure is only surfaced when
	// work/renewal otherwise succeeded; fencing + lease expiry handle late cleanup.
	if renewErr != nil {
		return errors.Join(ErrLeaseLost, renewErr)
	}
	if workErr != nil {
		return workErr
	}
	if releaseErr != nil {
		return errors.Join(ErrGuardUnavailable, releaseErr)
	}
	return nil
}
