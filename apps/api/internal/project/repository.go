package project

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("project not found")

type ListCursor struct {
	UpdatedAt time.Time
	ID        uuid.UUID
}

type ListOptions struct {
	Limit  int
	Cursor *ListCursor
}

type ListResult struct {
	Projects   []Project
	NextCursor *ListCursor
}

type Repository interface {
	Create(ctx context.Context, ownerID uuid.UUID, input CreateInput) (Project, error)
	List(ctx context.Context, ownerID uuid.UUID, options ListOptions) (ListResult, error)
	Get(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) (Project, error)
	Update(ctx context.Context, ownerID uuid.UUID, id uuid.UUID, input UpdateInput) (Project, error)
}
