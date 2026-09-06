package mediaasset

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type StockOriginRepository interface {
	FindStockOrigin(context.Context, uuid.UUID, uuid.UUID, string, string, Kind) (MediaAsset, error)
}

func (s *Service) FindStockOrigin(ctx context.Context, principal project.Principal, projectID uuid.UUID, providerKey, resultID string, kind Kind) (MediaAsset, error) {
	if principal.OwnerID == uuid.Nil {
		return MediaAsset{}, ErrUnauthenticated
	}
	providerKey = strings.TrimSpace(providerKey)
	resultID = strings.TrimSpace(resultID)
	if projectID == uuid.Nil || providerKey == "" || resultID == "" || !validKind(kind) {
		return MediaAsset{}, ErrInvalidInput
	}
	repo, ok := s.repo.(StockOriginRepository)
	if !ok {
		return MediaAsset{}, ErrPersistenceFailed
	}
	asset, err := repo.FindStockOrigin(ctx, principal.OwnerID, projectID, providerKey, resultID, kind)
	if err != nil {
		return MediaAsset{}, mapRepositoryError(err)
	}
	return cloneAsset(asset), nil
}
