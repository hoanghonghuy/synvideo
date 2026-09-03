package scenenarration

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	GetActive(ctx context.Context, ownerID, projectID uuid.UUID, planVersion int, sceneKey string) (Binding, error)
	ListActiveForPlan(ctx context.Context, ownerID, projectID uuid.UUID, planVersion int) ([]Binding, error)
	Assign(ctx context.Context, ownerID, projectID uuid.UUID, planVersion int, sceneKey string, assetID uuid.UUID) (Binding, error)
	ListHistory(ctx context.Context, ownerID, projectID uuid.UUID, planVersion int, sceneKey string) ([]Binding, error)
}
