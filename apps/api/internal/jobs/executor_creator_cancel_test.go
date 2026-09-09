package jobs_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
)

func TestExecutorFinalizesCreatorCancelForRunningJob(t *testing.T) {
	repo := newFakeJobRepository()
	registry := jobs.NewRegistry()
	entered := make(chan struct{})
	release := make(chan struct{})
	if err := registry.Register("creator_cancel_kind", jobs.HandlerFunc(func(ctx context.Context, job jobs.Job) (json.RawMessage, error) {
		close(entered)
		select {
		case <-release:
			return json.RawMessage(`{"late":true}`), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})); err != nil {
		t.Fatalf("register: %v", err)
	}

	ownerID := uuid.New()
	projectID := uuid.New()
	job, err := repo.Enqueue(context.Background(), jobs.EnqueueInput{
		OwnerID:     ownerID,
		ProjectID:   &projectID,
		Kind:        "creator_cancel_kind",
		MaxAttempts: 3,
		Payload:     json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	executor := jobs.NewExecutor(repo, registry, jobs.ExecutorConfig{
		LeaseDuration:     200 * time.Millisecond,
		CancellationGrace: 20 * time.Millisecond,
	})

	done := make(chan error, 1)
	go func() {
		_, runErr := executor.RunOnce(context.Background())
		done <- runErr
	}()
	<-entered

	running, err := repo.GetByID(context.Background(), ownerID, job.ID)
	if err != nil {
		t.Fatalf("get running job: %v", err)
	}
	if running.State != jobs.StateRunning || running.LeaseToken == nil {
		t.Fatalf("expected running leased job, got %#v", running)
	}
	if _, err := repo.RequestCancel(context.Background(), ownerID, projectID, job.ID); err != nil {
		t.Fatalf("request cancel: %v", err)
	}

	select {
	case runErr := <-done:
		if runErr != nil {
			t.Fatalf("RunOnce() error = %v", runErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("executor did not finalize creator cancel")
	}

	updated, err := repo.GetByID(context.Background(), ownerID, job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if updated.State != jobs.StateCancelled {
		t.Fatalf("expected cancelled, got %s", updated.State)
	}

	close(release)
	time.Sleep(30 * time.Millisecond)
	updated, err = repo.GetByID(context.Background(), ownerID, job.ID)
	if err != nil {
		t.Fatalf("get job after late completion: %v", err)
	}
	if updated.State != jobs.StateCancelled {
		t.Fatalf("late completion changed terminal state to %s", updated.State)
	}
}

func TestExecutorCreatorCancelWinsRaceOverSuccess(t *testing.T) {
	repo := newFakeJobRepository()
	registry := jobs.NewRegistry()
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	if err := registry.Register("creator_cancel_race", jobs.HandlerFunc(func(ctx context.Context, job jobs.Job) (json.RawMessage, error) {
		once.Do(func() { close(entered) })
		select {
		case <-release:
			return json.RawMessage(`{"won":"success"}`), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})); err != nil {
		t.Fatalf("register: %v", err)
	}

	ownerID := uuid.New()
	projectID := uuid.New()
	job, err := repo.Enqueue(context.Background(), jobs.EnqueueInput{
		OwnerID:     ownerID,
		ProjectID:   &projectID,
		Kind:        "creator_cancel_race",
		MaxAttempts: 3,
		Payload:     json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	executor := jobs.NewExecutor(repo, registry, jobs.ExecutorConfig{
		LeaseDuration:     200 * time.Millisecond,
		CancellationGrace: 20 * time.Millisecond,
	})

	done := make(chan error, 1)
	go func() {
		_, runErr := executor.RunOnce(context.Background())
		done <- runErr
	}()
	<-entered
	if _, err := repo.RequestCancel(context.Background(), ownerID, projectID, job.ID); err != nil {
		t.Fatalf("request cancel: %v", err)
	}

	select {
	case runErr := <-done:
		if runErr != nil {
			t.Fatalf("RunOnce() error = %v", runErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("executor did not finalize creator cancel race")
	}

	close(release)
	time.Sleep(30 * time.Millisecond)

	updated, err := repo.GetByID(context.Background(), ownerID, job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if updated.State != jobs.StateCancelled {
		t.Fatalf("expected cancelled to win race, got %s", updated.State)
	}
	if updated.Result != nil && len(updated.Result) > 0 {
		t.Fatalf("cancelled job retained success result: %s", string(updated.Result))
	}
}
