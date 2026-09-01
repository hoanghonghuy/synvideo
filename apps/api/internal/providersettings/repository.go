package providersettings

import (
	"context"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

// Repository manages persistence for owner provider settings.
type Repository interface {
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]Setting, error)
	GetByOwnerAndProvider(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID) (Setting, error)
	Save(ctx context.Context, setting Setting, expectedRevision *int) (Setting, error)
	Delete(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, expectedRevision int) error
}
