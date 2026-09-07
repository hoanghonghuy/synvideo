package scenenarrationjob

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
)

const (
	TemporaryObjectClassNarrationChunk = "narration_chunk"
	DefaultCleanupBatchSize            = 100
	DefaultCleanupPassTimeout          = 30 * time.Second
	DefaultCleanupObjectTimeout        = 5 * time.Second
	DefaultCleanupClaimDuration        = 30 * time.Second
	DefaultCleanupRetryDelay           = 5 * time.Minute
)

type TemporaryObject struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	ProjectID  uuid.UUID
	JobID      uuid.UUID
	ObjectKey  string
	ClaimToken uuid.UUID
}

type TemporaryObjectRepository interface {
	Track(context.Context, TemporaryObject) error
	MarkRemoved(context.Context, uuid.UUID, uuid.UUID, string) error
	ClaimCleanup(context.Context, int, time.Duration) ([]TemporaryObject, error)
	CompleteCleanup(context.Context, uuid.UUID, uuid.UUID) error
	RetryCleanup(context.Context, uuid.UUID, uuid.UUID, string, time.Time) error
}

type ReconcilerConfig struct {
	BatchSize     int
	PassTimeout   time.Duration
	ObjectTimeout time.Duration
	ClaimDuration time.Duration
	RetryDelay    time.Duration
}

type Reconciler struct {
	repo    TemporaryObjectRepository
	storage mediaasset.ObjectStorage
	config  ReconcilerConfig
}

func NewReconciler(repo TemporaryObjectRepository, storage mediaasset.ObjectStorage, config ReconcilerConfig) *Reconciler {
	if config.BatchSize <= 0 || config.BatchSize > DefaultCleanupBatchSize {
		config.BatchSize = DefaultCleanupBatchSize
	}
	if config.PassTimeout <= 0 || config.PassTimeout > DefaultCleanupPassTimeout {
		config.PassTimeout = DefaultCleanupPassTimeout
	}
	if config.ObjectTimeout <= 0 || config.ObjectTimeout > DefaultCleanupObjectTimeout {
		config.ObjectTimeout = DefaultCleanupObjectTimeout
	}
	if config.ClaimDuration <= 0 {
		config.ClaimDuration = DefaultCleanupClaimDuration
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = DefaultCleanupRetryDelay
	}
	return &Reconciler{repo: repo, storage: storage, config: config}
}

func (r *Reconciler) RunOnce(ctx context.Context) error {
	if r == nil || r.repo == nil || r.storage == nil {
		return nil
	}
	passCtx, cancel := context.WithTimeout(ctx, r.config.PassTimeout)
	defer cancel()

	objects, err := r.repo.ClaimCleanup(passCtx, r.config.BatchSize, r.config.ClaimDuration)
	if err != nil {
		return fmt.Errorf("claim temporary object cleanup: %w", err)
	}

	var passErr error
	for _, object := range objects {
		expectedPrefix := fmt.Sprintf("projects/%s/internal_chunks/%s/", object.ProjectID, object.JobID)
		if err := mediaasset.ValidateObjectStorageKey(object.ObjectKey); err != nil || !strings.HasPrefix(object.ObjectKey, expectedPrefix) {
			retryErr := r.repo.RetryCleanup(passCtx, object.ID, object.ClaimToken, "invalid_object_identity", time.Now().UTC().Add(r.config.RetryDelay))
			if err == nil {
				err = errors.New("object key does not match durable project/job identity")
			}
			passErr = errors.Join(passErr, fmt.Errorf("reject unsafe temporary object identity: %w", err), retryErr)
			continue
		}

		objectCtx, objectCancel := context.WithTimeout(passCtx, r.config.ObjectTimeout)
		deleteErr := r.storage.Delete(objectCtx, object.ObjectKey)
		objectCancel()
		if deleteErr != nil && !errors.Is(deleteErr, mediaasset.ErrObjectNotFound) {
			retryErr := r.repo.RetryCleanup(passCtx, object.ID, object.ClaimToken, "object_delete_failed", time.Now().UTC().Add(r.config.RetryDelay))
			passErr = errors.Join(passErr, fmt.Errorf("delete temporary object: %w", deleteErr), retryErr)
			continue
		}
		if err := r.repo.CompleteCleanup(passCtx, object.ID, object.ClaimToken); err != nil {
			passErr = errors.Join(passErr, fmt.Errorf("complete temporary object cleanup: %w", err))
		}
	}
	return passErr
}
