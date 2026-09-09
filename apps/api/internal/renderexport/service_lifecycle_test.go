package renderexport

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

type lifecycleJobReader struct {
	jobs map[uuid.UUID]jobs.Job
}

func newLifecycleJobReader() *lifecycleJobReader {
	return &lifecycleJobReader{jobs: make(map[uuid.UUID]jobs.Job)}
}

func (r *lifecycleJobReader) seed(job jobs.Job) {
	r.jobs[job.ID] = job
}

func (r *lifecycleJobReader) GetByIDForProject(_ context.Context, ownerID uuid.UUID, projectID uuid.UUID, id uuid.UUID) (jobs.Job, error) {
	job, ok := r.jobs[id]
	if !ok || job.OwnerID != ownerID || job.ProjectID == nil || *job.ProjectID != projectID {
		return jobs.Job{}, jobs.ErrJobNotFound
	}
	return job, nil
}

func (r *lifecycleJobReader) GetByDedupeKey(_ context.Context, ownerID uuid.UUID, kind string, dedupeKey string) (jobs.Job, error) {
	for _, job := range r.jobs {
		if job.OwnerID == ownerID && job.Kind == kind && job.DedupeKey != nil && *job.DedupeKey == dedupeKey {
			return job, nil
		}
	}
	return jobs.Job{}, jobs.ErrJobNotFound
}

func (r *lifecycleJobReader) ListByProjectKind(_ context.Context, options jobs.ListByProjectKindOptions) ([]jobs.Job, *jobs.ListCursor, error) {
	items := make([]jobs.Job, 0)
	for _, job := range r.jobs {
		if job.OwnerID != options.OwnerID || job.ProjectID == nil || *job.ProjectID != options.ProjectID || job.Kind != options.Kind {
			continue
		}
		if options.Cursor != nil {
			if job.CreatedAt.After(options.Cursor.CreatedAt) {
				continue
			}
			if job.CreatedAt.Equal(options.Cursor.CreatedAt) && job.ID.String() >= options.Cursor.ID.String() {
				continue
			}
		}
		items = append(items, job)
	}
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].CreatedAt.After(items[i].CreatedAt) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}
	var next *jobs.ListCursor
	if len(items) > limit {
		last := items[limit-1]
		next = &jobs.ListCursor{CreatedAt: last.CreatedAt, ID: last.ID}
		items = items[:limit]
	}
	return items, next, nil
}

func (r *lifecycleJobReader) RequestCancel(_ context.Context, ownerID uuid.UUID, projectID uuid.UUID, id uuid.UUID) (jobs.Job, error) {
	job, err := r.GetByIDForProject(context.Background(), ownerID, projectID, id)
	if err != nil {
		return jobs.Job{}, err
	}
	now := time.Now().UTC()
	switch job.State {
	case jobs.StateCancelled:
		return job, nil
	case jobs.StateSucceeded, jobs.StateFailed:
		return jobs.Job{}, jobs.ErrJobTerminal
	case jobs.StateQueued:
		job.State = jobs.StateCancelled
		code := jobs.ErrorCodeCancelled
		job.ErrorCode = &code
		job.FinishedAt = &now
	case jobs.StateRunning:
		if job.CancelRequestedAt == nil {
			job.CancelRequestedAt = &now
		}
	default:
		return jobs.Job{}, jobs.ErrInvalidInput
	}
	job.UpdatedAt = now
	r.jobs[id] = job
	return job, nil
}

type lifecycleQueue struct {
	reader *lifecycleJobReader
}

func (q *lifecycleQueue) Enqueue(_ context.Context, input jobs.EnqueueInput) (jobs.Job, error) {
	now := time.Now().UTC()
	job := jobs.Job{
		ID:          input.ID,
		OwnerID:     input.OwnerID,
		ProjectID:   input.ProjectID,
		Kind:        input.Kind,
		DedupeKey:   input.DedupeKey,
		State:       jobs.StateQueued,
		MaxAttempts: input.MaxAttempts,
		Payload:     input.Payload,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	for _, existing := range q.reader.jobs {
		if existing.OwnerID == input.OwnerID && existing.Kind == input.Kind && existing.DedupeKey != nil && input.DedupeKey != nil && *existing.DedupeKey == *input.DedupeKey {
			return jobs.Job{}, jobs.ErrDuplicateJob
		}
	}
	q.reader.seed(job)
	return job, nil
}

func terminalRenderJob(ownerID, projectID uuid.UUID, state jobs.State) jobs.Job {
	now := time.Now().UTC()
	payload, _ := json.Marshal(RenderPayload{
		SnapshotDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		SnapshotSchema: sceneeditor.SnapshotSchemaVersion,
		ProfileID:      LocalProfileID,
	})
	code := "ERR_TEST"
	return jobs.Job{
		ID:          uuid.New(),
		OwnerID:     ownerID,
		ProjectID:   &projectID,
		Kind:        JobKind,
		State:       state,
		MaxAttempts: DefaultMaxAttempts,
		Payload:     payload,
		ErrorCode:   &code,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestCancelQueuedRenderIsIdempotentAndTerminal(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	reader := newLifecycleJobReader()
	job := terminalRenderJob(ownerID, projectID, jobs.StateQueued)
	job.ErrorCode = nil
	reader.seed(job)
	service := NewServiceWithRuntime(nil, &lifecycleQueue{reader: reader}, reader, nil, uuid.New)

	first, err := service.Cancel(context.Background(), ownerID, projectID, job.ID)
	if err != nil || first.State != jobs.StateCancelled {
		t.Fatalf("first cancel = %#v err=%v", first, err)
	}
	second, err := service.Cancel(context.Background(), ownerID, projectID, job.ID)
	if err != nil || second.State != jobs.StateCancelled {
		t.Fatalf("second cancel = %#v err=%v", second, err)
	}
}

func TestCancelSucceededRenderReturnsConflict(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	reader := newLifecycleJobReader()
	job := terminalRenderJob(ownerID, projectID, jobs.StateSucceeded)
	job.ErrorCode = nil
	reader.seed(job)
	service := NewServiceWithRuntime(nil, &lifecycleQueue{reader: reader}, reader, nil, uuid.New)

	_, err := service.Cancel(context.Background(), ownerID, projectID, job.ID)
	if !errors.Is(err, ErrRenderNotCancellable) {
		t.Fatalf("Cancel() error = %v, want ErrRenderNotCancellable", err)
	}
}

func TestRetryCreatesLinkedJobWithoutMutatingSource(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	reader := newLifecycleJobReader()
	source := terminalRenderJob(ownerID, projectID, jobs.StateFailed)
	reader.seed(source)
	service := NewServiceWithRuntime(nil, &lifecycleQueue{reader: reader}, reader, nil, uuid.New)
	requestID := uuid.New()

	retryView, err := service.Retry(context.Background(), ownerID, projectID, source.ID, requestID)
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}
	if retryView.RetryOfRenderJobID == nil || *retryView.RetryOfRenderJobID != source.ID {
		t.Fatalf("retry lineage = %#v", retryView.RetryOfRenderJobID)
	}
	sourceAfter, err := reader.GetByIDForProject(context.Background(), ownerID, projectID, source.ID)
	if err != nil || sourceAfter.State != jobs.StateFailed {
		t.Fatalf("source mutated: %#v err=%v", sourceAfter, err)
	}
	retryAgain, err := service.Retry(context.Background(), ownerID, projectID, source.ID, requestID)
	if err != nil || retryAgain.ID != retryView.ID {
		t.Fatalf("retry idempotency failed: %#v err=%v", retryAgain, err)
	}
}

func TestRetryRejectsSucceededSource(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	reader := newLifecycleJobReader()
	source := terminalRenderJob(ownerID, projectID, jobs.StateSucceeded)
	source.ErrorCode = nil
	reader.seed(source)
	service := NewServiceWithRuntime(nil, &lifecycleQueue{reader: reader}, reader, nil, uuid.New)

	_, err := service.Retry(context.Background(), ownerID, projectID, source.ID, uuid.New())
	if !errors.Is(err, ErrRenderNotRetryable) {
		t.Fatalf("Retry() error = %v, want ErrRenderNotRetryable", err)
	}
}

func TestListHistoryIsOwnerProjectScoped(t *testing.T) {
	ownerID := uuid.New()
	otherOwner := uuid.New()
	projectID := uuid.New()
	reader := newLifecycleJobReader()
	first := terminalRenderJob(ownerID, projectID, jobs.StateFailed)
	second := terminalRenderJob(ownerID, projectID, jobs.StateCancelled)
	other := terminalRenderJob(otherOwner, projectID, jobs.StateFailed)
	reader.seed(first)
	reader.seed(second)
	reader.seed(other)
	service := NewServiceWithRuntime(nil, &lifecycleQueue{reader: reader}, reader, nil, uuid.New)

	result, err := service.ListHistory(context.Background(), ownerID, projectID, 10, "")
	if err != nil {
		t.Fatalf("ListHistory() error = %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("history items = %d, want 2", len(result.Items))
	}
}
