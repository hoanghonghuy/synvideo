package script

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	CreateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, input CreateDraftInput) (Script, error)
	UpdateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, input PutInput) (Script, error)
	Approve(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, revision int) (Script, error)
	GetByVersion(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int) (Script, error)
	ListVersions(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]Script, error)
}
