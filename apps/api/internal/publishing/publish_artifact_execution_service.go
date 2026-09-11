package publishing

import (
	"context"
	"errors"
	"io"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/renderexport"
)

const defaultPublishChunkBytes int64 = 8 * 1024 * 1024

var ErrPublishArtifactInvalid = errors.New("publishing render artifact is invalid")

type PublishMediaAssetLookup interface {
	Get(ctx context.Context, ownerID, projectID, assetID uuid.UUID) (mediaasset.MediaAsset, error)
}

type PublishObjectReader interface {
	OpenRange(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error)
}

type ArtifactExecutionService struct {
	attempts  AttemptRepository
	artifacts renderexport.ArtifactLookup
	assets    PublishMediaAssetLookup
	storage   PublishObjectReader
	executor  *PublishExecutionService
	chunkSize int64
}

func NewArtifactExecutionService(attempts AttemptRepository, artifacts renderexport.ArtifactLookup, assets PublishMediaAssetLookup, storage PublishObjectReader, executor *PublishExecutionService) (*ArtifactExecutionService, error) {
	if attempts == nil || artifacts == nil || assets == nil || storage == nil || executor == nil {
		return nil, ErrInvalidModel
	}
	return &ArtifactExecutionService{
		attempts:  attempts,
		artifacts: artifacts,
		assets:    assets,
		storage:   storage,
		executor:  executor,
		chunkSize: defaultPublishChunkBytes,
	}, nil
}

// ExecuteNextChunk resolves the immutable render output from durable identifiers,
// opens exactly the remaining byte range from object storage, and advances the
// existing resumable publish attempt. Callers never supply an arbitrary upload body.
func (s *ArtifactExecutionService) ExecuteNextChunk(ctx context.Context, ownerID, projectID, attemptID uuid.UUID) (PublishAttempt, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil || attemptID == uuid.Nil {
		return PublishAttempt{}, ErrInvalidModel
	}
	attempt, err := s.attempts.GetAttempt(ctx, ownerID, projectID, attemptID)
	if err != nil {
		return PublishAttempt{}, err
	}
	artifact, err := s.artifacts.Get(ctx, ownerID, projectID, attempt.RenderArtifactID)
	if err != nil {
		return PublishAttempt{}, err
	}
	if err := artifact.Validate(); err != nil || artifact.ID != attempt.RenderArtifactID || artifact.OwnerID != ownerID || artifact.ProjectID != projectID {
		return PublishAttempt{}, ErrPublishArtifactInvalid
	}
	asset, err := s.assets.Get(ctx, ownerID, projectID, artifact.MediaAssetID)
	if err != nil {
		return PublishAttempt{}, err
	}
	if asset.ID != artifact.MediaAssetID || asset.OwnerID != ownerID || asset.ProjectID != projectID || asset.ByteSize != artifact.ByteSize || asset.SHA256 != artifact.SHA256 || asset.MimeType != artifact.MimeType {
		return PublishAttempt{}, ErrPublishArtifactInvalid
	}
	if err := mediaasset.ValidateObjectKeyForAsset(asset.ObjectKey, projectID, asset.ID); err != nil {
		return PublishAttempt{}, ErrPublishArtifactInvalid
	}
	if attempt.UploadedBytes < 0 || attempt.UploadedBytes >= artifact.ByteSize {
		return PublishAttempt{}, ErrPublishExecution
	}
	remaining := artifact.ByteSize - attempt.UploadedBytes
	chunkBytes := s.chunkSize
	if remaining < chunkBytes {
		chunkBytes = remaining
	}
	body, err := s.storage.OpenRange(ctx, asset.ObjectKey, attempt.UploadedBytes, chunkBytes)
	if err != nil {
		return PublishAttempt{}, err
	}
	defer body.Close()
	return s.executor.ExecuteChunk(ctx, ownerID, projectID, attemptID, artifact.MimeType, artifact.ByteSize, chunkBytes, body)
}
