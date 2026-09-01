package mediaasset

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

const (
	defaultListLimit    = 20
	maxListLimit        = 100
	compensationTimeout = 5 * time.Second
)

type Service struct {
	projects project.Repository
	repo     Repository
	storage  ObjectStorage
	now      func() time.Time
}

func NewService(projects project.Repository, repo Repository, storage ObjectStorage) *Service {
	return &Service{projects: projects, repo: repo, storage: storage, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Store(ctx context.Context, principal project.Principal, projectID uuid.UUID, input CreateInput) (MediaAsset, error) {
	if principal.OwnerID == uuid.Nil {
		return MediaAsset{}, ErrUnauthenticated
	}
	if projectID == uuid.Nil {
		return MediaAsset{}, ErrNotFound
	}
	if err := input.NormalizeAndValidate(); err != nil {
		return MediaAsset{}, err
	}
	if _, err := s.projects.Get(ctx, principal.OwnerID, projectID); err != nil {
		return MediaAsset{}, mapRepositoryError(err)
	}
	if err := ctx.Err(); err != nil {
		return MediaAsset{}, err
	}

	assetID := uuid.New()
	objectKey := fmt.Sprintf("projects/%s/assets/%s", projectID, assetID)
	digest := &countingHash{Hash: sha256.New()}
	_, err := s.storage.Put(ctx, PutObjectInput{
		Key:         objectKey,
		Body:        io.TeeReader(input.Reader, digest),
		ContentType: input.MimeType,
	})
	if err != nil {
		return MediaAsset{}, mapStorageError(err)
	}

	asset := MediaAsset{
		ID:               assetID,
		OwnerID:          principal.OwnerID,
		ProjectID:        projectID,
		Kind:             input.Kind,
		Origin:           input.Origin,
		ObjectKey:        objectKey,
		MimeType:         input.MimeType,
		ByteSize:         digest.Size,
		SHA256:           hex.EncodeToString(digest.Sum(nil)),
		OriginalFilename: input.OriginalFilename,
		Metadata:         append([]byte(nil), input.Metadata...),
		CreatedAt:        s.now(),
		UpdatedAt:        s.now(),
	}
	if err := asset.Validate(); err != nil {
		_ = s.compensateDelete(objectKey)
		return MediaAsset{}, newPersistenceFailureError(err)
	}

	created, err := s.repo.Create(ctx, asset)
	if err != nil {
		_ = s.compensateDelete(objectKey)
		return MediaAsset{}, newPersistenceFailureError(err)
	}
	return cloneAsset(created), nil
}

func (s *Service) compensateDelete(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), compensationTimeout)
	defer cancel()
	return s.storage.Delete(ctx, key)
}

func (s *Service) Create(ctx context.Context, principal project.Principal, projectID uuid.UUID, input CreateInput) (MediaAsset, error) {
	return s.Store(ctx, principal, projectID, input)
}

func (s *Service) Get(ctx context.Context, principal project.Principal, projectID, assetID uuid.UUID) (MediaAsset, error) {
	if principal.OwnerID == uuid.Nil {
		return MediaAsset{}, ErrUnauthenticated
	}
	asset, err := s.repo.Get(ctx, principal.OwnerID, projectID, assetID)
	if err != nil {
		return MediaAsset{}, mapRepositoryError(err)
	}
	return cloneAsset(asset), nil
}

func (s *Service) List(ctx context.Context, principal project.Principal, projectID uuid.UUID, limit int) (ListResult, error) {
	if principal.OwnerID == uuid.Nil {
		return ListResult{}, ErrUnauthenticated
	}
	if limit == 0 {
		limit = defaultListLimit
	}
	if limit < 1 || limit > maxListLimit {
		return ListResult{}, ValidationError{Fields: map[string]string{"limit": "range_1_100"}}
	}
	result, err := s.repo.List(ctx, principal.OwnerID, projectID, ListOptions{Limit: limit})
	if err != nil {
		return ListResult{}, mapRepositoryError(err)
	}
	result.Assets = cloneAssets(result.Assets)
	return result, nil
}

func (s *Service) Open(ctx context.Context, principal project.Principal, projectID, assetID uuid.UUID) (io.ReadCloser, error) {
	asset, err := s.Get(ctx, principal, projectID, assetID)
	if err != nil {
		return nil, err
	}
	reader, err := s.storage.Open(ctx, asset.ObjectKey)
	if err != nil {
		return nil, mapStorageError(err)
	}
	return reader, nil
}

func (s *Service) Delete(ctx context.Context, principal project.Principal, projectID, assetID uuid.UUID) error {
	if principal.OwnerID == uuid.Nil {
		return ErrUnauthenticated
	}
	asset, err := s.repo.Get(ctx, principal.OwnerID, projectID, assetID)
	if err != nil {
		return mapRepositoryError(err)
	}
	if err := s.storage.Delete(ctx, asset.ObjectKey); err != nil && !errors.Is(err, ErrObjectNotFound) {
		return mapStorageError(err)
	}
	if err := s.repo.Delete(ctx, principal.OwnerID, projectID, assetID); err != nil {
		return mapRepositoryError(err)
	}
	return nil
}

type countingHash struct {
	hash.Hash
	Size int64
}

func (h *countingHash) Write(p []byte) (int, error) {
	n, err := h.Hash.Write(p)
	h.Size += int64(n)
	return n, err
}

func cloneAsset(asset MediaAsset) MediaAsset {
	asset.Metadata = append([]byte(nil), asset.Metadata...)
	return asset
}

func cloneAssets(assets []MediaAsset) []MediaAsset {
	cloned := make([]MediaAsset, len(assets))
	for index, asset := range assets {
		cloned[index] = cloneAsset(asset)
	}
	return cloned
}

func mapRepositoryError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, project.ErrNotFound) {
		return ErrNotFound
	}
	return ErrPersistenceFailed
}

func newPersistenceFailureError(cause error) error {
	_ = cause
	return fmt.Errorf("%w", ErrPersistenceFailed)
}

func mapStorageError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, ErrObjectNotFound) {
		return ErrObjectNotFound
	}
	return fmt.Errorf("%w", ErrStorageFailed)
}
