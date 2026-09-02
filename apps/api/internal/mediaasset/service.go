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
	if input.MaxBytes < 0 {
		return MediaAsset{}, ErrInvalidInput
	}

	assetID := uuid.New()
	objectKey := fmt.Sprintf("projects/%s/assets/%s", projectID, assetID)
	digest := &countingHash{Hash: sha256.New()}
	body := input.Reader
	var bounded *maxBytesReader
	if input.MaxBytes > 0 {
		bounded = &maxBytesReader{reader: body, max: input.MaxBytes}
		body = bounded
	}
	_, err := s.storage.Put(ctx, PutObjectInput{
		Key:         objectKey,
		Body:        io.TeeReader(body, digest),
		ContentType: input.MimeType,
	})
	if bounded != nil && bounded.exceeded {
		_ = s.compensateDelete(objectKey)
		return MediaAsset{}, ErrTooLarge
	}
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

func (s *Service) OpenRange(ctx context.Context, principal project.Principal, projectID, assetID uuid.UUID, offset, length int64) (io.ReadCloser, error) {
	if offset < 0 || length < 1 {
		return nil, ErrInvalidInput
	}
	asset, err := s.Get(ctx, principal, projectID, assetID)
	if err != nil {
		return nil, err
	}
	if offset >= asset.ByteSize || length > asset.ByteSize-offset {
		return nil, ErrInvalidInput
	}
	reader, err := s.storage.OpenRange(ctx, asset.ObjectKey, offset, length)
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
	if checker, ok := s.repo.(ReferenceChecker); ok {
		referenced, checkErr := checker.HasReferences(ctx, principal.OwnerID, projectID, assetID)
		if checkErr != nil {
			return mapRepositoryError(checkErr)
		}
		if referenced {
			return ErrInUse
		}
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
	if errors.Is(err, ErrInUse) {
		return ErrInUse
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
	if errors.Is(err, ErrTooLarge) {
		return ErrTooLarge
	}
	return fmt.Errorf("%w", ErrStorageFailed)
}

type maxBytesReader struct {
	reader   io.Reader
	max      int64
	read     int64
	exceeded bool
}

func (r *maxBytesReader) Read(p []byte) (int, error) {
	if r.read > r.max {
		r.exceeded = true
		return 0, ErrTooLarge
	}
	remaining := r.max - r.read
	if remaining == 0 {
		var probe [1]byte
		n, err := r.reader.Read(probe[:])
		if n > 0 {
			return 0, ErrTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > remaining+1 {
		p = p[:remaining+1]
	}
	n, err := r.reader.Read(p)
	r.read += int64(n)
	if r.read > r.max {
		r.exceeded = true
		return n, ErrTooLarge
	}
	return n, err
}
