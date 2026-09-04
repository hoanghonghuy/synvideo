package jobs_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
)

type finalizerCountingRepository struct {
	*fakeJobRepository
	finalizeCalls atomic.Int32
}

func (f *finalizerCountingRepository) MarkSuccess(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, result json.RawMessage) (jobs.Job, error) {
	f.finalizeCalls.Add(1)
	return f.fakeJobRepository.MarkSuccess(ctx, id, leaseToken, result)
}

func (f *finalizerCountingRepository) MarkRetryableFailure(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, errorCode string, nextAvailableAt time.Time) (jobs.Job, error) {
	f.finalizeCalls.Add(1)
	return f.fakeJobRepository.MarkRetryableFailure(ctx, id, leaseToken, errorCode, nextAvailableAt)
}

func (f *finalizerCountingRepository) MarkTerminalFailure(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, errorCode string) (jobs.Job, error) {
	f.finalizeCalls.Add(1)
	return f.fakeJobRepository.MarkTerminalFailure(ctx, id, leaseToken, errorCode)
}

func TestExecutorBoundsParentCancellationForNonCooperativeHandler(t *testing.T) {
	repo := &finalizerCountingRepository{fakeJobRepository: newFakeJobRepository()}
	registry := jobs.NewRegistry()
	entered := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	if err := registry.Register("non_cooperative_cancel", jobs.HandlerFunc(func(ctx context.Context, job jobs.Job) (json.RawMessage, error) {
		close(entered)
		<-release
		close(finished)
		return json.RawMessage(`{"late":true}`), nil
	})); err != nil {
		t.Fatalf("register: %v", err)
	}

	ownerID := uuid.New()
	job, err := repo.Enqueue(context.Background(), jobs.EnqueueInput{OwnerID: ownerID, Kind: "non_cooperative_cancel", MaxAttempts: 3, Payload: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	executor := jobs.NewExecutor(repo, registry, jobs.ExecutorConfig{
		LeaseDuration:     200 * time.Millisecond,
		CancellationGrace: 20 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, runErr := executor.RunOnce(ctx)
		done <- runErr
	}()
	<-entered
	cancel()

	select {
	case runErr := <-done:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", runErr)
		}
	case <-time.After(150 * time.Millisecond):
		t.Fatal("RunOnce exceeded bounded cancellation grace")
	}
	if got := repo.finalizeCalls.Load(); got != 0 {
		t.Fatalf("expected no finalizer before late completion, got %d", got)
	}

	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("late handler did not finish")
	}
	time.Sleep(20 * time.Millisecond)
	if got := repo.finalizeCalls.Load(); got != 0 {
		t.Fatalf("late completion finalized canceled job: %d calls", got)
	}
	updated, err := repo.GetByID(context.Background(), ownerID, job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if updated.State != jobs.StateRunning {
		t.Fatalf("expected running lease-owned job after cancellation, got %s", updated.State)
	}
}

func TestExecutorBoundsLeaseLossForNonCooperativeHandler(t *testing.T) {
	base := newFakeJobRepository()
	base.renewErr = jobs.ErrStaleLease
	repo := &finalizerCountingRepository{fakeJobRepository: base}
	registry := jobs.NewRegistry()
	entered := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	if err := registry.Register("non_cooperative_lease_loss", jobs.HandlerFunc(func(ctx context.Context, job jobs.Job) (json.RawMessage, error) {
		close(entered)
		<-release
		close(finished)
		return nil, jobs.NewTerminalJobError("ERR_LATE", errors.New("late terminal failure"))
	})); err != nil {
		t.Fatalf("register: %v", err)
	}

	ownerID := uuid.New()
	job, err := repo.Enqueue(context.Background(), jobs.EnqueueInput{OwnerID: ownerID, Kind: "non_cooperative_lease_loss", MaxAttempts: 3, Payload: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	executor := jobs.NewExecutor(repo, registry, jobs.ExecutorConfig{
		LeaseDuration:     60 * time.Millisecond,
		CancellationGrace: 10 * time.Millisecond,
	})

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
	case <-time.After(150 * time.Millisecond):
		t.Fatal("RunOnce exceeded bounded lease-loss grace")
	}
	if got := repo.finalizeCalls.Load(); got != 0 {
		t.Fatalf("expected no finalizer after lease loss, got %d", got)
	}

	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("late handler did not finish")
	}
	time.Sleep(20 * time.Millisecond)
	if got := repo.finalizeCalls.Load(); got != 0 {
		t.Fatalf("late completion finalized stale-lease job: %d calls", got)
	}
	updated, err := repo.GetByID(context.Background(), ownerID, job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if updated.State != jobs.StateRunning {
		t.Fatalf("expected stale-lease job to remain running for reclaim, got %s", updated.State)
	}
}
