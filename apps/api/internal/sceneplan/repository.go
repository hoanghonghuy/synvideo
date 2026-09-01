package sceneplan

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	CreateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, input CreateDraftInput) (Plan, error)
	UpdateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, input PutInput) (Plan, error)
	Approve(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, revision int) (Plan, error)
	GetByVersion(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int) (Plan, error)
	ListVersions(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]Plan, error)
}
