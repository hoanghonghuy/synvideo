package generatedimagejob

import (
	"context"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type MediaAssetWriter interface {
	Store(context.Context, project.Principal, uuid.UUID, mediaasset.CreateInput) (mediaasset.MediaAsset, error)
}

type GeneratedAssetFinder interface {
	FindGeneratedByJob(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (mediaasset.MediaAsset, error)
}

type AssetStore struct {
	writer MediaAssetWriter
	finder GeneratedAssetFinder
}

func NewAssetStore(writer MediaAssetWriter, finder GeneratedAssetFinder) *AssetStore {
	return &AssetStore{writer: writer, finder: finder}
}

func (s *AssetStore) FindGeneratedByJob(ctx context.Context, principal project.Principal, projectID, jobID uuid.UUID) (mediaasset.MediaAsset, error) {
	if principal.OwnerID == uuid.Nil {
		return mediaasset.MediaAsset{}, mediaasset.ErrUnauthenticated
	}
	if s.finder == nil {
		return mediaasset.MediaAsset{}, mediaasset.ErrNotFound
	}
	return s.finder.FindGeneratedByJob(ctx, principal.OwnerID, projectID, jobID)
}

func (s *AssetStore) Store(ctx context.Context, principal project.Principal, projectID uuid.UUID, input mediaasset.CreateInput) (mediaasset.MediaAsset, error) {
	return s.writer.Store(ctx, principal, projectID, input)
}
