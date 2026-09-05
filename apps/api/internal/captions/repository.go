package captions

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	GetLatest(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int, sceneKey string) (Document, error)
	GetRevision(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int, sceneKey string, revision int) (Document, error)
	ListHistory(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int, sceneKey string) ([]Document, error)
	CreateInitial(ctx context.Context, doc Document) (Document, error)
	CreateRevision(ctx context.Context, doc Document, expectedRevision int) (Document, error)
}
