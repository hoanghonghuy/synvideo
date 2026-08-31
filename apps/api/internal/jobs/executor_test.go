package jobs_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
)

type fakeJobRepository struct {
	mu           sync.Mutex
	jobs         map[uuid.UUID]*jobs.Job
	claimedToken map[uuid.UUID]uuid.UUID
	renewErr     error
}

func newFakeJobRepository() *fakeJobRepository {
	return &fakeJobRepository{
		jobs:         make(map[uuid.UUID]*jobs.Job),
		claimedToken: make(map[uuid.UUID]uuid.UUID),
	}
}

func (f *fakeJobRepository) Enqueue(ctx context.Context, input jobs.EnqueueInput) (jobs.Job, error) {
	if err := input.Validate(); err != nil {
		return jobs.Job{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	id := input.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	now := time.Now().UTC()
	avail := now
	if input.AvailableAt != nil {
		avail = input.AvailableAt.UTC()
	}
	maxAttempts := input.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	job := jobs.Job{
		ID:          id,
		OwnerID:     input.OwnerID,
		ProjectID:   input.ProjectID,
		Kind:        input.Kind,
		DedupeKey:   input.DedupeKey,
		State:       jobs.StateQueued,
		Attempt:     0,
		MaxAttempts: maxAttempts,
		AvailableAt: avail,
		Payload:     input.Payload,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	f.jobs[id] = &job
	return job, nil
}

func (f *fakeJobRepository) GetByID(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) (jobs.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.jobs[id]
	if !ok || j.OwnerID != ownerID {
		return jobs.Job{}, jobs.ErrJobNotFound
	}
	return *j, nil
}

func (f *fakeJobRepository) GetByIDForProject(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, id uuid.UUID) (jobs.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.jobs[id]
	if !ok || j.OwnerID != ownerID || j.ProjectID == nil || *j.ProjectID != projectID {
		return jobs.Job{}, jobs.ErrJobNotFound
	}
	return *j, nil
}

func (f *fakeJobRepository) ClaimNext(ctx context.Context, opts jobs.ClaimOptions) (jobs.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now().UTC()

	kindSet := make(map[string]bool)
	for _, k := range opts.Kinds {
		kindSet[k] = true
	}

	for _, j := range f.jobs {
		if !kindSet[j.Kind] {
			continue
		}
		canClaim := false
		if j.State == jobs.StateQueued && (j.AvailableAt.Before(now) || j.AvailableAt.Equal(now)) {
			canClaim = true
		} else if j.State == jobs.StateRunning && j.LeaseUntil != nil && j.LeaseUntil.Before(now) && j.Attempt < j.MaxAttempts {
			canClaim = true
		}

		if canClaim {
			token := uuid.New()
			j.State = jobs.StateRunning
			j.Attempt++
			j.LeaseToken = &token
			leaseDeadline := now.Add(opts.LeaseDuration)
			j.LeaseUntil = &leaseDeadline
			if j.StartedAt == nil {
				j.StartedAt = &now
			}
			j.UpdatedAt = now
			f.claimedToken[j.ID] = token
			return *j, nil
		}
	}
	return jobs.Job{}, jobs.ErrNoJobAvailable
}

func (f *fakeJobRepository) RenewLease(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, extendDuration time.Duration) (jobs.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.renewErr != nil {
		return jobs.Job{}, f.renewErr
	}
	j, ok := f.jobs[id]
	if !ok {
		return jobs.Job{}, jobs.ErrJobNotFound
	}
	if j.State != jobs.StateRunning || j.LeaseToken == nil || *j.LeaseToken != leaseToken {
		return jobs.Job{}, jobs.ErrStaleLease
	}
	now := time.Now().UTC()
	newDeadline := now.Add(extendDuration)
	j.LeaseUntil = &newDeadline
	j.UpdatedAt = now
	return *j, nil
}

func (f *fakeJobRepository) MarkSuccess(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, result json.RawMessage) (jobs.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.jobs[id]
	if !ok {
		return jobs.Job{}, jobs.ErrJobNotFound
	}
	if j.State != jobs.StateRunning || j.LeaseToken == nil || *j.LeaseToken != leaseToken {
		return jobs.Job{}, jobs.ErrStaleLease
	}
	now := time.Now().UTC()
	j.State = jobs.StateSucceeded
	j.Result = result
	j.LeaseToken = nil
	j.LeaseUntil = nil
	j.FinishedAt = &now
	j.UpdatedAt = now
	return *j, nil
}

func (f *fakeJobRepository) MarkRetryableFailure(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, errorCode string, nextAvailableAt time.Time) (jobs.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.jobs[id]
	if !ok {
		return jobs.Job{}, jobs.ErrJobNotFound
	}
	if j.State != jobs.StateRunning || j.LeaseToken == nil || *j.LeaseToken != leaseToken {
		return jobs.Job{}, jobs.ErrStaleLease
	}
	now := time.Now().UTC()
	j.State = jobs.StateQueued
	j.AvailableAt = nextAvailableAt.UTC()
	j.ErrorCode = &errorCode
	j.LeaseToken = nil
	j.LeaseUntil = nil
	j.UpdatedAt = now
	return *j, nil
}

func (f *fakeJobRepository) MarkTerminalFailure(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, errorCode string) (jobs.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.jobs[id]
	if !ok {
		return jobs.Job{}, jobs.ErrJobNotFound
	}
	if j.State != jobs.StateRunning || j.LeaseToken == nil || *j.LeaseToken != leaseToken {
		return jobs.Job{}, jobs.ErrStaleLease
	}
	now := time.Now().UTC()
	j.State = jobs.StateFailed
	j.ErrorCode = &errorCode
	j.LeaseToken = nil
	j.LeaseUntil = nil
	j.FinishedAt = &now
	j.UpdatedAt = now
	return *j, nil
}

func TestExecutorRunOnceSuccess(t *testing.T) {
	repo := newFakeJobRepository()
	registry := jobs.NewRegistry()

	handled := false
	handler := jobs.HandlerFunc(func(ctx context.Context, job jobs.Job) (json.RawMessage, error) {
		handled = true
		return json.RawMessage(`{"greeting":"hello"}`), nil
	})
	_ = registry.Register("test_kind", handler)

	ownerID := uuid.New()
	job, err := repo.Enqueue(context.Background(), jobs.EnqueueInput{
		OwnerID:     ownerID,
		Kind:        "test_kind",
		MaxAttempts: 3,
		Payload:     json.RawMessage(`{"foo":"bar"}`),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	executor := jobs.NewExecutor(repo, registry, jobs.ExecutorConfig{
		LeaseDuration:  10 * time.Second,
		DefaultBackoff: 5 * time.Second,
	})

	executed, err := executor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if !executed || !handled {
		t.Fatalf("expected job to be executed")
	}

	updated, err := repo.GetByID(context.Background(), ownerID, job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if updated.State != jobs.StateSucceeded {
		t.Fatalf("expected succeeded, got %s", updated.State)
	}
	if string(updated.Result) != `{"greeting":"hello"}` {
		t.Fatalf("unexpected result: %s", string(updated.Result))
	}
}

func TestExecutorContextCancellation(t *testing.T) {
	repo := newFakeJobRepository()
	registry := jobs.NewRegistry()

	entered := make(chan struct{})
	handler := jobs.HandlerFunc(func(ctx context.Context, job jobs.Job) (json.RawMessage, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	_ = registry.Register("cancel_kind", handler)

	ownerID := uuid.New()
	job, err := repo.Enqueue(context.Background(), jobs.EnqueueInput{
		OwnerID:     ownerID,
		Kind:        "cancel_kind",
		MaxAttempts: 3,
		Payload:     json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	executor := jobs.NewExecutor(repo, registry, jobs.ExecutorConfig{
		LeaseDuration:  10 * time.Second,
		DefaultBackoff: 5 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	var execErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, execErr = executor.RunOnce(ctx)
	}()

	<-entered
	cancel()
	wg.Wait()

	if !errors.Is(execErr, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", execErr)
	}

	// State should remain running with lease intact (not corrupted or marked failed prematurely)
	updated, err := repo.GetByID(context.Background(), ownerID, job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if updated.State != jobs.StateRunning {
		t.Fatalf("expected running after cancellation, got %s", updated.State)
	}
}

func TestExecutorRenewsLeaseDuringLongHandler(t *testing.T) {
	repo := newFakeJobRepository()
	registry := jobs.NewRegistry()

	entered := make(chan struct{})
	release := make(chan struct{})
	var handlerCalls atomic.Int32
	handler := jobs.HandlerFunc(func(ctx context.Context, job jobs.Job) (json.RawMessage, error) {
		if handlerCalls.Add(1) > 1 {
			return nil, errors.New("handler called more than once")
		}
		close(entered)
		select {
		case <-release:
			return json.RawMessage(`{"ok":true}`), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	if err := registry.Register("long_kind", handler); err != nil {
		t.Fatalf("register handler: %v", err)
	}

	secondCalled := make(chan struct{}, 1)
	secondRegistry := jobs.NewRegistry()
	if err := secondRegistry.Register("long_kind", jobs.HandlerFunc(func(ctx context.Context, job jobs.Job) (json.RawMessage, error) {
		secondCalled <- struct{}{}
		return json.RawMessage(`{"duplicate":true}`), nil
	})); err != nil {
		t.Fatalf("register second handler: %v", err)
	}

	ownerID := uuid.New()
	job, err := repo.Enqueue(context.Background(), jobs.EnqueueInput{
		OwnerID:     ownerID,
		Kind:        "long_kind",
		MaxAttempts: 3,
		Payload:     json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	firstExecutor := jobs.NewExecutor(repo, registry, jobs.ExecutorConfig{LeaseDuration: 40 * time.Millisecond})
	secondExecutor := jobs.NewExecutor(repo, secondRegistry, jobs.ExecutorConfig{LeaseDuration: 40 * time.Millisecond})
	firstDone := make(chan error, 1)
	go func() {
		_, runErr := firstExecutor.RunOnce(context.Background())
		firstDone <- runErr
	}()

	<-entered
	time.Sleep(120 * time.Millisecond)
	executed, err := secondExecutor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("second executor: %v", err)
	}
	if executed {
		t.Fatalf("second executor reclaimed a healthy long-running job")
	}
	select {
	case <-secondCalled:
		t.Fatalf("second handler ran while first handler was healthy")
	default:
	}

	close(release)
	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first executor: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first handler did not finish")
	}

	updated, err := repo.GetByID(context.Background(), ownerID, job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if updated.State != jobs.StateSucceeded || handlerCalls.Load() != 1 {
		t.Fatalf("expected one successful handler call, state=%s calls=%d", updated.State, handlerCalls.Load())
	}
}

func TestExecutorDoesNotCommitAfterLeaseRenewalLoss(t *testing.T) {
	repo := newFakeJobRepository()
	repo.renewErr = jobs.ErrStaleLease
	registry := jobs.NewRegistry()

	entered := make(chan struct{})
	if err := registry.Register("lost_lease_kind", jobs.HandlerFunc(func(ctx context.Context, job jobs.Job) (json.RawMessage, error) {
		close(entered)
		<-ctx.Done()
		return json.RawMessage(`{"should_not":"commit"}`), nil
	})); err != nil {
		t.Fatalf("register handler: %v", err)
	}

	ownerID := uuid.New()
	job, err := repo.Enqueue(context.Background(), jobs.EnqueueInput{
		OwnerID:     ownerID,
		Kind:        "lost_lease_kind",
		MaxAttempts: 3,
		Payload:     json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	executor := jobs.NewExecutor(repo, registry, jobs.ExecutorConfig{LeaseDuration: 40 * time.Millisecond})
	done := make(chan error, 1)
	go func() {
		_, runErr := executor.RunOnce(context.Background())
		done <- runErr
	}()
	<-entered

	select {
	case runErr := <-done:
		if !errors.Is(runErr, jobs.ErrStaleLease) {
			t.Fatalf("expected ErrStaleLease, got %v", runErr)
		}
	case <-time.After(time.Second):
		t.Fatal("executor did not stop after lease renewal loss")
	}

	updated, err := repo.GetByID(context.Background(), ownerID, job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if updated.State != jobs.StateRunning {
		t.Fatalf("expected lease-lost job to remain running for reclaim, got %s", updated.State)
	}
}
