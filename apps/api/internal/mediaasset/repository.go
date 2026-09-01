package mediaasset

import (
	"context"

	"github.com/google/uuid"
)

type ListOptions struct{ Limit int }

type ListResult struct{ Assets []MediaAsset }

type Repository interface {
	Create(context.Context, MediaAsset) (MediaAsset, error)
	Get(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (MediaAsset, error)
	List(context.Context, uuid.UUID, uuid.UUID, ListOptions) (ListResult, error)
	Delete(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
}
