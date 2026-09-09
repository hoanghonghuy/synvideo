package jobs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ListCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

type ListByProjectKindOptions struct {
	OwnerID   uuid.UUID
	ProjectID uuid.UUID
	Kind      string
	Limit     int
	Cursor    *ListCursor
}

type Repository interface {
	Enqueue(ctx context.Context, input EnqueueInput) (Job, error)
	GetByID(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) (Job, error)
	GetByIDForProject(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, id uuid.UUID) (Job, error)
	GetByDedupeKey(ctx context.Context, ownerID uuid.UUID, kind string, dedupeKey string) (Job, error)
	ListByProjectKind(ctx context.Context, options ListByProjectKindOptions) ([]Job, *ListCursor, error)
	ClaimNext(ctx context.Context, options ClaimOptions) (Job, error)
	RenewLease(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, extendDuration time.Duration) (Job, error)
	RequestCancel(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, id uuid.UUID) (Job, error)
	IsCancelRequested(ctx context.Context, id uuid.UUID) (bool, error)
	MarkSuccess(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, result json.RawMessage) (Job, error)
	MarkCancelled(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID) (Job, error)
	MarkRetryableFailure(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, errorCode string, nextAvailableAt time.Time) (Job, error)
	MarkTerminalFailure(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, errorCode string) (Job, error)
}
