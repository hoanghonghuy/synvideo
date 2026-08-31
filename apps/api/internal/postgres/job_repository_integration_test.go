package postgres

import (
	"bytes"
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

func TestJobRepositoryIntegration(t *testing.T) {
	pool := integrationPool(t)
	projectRepo := NewProjectRepository(pool)
	jobRepo := NewJobRepository(pool)

	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	otherOwnerID := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	projectItem, err := projectRepo.Create(context.Background(), ownerID, validIntegrationCreateInput("Jobs Test Project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// 1. Enqueue persists queued, attempt 0
	t.Run("1. Enqueue persists queued attempt 0", func(t *testing.T) {
		dedupe := "dedupe-enqueue-1"
		job, err := jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
			OwnerID:     ownerID,
			ProjectID:   &projectItem.ID,
			Kind:        "test_queue_kind",
			DedupeKey:   &dedupe,
			MaxAttempts: 3,
			Payload:     json.RawMessage(`{"action":"process"}`),
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		if job.State != jobs.StateQueued || job.Attempt != 0 || job.MaxAttempts != 3 {
			t.Fatalf("unexpected state/attempt: state=%s, attempt=%d, max_attempts=%d", job.State, job.Attempt, job.MaxAttempts)
		}
		if job.LeaseToken != nil || job.LeaseUntil != nil {
			t.Fatalf("queued job should not have lease token or deadline")
		}

		fetched, err := jobRepo.GetByID(context.Background(), ownerID, job.ID)
		if err != nil {
			t.Fatalf("get by id: %v", err)
		}
		if fetched.ID != job.ID || fetched.OwnerID != ownerID || *fetched.ProjectID != projectItem.ID {
			t.Fatalf("fetched job mismatch: %#v", fetched)
		}
	})

	t.Run("payload and result must be json objects", func(t *testing.T) {
		_, err := jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
			OwnerID:     ownerID,
			Kind:        "invalid_payload_kind",
			MaxAttempts: 1,
			Payload:     json.RawMessage(`[]`),
		})
		if !errors.Is(err, jobs.ErrInvalidInput) {
			t.Fatalf("expected invalid payload error, got %v", err)
		}

		job, err := jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
			OwnerID:     ownerID,
			Kind:        "invalid_result_kind",
			MaxAttempts: 1,
			Payload:     json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("enqueue valid job: %v", err)
		}
		claimed, err := jobRepo.ClaimNext(context.Background(), jobs.ClaimOptions{
			Kinds:         []string{"invalid_result_kind"},
			LeaseDuration: 10 * time.Second,
		})
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if _, err := jobRepo.MarkSuccess(context.Background(), job.ID, *claimed.LeaseToken, json.RawMessage(`null`)); !errors.Is(err, jobs.ErrInvalidInput) {
			t.Fatalf("expected invalid result error, got %v", err)
		}
		if _, err := jobRepo.MarkTerminalFailure(context.Background(), job.ID, *claimed.LeaseToken, "ERR_TEST_CLEANUP"); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	})

	// 2 & 3. Concurrent claimers yield exactly one owner for an attempt + atomic lease/attempt increment
	t.Run("2-3. Concurrent claimers and atomic lease", func(t *testing.T) {
		dedupe := "dedupe-claim-concurrent"
		job, err := jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
			OwnerID:     ownerID,
			ProjectID:   &projectItem.ID,
			Kind:        "concurrent_kind",
			DedupeKey:   &dedupe,
			MaxAttempts: 3,
			Payload:     json.RawMessage(`{"num":1}`),
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}

		var claimedCount int32
		var claimedJob jobs.Job
		var mu sync.Mutex
		var wg sync.WaitGroup

		concurrentWorkers := 5
		wg.Add(concurrentWorkers)

		for i := 0; i < concurrentWorkers; i++ {
			go func() {
				defer wg.Done()
				j, err := jobRepo.ClaimNext(context.Background(), jobs.ClaimOptions{
					Kinds:         []string{"concurrent_kind"},
					LeaseDuration: 10 * time.Second,
				})
				if err == nil {
					atomic.AddInt32(&claimedCount, 1)
					mu.Lock()
					claimedJob = j
					mu.Unlock()
				} else if !errors.Is(err, jobs.ErrNoJobAvailable) {
					t.Errorf("unexpected claim error: %v", err)
				}
			}()
		}
		wg.Wait()

		if claimedCount != 1 {
			t.Fatalf("expected exactly 1 worker to claim, got %d", claimedCount)
		}
		if claimedJob.ID != job.ID {
			t.Fatalf("claimed job ID mismatch: %s vs %s", claimedJob.ID, job.ID)
		}
		if claimedJob.State != jobs.StateRunning || claimedJob.Attempt != 1 {
			t.Fatalf("expected running state and attempt 1, got state=%s attempt=%d", claimedJob.State, claimedJob.Attempt)
		}
		if claimedJob.LeaseToken == nil || claimedJob.LeaseUntil == nil {
			t.Fatalf("expected non-nil lease token and deadline")
		}
	})

	// 4. Success is durable and terminal
	t.Run("4. Success is durable and terminal", func(t *testing.T) {
		dedupe := "dedupe-success"
		job, err := jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
			OwnerID:     ownerID,
			Kind:        "success_kind",
			DedupeKey:   &dedupe,
			MaxAttempts: 3,
			Payload:     json.RawMessage(`{"task":"render"}`),
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}

		claimed, err := jobRepo.ClaimNext(context.Background(), jobs.ClaimOptions{
			Kinds:         []string{"success_kind"},
			LeaseDuration: 10 * time.Second,
		})
		if err != nil {
			t.Fatalf("claim: %v", err)
		}

		resultJSON := json.RawMessage(`{"output_url":"https://example.com/video.mp4"}`)
		succeeded, err := jobRepo.MarkSuccess(context.Background(), job.ID, *claimed.LeaseToken, resultJSON)
		if err != nil {
			t.Fatalf("mark success: %v", err)
		}
		if succeeded.State != jobs.StateSucceeded || succeeded.FinishedAt == nil {
			t.Fatalf("expected succeeded state and finished_at set: %#v", succeeded)
		}

		var expectedMap, actualMap map[string]any
		_ = json.Unmarshal(resultJSON, &expectedMap)
		_ = json.Unmarshal(succeeded.Result, &actualMap)
		if expectedMap["output_url"] != actualMap["output_url"] {
			t.Fatalf("unexpected result payload: %s", string(succeeded.Result))
		}

		// Subsequent claim cannot pick it up
		_, err = jobRepo.ClaimNext(context.Background(), jobs.ClaimOptions{
			Kinds:         []string{"success_kind"},
			LeaseDuration: 10 * time.Second,
		})
		if !errors.Is(err, jobs.ErrNoJobAvailable) {
			t.Fatalf("expected ErrNoJobAvailable for succeeded job, got %v", err)
		}

		// Mark again with old lease token should fail with ErrStaleLease
		_, err = jobRepo.MarkSuccess(context.Background(), job.ID, *claimed.LeaseToken, resultJSON)
		if !errors.Is(err, jobs.ErrStaleLease) {
			t.Fatalf("expected ErrStaleLease on already finished job, got %v", err)
		}
	})

	// 5. Retryable failure requeues only when attempts remain
	t.Run("5. Retryable failure requeues", func(t *testing.T) {
		dedupe := "dedupe-retry"
		job, err := jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
			OwnerID:     ownerID,
			Kind:        "retry_kind",
			DedupeKey:   &dedupe,
			MaxAttempts: 3,
			Payload:     json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}

		claimed, err := jobRepo.ClaimNext(context.Background(), jobs.ClaimOptions{
			Kinds:         []string{"retry_kind"},
			LeaseDuration: 10 * time.Second,
		})
		if err != nil {
			t.Fatalf("claim: %v", err)
		}

		nextAvail := time.Now().UTC().Add(100 * time.Millisecond)
		requeued, err := jobRepo.MarkRetryableFailure(context.Background(), job.ID, *claimed.LeaseToken, "ERR_TRANSIENT", nextAvail)
		if err != nil {
			t.Fatalf("mark retryable: %v", err)
		}
		if requeued.State != jobs.StateQueued || requeued.Attempt != 1 {
			t.Fatalf("expected queued with attempt 1, got state=%s attempt=%d", requeued.State, requeued.Attempt)
		}
		if requeued.ErrorCode == nil || *requeued.ErrorCode != "ERR_TRANSIENT" {
			t.Fatalf("expected error_code ERR_TRANSIENT, got %v", requeued.ErrorCode)
		}

		// Before nextAvail, claim should return ErrNoJobAvailable
		_, err = jobRepo.ClaimNext(context.Background(), jobs.ClaimOptions{
			Kinds:         []string{"retry_kind"},
			LeaseDuration: 10 * time.Second,
		})
		if !errors.Is(err, jobs.ErrNoJobAvailable) {
			t.Fatalf("expected ErrNoJobAvailable before available_at, got %v", err)
		}

		// Wait until available_at passes
		time.Sleep(150 * time.Millisecond)

		claimed2, err := jobRepo.ClaimNext(context.Background(), jobs.ClaimOptions{
			Kinds:         []string{"retry_kind"},
			LeaseDuration: 10 * time.Second,
		})
		if err != nil {
			t.Fatalf("claim attempt 2: %v", err)
		}
		if claimed2.Attempt != 2 {
			t.Fatalf("expected attempt 2 on second claim, got %d", claimed2.Attempt)
		}
	})

	// 6. Terminal failure
	t.Run("6. Terminal failure", func(t *testing.T) {
		dedupe := "dedupe-terminal"
		job, err := jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
			OwnerID:     ownerID,
			Kind:        "terminal_kind",
			DedupeKey:   &dedupe,
			MaxAttempts: 1,
			Payload:     json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}

		claimed, err := jobRepo.ClaimNext(context.Background(), jobs.ClaimOptions{
			Kinds:         []string{"terminal_kind"},
			LeaseDuration: 10 * time.Second,
		})
		if err != nil {
			t.Fatalf("claim: %v", err)
		}

		failed, err := jobRepo.MarkTerminalFailure(context.Background(), job.ID, *claimed.LeaseToken, "ERR_INVALID_PAYLOAD")
		if err != nil {
			t.Fatalf("mark terminal failure: %v", err)
		}
		if failed.State != jobs.StateFailed || failed.FinishedAt == nil {
			t.Fatalf("expected failed state and finished_at set: %#v", failed)
		}
		if failed.ErrorCode == nil || *failed.ErrorCode != "ERR_INVALID_PAYLOAD" {
			t.Fatalf("expected ERR_INVALID_PAYLOAD error code, got %v", failed.ErrorCode)
		}

		// Cannot be claimed
		_, err = jobRepo.ClaimNext(context.Background(), jobs.ClaimOptions{
			Kinds:         []string{"terminal_kind"},
			LeaseDuration: 10 * time.Second,
		})
		if !errors.Is(err, jobs.ErrNoJobAvailable) {
			t.Fatalf("expected ErrNoJobAvailable for terminal failed job, got %v", err)
		}
	})

	t.Run("6b. Retry cannot exceed max attempts", func(t *testing.T) {
		job, err := jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
			OwnerID:     ownerID,
			Kind:        "retry_exhausted_kind",
			MaxAttempts: 1,
			Payload:     json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		claimed, err := jobRepo.ClaimNext(context.Background(), jobs.ClaimOptions{
			Kinds:         []string{"retry_exhausted_kind"},
			LeaseDuration: 10 * time.Second,
		})
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if _, err := jobRepo.MarkRetryableFailure(context.Background(), job.ID, *claimed.LeaseToken, "ERR_RETRY", time.Now().UTC()); err == nil {
			t.Fatal("expected retry at max attempts to be rejected")
		}
		current, err := jobRepo.GetByID(context.Background(), ownerID, job.ID)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		if current.State != jobs.StateRunning || current.Attempt != current.MaxAttempts {
			t.Fatalf("expected exhausted job to remain running at max attempt, got state=%s attempt=%d", current.State, current.Attempt)
		}
		if _, err := jobRepo.MarkTerminalFailure(context.Background(), job.ID, *claimed.LeaseToken, "ERR_TEST_CLEANUP"); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	})

	// 7. Expired lease is reclaimable after simulated worker crash
	t.Run("7. Expired lease is reclaimable", func(t *testing.T) {
		dedupe := "dedupe-expire-reclaim"
		job, err := jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
			OwnerID:     ownerID,
			Kind:        "crash_kind",
			DedupeKey:   &dedupe,
			MaxAttempts: 3,
			Payload:     json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}

		// Claim with short lease (50ms)
		claimed1, err := jobRepo.ClaimNext(context.Background(), jobs.ClaimOptions{
			Kinds:         []string{"crash_kind"},
			LeaseDuration: 50 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("claim 1: %v", err)
		}
		if claimed1.Attempt != 1 {
			t.Fatalf("expected attempt 1, got %d", claimed1.Attempt)
		}

		// Worker 1 crashes / abandons the job without calling MarkSuccess or MarkFailure.
		// Wait for lease to expire
		time.Sleep(100 * time.Millisecond)

		// Worker 2 claims next available job -> should reclaim the crashed job
		claimed2, err := jobRepo.ClaimNext(context.Background(), jobs.ClaimOptions{
			Kinds:         []string{"crash_kind"},
			LeaseDuration: 10 * time.Second,
		})
		if err != nil {
			t.Fatalf("claim 2 (reclaim): %v", err)
		}
		if claimed2.ID != job.ID {
			t.Fatalf("reclaimed job ID mismatch: %s vs %s", claimed2.ID, job.ID)
		}
		if claimed2.Attempt != 2 {
			t.Fatalf("expected attempt 2 after reclaim, got %d", claimed2.Attempt)
		}
		if *claimed2.LeaseToken == *claimed1.LeaseToken {
			t.Fatalf("expected new lease token for second attempt, got identical token")
		}

		// 8. Stale lease token cannot complete after reclaim
		_, err = jobRepo.MarkSuccess(context.Background(), job.ID, *claimed1.LeaseToken, json.RawMessage(`{"bad":true}`))
		if !errors.Is(err, jobs.ErrStaleLease) {
			t.Fatalf("expected ErrStaleLease when crashed worker tries to complete with old token, got %v", err)
		}

		// Worker 2 completes successfully with its valid lease token
		_, err = jobRepo.MarkSuccess(context.Background(), job.ID, *claimed2.LeaseToken, json.RawMessage(`{"good":true}`))
		if err != nil {
			t.Fatalf("worker 2 mark success: %v", err)
		}
	})

	t.Run("7b. Expired final attempt becomes terminal", func(t *testing.T) {
		kind := "final_expired_kind_" + uuid.NewString()
		job, err := jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
			OwnerID:     ownerID,
			Kind:        kind,
			MaxAttempts: 1,
			Payload:     json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		if _, err := jobRepo.ClaimNext(context.Background(), jobs.ClaimOptions{
			Kinds:         []string{kind},
			LeaseDuration: 50 * time.Millisecond,
		}); err != nil {
			t.Fatalf("claim: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
		if _, err := jobRepo.ClaimNext(context.Background(), jobs.ClaimOptions{
			Kinds:         []string{kind},
			LeaseDuration: 10 * time.Second,
		}); !errors.Is(err, jobs.ErrNoJobAvailable) {
			t.Fatalf("expected no claim after final lease expiry, got %v", err)
		}
		current, err := jobRepo.GetByID(context.Background(), ownerID, job.ID)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		if current.State != jobs.StateFailed || current.FinishedAt == nil {
			t.Fatalf("expected expired final attempt to be terminal failed, got %#v", current)
		}
		if current.ErrorCode == nil || *current.ErrorCode != "ERR_MAX_ATTEMPTS_EXCEEDED" {
			t.Fatalf("expected max attempts error code, got %v", current.ErrorCode)
		}
	})

	// 9. Owner / Project get is non-disclosing
	t.Run("9. Owner and Project non-disclosing isolation", func(t *testing.T) {
		job, err := jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
			OwnerID:   ownerID,
			ProjectID: &projectItem.ID,
			Kind:      "isolation_kind",
			Payload:   json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}

		// Non-matching owner should get ErrJobNotFound
		_, err = jobRepo.GetByID(context.Background(), otherOwnerID, job.ID)
		if !errors.Is(err, jobs.ErrJobNotFound) {
			t.Fatalf("expected ErrJobNotFound for wrong owner, got %v", err)
		}

		// Non-matching project should get ErrJobNotFound
		otherProjectID := uuid.New()
		_, err = jobRepo.GetByIDForProject(context.Background(), ownerID, otherProjectID, job.ID)
		if !errors.Is(err, jobs.ErrJobNotFound) {
			t.Fatalf("expected ErrJobNotFound for wrong project, got %v", err)
		}

		// Correct owner and project gets the job
		got, err := jobRepo.GetByIDForProject(context.Background(), ownerID, projectItem.ID, job.ID)
		if err != nil {
			t.Fatalf("get by project: %v", err)
		}
		if got.ID != job.ID {
			t.Fatalf("job id mismatch: %s vs %s", got.ID, job.ID)
		}
	})

	// 10. Dedupe key prevents duplicate accepted work where supplied
	t.Run("10. Dedupe key uniqueness", func(t *testing.T) {
		dedupeKey := "dedupe-unique-123"
		_, err := jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
			OwnerID:   ownerID,
			Kind:      "dedupe_kind",
			DedupeKey: &dedupeKey,
			Payload:   json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("first enqueue: %v", err)
		}

		// Second enqueue with same owner + kind + dedupeKey must return ErrDuplicateJob
		_, err = jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
			OwnerID:   ownerID,
			Kind:      "dedupe_kind",
			DedupeKey: &dedupeKey,
			Payload:   json.RawMessage(`{}`),
		})
		if !errors.Is(err, jobs.ErrDuplicateJob) {
			t.Fatalf("expected ErrDuplicateJob on duplicate dedupe key, got %v", err)
		}

		// Different owner with same dedupeKey should succeed
		_, err = jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
			OwnerID:   otherOwnerID,
			Kind:      "dedupe_kind",
			DedupeKey: &dedupeKey,
			Payload:   json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("enqueue for different owner should succeed: %v", err)
		}
	})

	// 11. Lease renewal
	t.Run("11. Lease renewal", func(t *testing.T) {
		job, err := jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
			OwnerID: ownerID,
			Kind:    "renew_kind",
			Payload: json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}

		claimed, err := jobRepo.ClaimNext(context.Background(), jobs.ClaimOptions{
			Kinds:         []string{"renew_kind"},
			LeaseDuration: 5 * time.Second,
		})
		if err != nil {
			t.Fatalf("claim: %v", err)
		}

		renewed, err := jobRepo.RenewLease(context.Background(), job.ID, *claimed.LeaseToken, 10*time.Second)
		if err != nil {
			t.Fatalf("renew lease: %v", err)
		}
		if renewed.LeaseUntil == nil || !renewed.LeaseUntil.After(*claimed.LeaseUntil) {
			t.Fatalf("expected extended lease deadline")
		}

		// Stale token should fail
		badToken := uuid.New()
		_, err = jobRepo.RenewLease(context.Background(), job.ID, badToken, 10*time.Second)
		if !errors.Is(err, jobs.ErrStaleLease) {
			t.Fatalf("expected ErrStaleLease on bad token renewal, got %v", err)
		}
	})
	_ = bytes.Equal
}
