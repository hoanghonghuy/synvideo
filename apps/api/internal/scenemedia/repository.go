package scenemedia

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	AssignPrimaryVisual(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int, sceneKey string, assetID uuid.UUID) (Binding, error)
	GetCurrent(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int, sceneKey string) (Binding, error)
	ListCurrent(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int) ([]Binding, error)
	ListHistory(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int, sceneKey string) ([]Binding, error)
}
