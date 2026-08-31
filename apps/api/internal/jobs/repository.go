package jobs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	Enqueue(ctx context.Context, input EnqueueInput) (Job, error)
	GetByID(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) (Job, error)
	GetByIDForProject(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, id uuid.UUID) (Job, error)
	ClaimNext(ctx context.Context, options ClaimOptions) (Job, error)
	RenewLease(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, extendDuration time.Duration) (Job, error)
	MarkSuccess(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, result json.RawMessage) (Job, error)
	MarkRetryableFailure(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, errorCode string, nextAvailableAt time.Time) (Job, error)
	MarkTerminalFailure(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, errorCode string) (Job, error)
}
