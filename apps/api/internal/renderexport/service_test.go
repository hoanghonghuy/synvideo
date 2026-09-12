package renderexport

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

type snapshotStoreStub struct {
	snapshot  sceneeditor.Snapshot
	err       error
	ownerID   uuid.UUID
	projectID uuid.UUID
	digest    string
}

func (s *snapshotStoreStub) GetSnapshot(_ context.Context, ownerID, projectID uuid.UUID, digest string) (sceneeditor.Snapshot, error) {
	s.ownerID = ownerID
	s.projectID = projectID
	s.digest = digest
	return s.snapshot, s.err
}

type jobQueueStub struct {
	input jobs.EnqueueInput
	job   jobs.Job
	err   error
	calls int
}

func (q *jobQueueStub) Enqueue(_ context.Context, input jobs.EnqueueInput) (jobs.Job, error) {
	q.calls++
	q.input = input
	return q.job, q.err
}

func TestEnqueueBindsJobToAuthoritativeImmutableSnapshot(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	jobID := uuid.New()
	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	store := &snapshotStoreStub{snapshot: sceneeditor.Snapshot{
		SchemaVersion: sceneeditor.SnapshotSchemaVersion,
		ProjectID:     projectID,
		Digest:        digest,
	}}
	queue := &jobQueueStub{job: jobs.Job{ID: jobID}}
	service := NewService(store, queue, func() uuid.UUID { return jobID })

	job, err := service.Enqueue(context.Background(), ownerID, projectID, digest)
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if job.ID != jobID {
		t.Fatalf("job ID = %s, want %s", job.ID, jobID)
	}
	if store.ownerID != ownerID || store.projectID != projectID || store.digest != digest {
		t.Fatalf("snapshot lookup = (%s, %s, %s), want authoritative request identity", store.ownerID, store.projectID, store.digest)
	}
	if queue.calls != 1 {
		t.Fatalf("enqueue calls = %d, want 1", queue.calls)
	}
	if queue.input.Kind != JobKind || queue.input.ProjectID == nil || *queue.input.ProjectID != projectID {
		t.Fatalf("unexpected job identity: %#v", queue.input)
	}
	if queue.input.DedupeKey == nil || *queue.input.DedupeKey != "render:"+projectID.String()+":"+digest+":"+LocalProfileID {
		t.Fatalf("dedupe key = %v", queue.input.DedupeKey)
	}
	if queue.input.MaxAttempts != DefaultMaxAttempts {
		t.Fatalf("max attempts = %d, want %d", queue.input.MaxAttempts, DefaultMaxAttempts)
	}
	var payload RenderPayload
	if err := json.Unmarshal(queue.input.Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.SnapshotDigest != digest || payload.SnapshotSchema != sceneeditor.SnapshotSchemaVersion || payload.ProfileID != LocalProfileID || payload.SubtitleMode != SubtitleModeOff {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestEnqueueWebVTTModeIsImmutableAndDedupeDistinct(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	store := &snapshotStoreStub{snapshot: sceneeditor.Snapshot{
		SchemaVersion: sceneeditor.SnapshotSchemaVersion,
		ProjectID:     projectID,
		Digest:        digest,
	}}
	queue := &jobQueueStub{job: jobs.Job{ID: uuid.New()}}
	service := NewService(store, queue, uuid.New)

	_, err := service.Enqueue(context.Background(), ownerID, projectID, digest, SubtitleModeWebVTT)
	if err != nil {
		t.Fatal(err)
	}
	if queue.input.DedupeKey == nil || *queue.input.DedupeKey != "render:"+projectID.String()+":"+digest+":"+LocalProfileID+":webvtt" {
		t.Fatalf("webvtt dedupe key = %v", queue.input.DedupeKey)
	}
	var payload RenderPayload
	if err := json.Unmarshal(queue.input.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.SubtitleMode != SubtitleModeWebVTT {
		t.Fatalf("subtitle mode = %q, want %q", payload.SubtitleMode, SubtitleModeWebVTT)
	}
}

func TestEnqueueRejectsInvalidSubtitleModeBeforeDependencies(t *testing.T) {
	store := &snapshotStoreStub{}
	queue := &jobQueueStub{}
	service := NewService(store, queue, uuid.New)
	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	_, err := service.Enqueue(context.Background(), uuid.New(), uuid.New(), digest, SubtitleMode("burned-in"))
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Enqueue() error = %v, want ErrInvalidRequest", err)
	}
	if store.digest != "" || queue.calls != 0 {
		t.Fatalf("invalid subtitle mode reached dependencies: digest=%q calls=%d", store.digest, queue.calls)
	}
}

func TestEnqueueRejectsSnapshotIdentityMismatchBeforeQueueing(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	store := &snapshotStoreStub{snapshot: sceneeditor.Snapshot{
		SchemaVersion: sceneeditor.SnapshotSchemaVersion,
		ProjectID:     uuid.New(),
		Digest:        digest,
	}}
	queue := &jobQueueStub{}
	service := NewService(store, queue, uuid.New)

	_, err := service.Enqueue(context.Background(), ownerID, projectID, digest)
	if !errors.Is(err, ErrSnapshotMismatch) {
		t.Fatalf("Enqueue() error = %v, want ErrSnapshotMismatch", err)
	}
	if queue.calls != 0 {
		t.Fatalf("enqueue calls = %d, want 0", queue.calls)
	}
}

func TestEnqueueRejectsInvalidDigestWithoutSnapshotLookup(t *testing.T) {
	store := &snapshotStoreStub{}
	queue := &jobQueueStub{}
	service := NewService(store, queue, uuid.New)

	_, err := service.Enqueue(context.Background(), uuid.New(), uuid.New(), "not-a-digest")
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Enqueue() error = %v, want ErrInvalidRequest", err)
	}
	if store.digest != "" || queue.calls != 0 {
		t.Fatalf("invalid request reached dependencies: digest=%q calls=%d", store.digest, queue.calls)
	}
}
