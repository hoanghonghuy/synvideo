package scenenarrationjob_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarrationjob"
)

type lifecycleRepo struct {
	claimed   []scenenarrationjob.TemporaryObject
	claimLimit int
	completed []uuid.UUID
	retried   []uuid.UUID
	retryCode string
}

func (r *lifecycleRepo) Track(context.Context, scenenarrationjob.TemporaryObject) error { return nil }
func (r *lifecycleRepo) MarkRemoved(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }
func (r *lifecycleRepo) ClaimCleanup(_ context.Context, limit int, _ time.Duration) ([]scenenarrationjob.TemporaryObject, error) {
	r.claimLimit = limit
	return r.claimed, nil
}
func (r *lifecycleRepo) CompleteCleanup(_ context.Context, id, _ uuid.UUID) error {
	r.completed = append(r.completed, id)
	return nil
}
func (r *lifecycleRepo) RetryCleanup(_ context.Context, id, _ uuid.UUID, code string, _ time.Time) error {
	r.retried = append(r.retried, id)
	r.retryCode = code
	return nil
}

type lifecycleStorage struct {
	deleteErr error
	deleted   []string
}

func (s *lifecycleStorage) Put(context.Context, mediaasset.PutObjectInput) (mediaasset.ObjectInfo, error) {
	return mediaasset.ObjectInfo{}, nil
}
func (s *lifecycleStorage) Stat(context.Context, string) (mediaasset.ObjectInfo, error) {
	return mediaasset.ObjectInfo{}, nil
}
func (s *lifecycleStorage) Open(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}
func (s *lifecycleStorage) OpenRange(context.Context, string, int64, int64) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}
func (s *lifecycleStorage) Delete(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return s.deleteErr
}

func TestTemporaryObjectReconciler_CompletesMissingObjectIdempotently(t *testing.T) {
	projectID := uuid.New()
	jobID := uuid.New()
	id := uuid.New()
	repo := &lifecycleRepo{claimed: []scenenarrationjob.TemporaryObject{{
		ID: id, ProjectID: projectID, JobID: jobID,
		ObjectKey: "projects/" + projectID.String() + "/internal_chunks/" + jobID.String() + "/0",
		ClaimToken: uuid.New(),
	}}}
	storage := &lifecycleStorage{deleteErr: mediaasset.ErrObjectNotFound}
	reconciler := scenenarrationjob.NewReconciler(repo, storage, scenenarrationjob.ReconcilerConfig{BatchSize: 10})

	if err := reconciler.RunOnce(context.Background()); err != nil {
		t.Fatalf("reconcile missing object: %v", err)
	}
	if repo.claimLimit != 10 {
		t.Fatalf("expected configured bounded claim size 10, got %d", repo.claimLimit)
	}
	if len(repo.completed) != 1 || repo.completed[0] != id {
		t.Fatalf("expected idempotent cleanup completion, got %#v", repo.completed)
	}
	if len(repo.retried) != 0 {
		t.Fatalf("did not expect retry for missing object, got %#v", repo.retried)
	}
}

func TestTemporaryObjectReconciler_RetriesDeleteFailure(t *testing.T) {
	projectID := uuid.New()
	jobID := uuid.New()
	id := uuid.New()
	repo := &lifecycleRepo{claimed: []scenenarrationjob.TemporaryObject{{
		ID: id, ProjectID: projectID, JobID: jobID,
		ObjectKey: "projects/" + projectID.String() + "/internal_chunks/" + jobID.String() + "/1",
		ClaimToken: uuid.New(),
	}}}
	storage := &lifecycleStorage{deleteErr: errors.New("storage unavailable")}
	reconciler := scenenarrationjob.NewReconciler(repo, storage, scenenarrationjob.ReconcilerConfig{})

	if err := reconciler.RunOnce(context.Background()); err == nil {
		t.Fatal("expected delete failure to remain observable")
	}
	if len(repo.completed) != 0 || len(repo.retried) != 1 || repo.retried[0] != id {
		t.Fatalf("expected one durable retry and no completion; completed=%#v retried=%#v", repo.completed, repo.retried)
	}
	if repo.retryCode != "object_delete_failed" {
		t.Fatalf("unexpected privacy-safe retry code %q", repo.retryCode)
	}
}

func TestTemporaryObjectReconciler_RejectsCrossProjectIdentity(t *testing.T) {
	projectID := uuid.New()
	otherProjectID := uuid.New()
	jobID := uuid.New()
	id := uuid.New()
	repo := &lifecycleRepo{claimed: []scenenarrationjob.TemporaryObject{{
		ID: id, ProjectID: projectID, JobID: jobID,
		ObjectKey: "projects/" + otherProjectID.String() + "/internal_chunks/" + jobID.String() + "/0",
		ClaimToken: uuid.New(),
	}}}
	storage := &lifecycleStorage{}
	reconciler := scenenarrationjob.NewReconciler(repo, storage, scenenarrationjob.ReconcilerConfig{})

	if err := reconciler.RunOnce(context.Background()); err == nil {
		t.Fatal("expected cross-project object identity to be rejected")
	}
	if len(storage.deleted) != 0 {
		t.Fatalf("unsafe object must not be deleted, got %#v", storage.deleted)
	}
	if len(repo.retried) != 1 || repo.retryCode != "invalid_object_identity" {
		t.Fatalf("expected durable invalid-identity retry, got %#v code=%q", repo.retried, repo.retryCode)
	}
}
