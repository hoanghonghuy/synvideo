package creativebrief

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrNotFound           = errors.New("creative brief not found")
	ErrStaleRevision      = errors.New("creative brief stale revision")
	ErrRevisionRequired   = errors.New("creative brief revision is required")
	ErrRevisionUnexpected = errors.New("creative brief revision is only valid for updates")
	ErrUnauthenticated    = errors.New("request principal is required")
)

type Repository interface {
	Get(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) (CreativeBrief, error)
	Put(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, input PutInput) (CreativeBrief, bool, error)
}
