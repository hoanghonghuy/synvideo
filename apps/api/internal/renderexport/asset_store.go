package renderexport

import (
	"context"
	"io"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type renderMediaService interface {
	Get(context.Context, project.Principal, uuid.UUID, uuid.UUID) (mediaasset.MediaAsset, error)
	Open(context.Context, project.Principal, uuid.UUID, uuid.UUID) (io.ReadCloser, error)
	Store(context.Context, project.Principal, uuid.UUID, mediaasset.CreateInput) (mediaasset.MediaAsset, error)
	Delete(context.Context, project.Principal, uuid.UUID, uuid.UUID) error
}

type renderMediaRepository interface {
	FindSystemRenderByJob(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (mediaasset.MediaAsset, error)
	FindSystemRenderSubtitleByJob(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (mediaasset.MediaAsset, error)
}

type AssetStore struct {
	service renderMediaService
	repo    renderMediaRepository
}

func NewAssetStore(service renderMediaService, repo renderMediaRepository) *AssetStore {
	return &AssetStore{service: service, repo: repo}
}

func (s *AssetStore) Get(ctx context.Context, principal project.Principal, projectID, assetID uuid.UUID) (mediaasset.MediaAsset, error) {
	return s.service.Get(ctx, principal, projectID, assetID)
}

func (s *AssetStore) Open(ctx context.Context, principal project.Principal, projectID, assetID uuid.UUID) (io.ReadCloser, error) {
	return s.service.Open(ctx, principal, projectID, assetID)
}

func (s *AssetStore) FindFinalByJob(ctx context.Context, principal project.Principal, projectID, jobID uuid.UUID) (mediaasset.MediaAsset, error) {
	if principal.OwnerID == uuid.Nil {
		return mediaasset.MediaAsset{}, mediaasset.ErrUnauthenticated
	}
	return s.repo.FindSystemRenderByJob(ctx, principal.OwnerID, projectID, jobID)
}

func (s *AssetStore) FindSubtitleByJob(ctx context.Context, principal project.Principal, projectID, jobID uuid.UUID) (mediaasset.MediaAsset, error) {
	if principal.OwnerID == uuid.Nil {
		return mediaasset.MediaAsset{}, mediaasset.ErrUnauthenticated
	}
	return s.repo.FindSystemRenderSubtitleByJob(ctx, principal.OwnerID, projectID, jobID)
}

func (s *AssetStore) Store(ctx context.Context, principal project.Principal, projectID uuid.UUID, input mediaasset.CreateInput) (mediaasset.MediaAsset, error) {
	return s.service.Store(ctx, principal, projectID, input)
}

func (s *AssetStore) Delete(ctx context.Context, principal project.Principal, projectID, assetID uuid.UUID) error {
	return s.service.Delete(ctx, principal, projectID, assetID)
}
