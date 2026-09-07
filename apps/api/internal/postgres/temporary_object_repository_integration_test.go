package postgres

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarrationjob"
)

func TestTemporaryObjectRepositoryIntegration(t *testing.T) {
	pool := integrationPool(t)
	projectRepo := NewProjectRepository(pool)
	jobRepo := NewJobRepository(pool)
	temporaryRepo := NewTemporaryObjectRepository(pool)
	ownerID := uuid.New()

	projectItem, err := projectRepo.Create(context.Background(), ownerID, validIntegrationCreateInput("Temporary Object Lifecycle"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	terminalJob, err := jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
		OwnerID: ownerID, ProjectID: &projectItem.ID, Kind: "temp_object_terminal_test", MaxAttempts: 1, Payload: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("enqueue terminal job: %v", err)
	}
	claimedJob, err := jobRepo.ClaimNext(context.Background(), jobs.ClaimOptions{Kinds: []string{"temp_object_terminal_test"}, LeaseDuration: time.Minute})
	if err != nil {
		t.Fatalf("claim terminal job: %v", err)
	}
	if _, err := jobRepo.MarkTerminalFailure(context.Background(), terminalJob.ID, *claimedJob.LeaseToken, "ERR_TEST_TERMINAL"); err != nil {
		t.Fatalf("mark terminal job: %v", err)
	}

	objectID := uuid.New()
	objectKey := "projects/" + projectItem.ID.String() + "/internal_chunks/" + terminalJob.ID.String() + "/0"
	if err := temporaryRepo.Track(context.Background(), scenenarrationjob.TemporaryObject{
		ID: objectID, ProjectID: projectItem.ID, JobID: terminalJob.ID, ObjectKey: objectKey,
	}); err != nil {
		t.Fatalf("track terminal object: %v", err)
	}

	var wg sync.WaitGroup
	claims := make(chan []scenenarrationjob.TemporaryObject, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, claimErr := temporaryRepo.ClaimCleanup(context.Background(), 1, time.Minute)
			if claimErr != nil {
				errs <- claimErr
				return
			}
			claims <- items
		}()
	}
	wg.Wait()
	close(claims)
	close(errs)
	for claimErr := range errs {
		t.Fatalf("concurrent cleanup claim: %v", claimErr)
	}

	var matching []scenenarrationjob.TemporaryObject
	for batch := range claims {
		for _, object := range batch {
			if object.ID == objectID {
				matching = append(matching, object)
			}
		}
	}
	if len(matching) != 1 {
		t.Fatalf("expected exactly one worker to claim terminal object, got %d", len(matching))
	}
	if err := temporaryRepo.CompleteCleanup(context.Background(), matching[0].ID, matching[0].ClaimToken); err != nil {
		t.Fatalf("complete cleanup: %v", err)
	}

	activeJob, err := jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
		OwnerID: ownerID, ProjectID: &projectItem.ID, Kind: "temp_object_active_test", MaxAttempts: 3, Payload: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("enqueue active job: %v", err)
	}
	activeObjectID := uuid.New()
	activeKey := "projects/" + projectItem.ID.String() + "/internal_chunks/" + activeJob.ID.String() + "/0"
	if err := temporaryRepo.Track(context.Background(), scenenarrationjob.TemporaryObject{
		ID: activeObjectID, ProjectID: projectItem.ID, JobID: activeJob.ID, ObjectKey: activeKey,
	}); err != nil {
		t.Fatalf("track active object: %v", err)
	}

	items, err := temporaryRepo.ClaimCleanup(context.Background(), scenenarrationjob.DefaultCleanupBatchSize, time.Minute)
	if err != nil {
		t.Fatalf("claim while active object exists: %v", err)
	}
	for _, object := range items {
		if object.ID == activeObjectID {
			t.Fatal("queued/retryable job checkpoint must not become cleanup-eligible")
		}
	}
}
