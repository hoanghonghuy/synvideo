package paidgeneration

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

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
	if guard == nil || work == nil || ownerID == uuid.Nil || projectID == uuid.Nil || requestID == uuid.Nil || !policy.Valid() {
		return ErrGuardUnavailable
	}

	reservation, err := guard.Reserve(ctx, ownerID, projectID, operation, requestID, policy)
	if err != nil {
		return err
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
		ticker := timeNewTicker(interval)
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

	// Release with a non-cancelled context: caller cancellation must not leak an
	// in-flight slot until lease expiry. Fencing keeps a stale holder harmless.
	releaseErr := guard.Release(context.Background(), reservation)

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
