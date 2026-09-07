package sceneeditor

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	GetLatest(ctx context.Context, ownerID, projectID uuid.UUID) (Document, error)
	GetRevision(ctx context.Context, ownerID, projectID uuid.UUID, revision int) (Document, error)
	CreateInitial(ctx context.Context, doc Document) (Document, error)
	CreateRevision(ctx context.Context, doc Document, expectedRevision int) (Document, error)
	CreateSnapshot(ctx context.Context, ownerID uuid.UUID, snapshot Snapshot) (Snapshot, error)
	GetSnapshot(ctx context.Context, ownerID, projectID uuid.UUID, digest string) (Snapshot, error)
}

type DependencyResolver interface {
	State(ctx context.Context, ownerID uuid.UUID, doc Document) ([]DependencyState, error)
}
