package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/renderexport"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

func TestRenderExportServiceIntegrationHistoryPaginationTieBreakAndRestart(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	ownerID := uuid.New()
	projectRepo := NewProjectRepository(pool)
	projectItem, err := projectRepo.Create(ctx, ownerID, validIntegrationCreateInput("Render history pagination"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	projectID := projectItem.ID

	jobRepo := NewJobRepository(pool)
	service := renderexport.NewServiceWithRuntime(nil, jobRepo, jobRepo, nil, uuid.New)

	jobIDs := make([]uuid.UUID, 4)
	for i := range jobIDs {
		jobIDs[i] = uuid.New()
		if _, err := jobRepo.Enqueue(ctx, jobs.EnqueueInput{
			ID:          jobIDs[i],
			OwnerID:     ownerID,
			ProjectID:   &projectID,
			Kind:        renderexport.JobKind,
			MaxAttempts: 2,
			Payload:     integrationRenderPayload(),
		}); err != nil {
			t.Fatalf("enqueue render job %d: %v", i, err)
		}
	}

	fixedTime := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `
		UPDATE jobs
		SET created_at = $1, updated_at = $1
		WHERE owner_id = $2 AND project_id = $3 AND kind = $4;
	`, fixedTime, ownerID, projectID, renderexport.JobKind); err != nil {
		t.Fatalf("force identical created_at: %v", err)
	}

	firstPage, err := service.ListHistory(ctx, ownerID, projectID, 2, "")
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(firstPage.Items) != 2 {
		t.Fatalf("expected 2 items on first page, got %d", len(firstPage.Items))
	}
	if firstPage.NextCursor == "" {
		t.Fatal("expected next cursor on first page")
	}
	assertDescendingCreatedAtID(t, firstPage.Items)
	if firstPage.Items[0].ID.String() <= firstPage.Items[1].ID.String() {
		t.Fatalf("expected descending id tie-break within identical timestamp, got %s <= %s",
			firstPage.Items[0].ID, firstPage.Items[1].ID)
	}

	secondPage, err := service.ListHistory(ctx, ownerID, projectID, 2, firstPage.NextCursor)
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if len(secondPage.Items) != 2 {
		t.Fatalf("expected 2 items on second page, got %d", len(secondPage.Items))
	}
	assertDescendingCreatedAtID(t, secondPage.Items)
	if firstPage.Items[1].ID.String() <= secondPage.Items[0].ID.String() {
		t.Fatalf("expected page boundary id %s > next page first id %s",
			firstPage.Items[1].ID, secondPage.Items[0].ID)
	}
	if secondPage.NextCursor != "" {
		t.Fatalf("expected empty next cursor on last page, got %q", secondPage.NextCursor)
	}

	seen := map[uuid.UUID]bool{}
	for _, item := range append(firstPage.Items, secondPage.Items...) {
		if seen[item.ID] {
			t.Fatalf("duplicate history item %s across pages", item.ID)
		}
		seen[item.ID] = true
	}
	if len(seen) != 4 {
		t.Fatalf("expected 4 unique history items across pages, got %d", len(seen))
	}
	for _, id := range jobIDs {
		if !seen[id] {
			t.Fatalf("expected job %s in paginated history", id)
		}
	}

	restartFirst, err := service.ListHistory(ctx, ownerID, projectID, 2, "")
	if err != nil {
		t.Fatalf("restart first page: %v", err)
	}
	if restartFirst.NextCursor != firstPage.NextCursor {
		t.Fatalf("restart cursor mismatch: %q vs %q", restartFirst.NextCursor, firstPage.NextCursor)
	}
	for i := range firstPage.Items {
		if restartFirst.Items[i].ID != firstPage.Items[i].ID {
			t.Fatalf("restart page 1 item %d = %s, want %s", i, restartFirst.Items[i].ID, firstPage.Items[i].ID)
		}
	}
	restartSecond, err := service.ListHistory(ctx, ownerID, projectID, 2, restartFirst.NextCursor)
	if err != nil {
		t.Fatalf("restart second page: %v", err)
	}
	for i := range secondPage.Items {
		if restartSecond.Items[i].ID != secondPage.Items[i].ID {
			t.Fatalf("restart page 2 item %d = %s, want %s", i, restartSecond.Items[i].ID, secondPage.Items[i].ID)
		}
	}
}

func TestRenderExportServiceIntegrationHistoryOwnerProjectIsolation(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	ownerA := uuid.New()
	ownerB := uuid.New()
	projectRepo := NewProjectRepository(pool)
	projectA, err := projectRepo.Create(ctx, ownerA, validIntegrationCreateInput("Render history A"))
	if err != nil {
		t.Fatalf("create project A: %v", err)
	}
	projectB, err := projectRepo.Create(ctx, ownerA, validIntegrationCreateInput("Render history B"))
	if err != nil {
		t.Fatalf("create project B: %v", err)
	}
	projectOtherOwner, err := projectRepo.Create(ctx, ownerB, validIntegrationCreateInput("Render history other owner"))
	if err != nil {
		t.Fatalf("create other-owner project: %v", err)
	}

	jobRepo := NewJobRepository(pool)
	service := renderexport.NewServiceWithRuntime(nil, jobRepo, jobRepo, nil, uuid.New)

	projectAJob1 := uuid.New()
	projectAJob2 := uuid.New()
	projectBJob := uuid.New()
	seedRenderJob(t, jobRepo, ownerA, projectA.ID, projectAJob1)
	seedRenderJob(t, jobRepo, ownerA, projectA.ID, projectAJob2)
	seedRenderJob(t, jobRepo, ownerA, projectB.ID, projectBJob)
	foreignJobID := uuid.New()
	seedRenderJob(t, jobRepo, ownerB, projectOtherOwner.ID, foreignJobID)

	historyA, err := service.ListHistory(ctx, ownerA, projectA.ID, 10, "")
	if err != nil {
		t.Fatalf("list ownerA projectA: %v", err)
	}
	if len(historyA.Items) != 2 {
		t.Fatalf("ownerA projectA history = %d items, want 2", len(historyA.Items))
	}
	for _, item := range historyA.Items {
		if item.ID == foreignJobID {
			t.Fatalf("foreign owner job leaked into ownerA projectA history")
		}
	}

	historyB, err := service.ListHistory(ctx, ownerA, projectB.ID, 10, "")
	if err != nil {
		t.Fatalf("list ownerA projectB: %v", err)
	}
	if len(historyB.Items) != 1 {
		t.Fatalf("ownerA projectB history = %d items, want 1", len(historyB.Items))
	}

	crossOwnerHistory, err := service.ListHistory(ctx, ownerB, projectA.ID, 10, "")
	if err != nil {
		t.Fatalf("list ownerB projectA: %v", err)
	}
	if len(crossOwnerHistory.Items) != 0 {
		t.Fatalf("cross-owner history leaked %d items from ownerA projectA", len(crossOwnerHistory.Items))
	}

	if historyB.Items[0].ID != projectBJob {
		t.Fatalf("projectB history item = %s, want %s", historyB.Items[0].ID, projectBJob)
	}
	for _, item := range historyA.Items {
		if item.ID == projectBJob {
			t.Fatalf("cross-project history leaked projectB job %s into projectA", projectBJob)
		}
	}
}

func TestRenderExportServiceIntegrationRetryRejectsCrossScopeWithoutLeakage(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	ownerA := uuid.New()
	ownerB := uuid.New()
	projectRepo := NewProjectRepository(pool)
	projectA, err := projectRepo.Create(ctx, ownerA, validIntegrationCreateInput("Render retry scope A"))
	if err != nil {
		t.Fatalf("create project A: %v", err)
	}
	projectB, err := projectRepo.Create(ctx, ownerA, validIntegrationCreateInput("Render retry scope B"))
	if err != nil {
		t.Fatalf("create project B: %v", err)
	}

	jobRepo := NewJobRepository(pool)
	service := renderexport.NewServiceWithRuntime(nil, jobRepo, jobRepo, nil, uuid.New)
	sourceID := uuid.New()
	seedTerminalRenderJob(t, pool, jobRepo, ownerA, projectA.ID, sourceID, jobs.StateFailed)

	_, err = service.Retry(ctx, ownerB, projectA.ID, sourceID, uuid.New())
	if !errors.Is(err, renderexport.ErrRenderNotFound) {
		t.Fatalf("cross-owner retry error = %v, want ErrRenderNotFound", err)
	}
	if errors.Is(err, renderexport.ErrRenderNotRetryable) {
		t.Fatal("cross-owner retry leaked terminal-state conflict instead of not found")
	}

	_, err = service.Retry(ctx, ownerA, projectB.ID, sourceID, uuid.New())
	if !errors.Is(err, renderexport.ErrRenderNotFound) {
		t.Fatalf("cross-project retry error = %v, want ErrRenderNotFound", err)
	}
	if errors.Is(err, renderexport.ErrRenderNotRetryable) {
		t.Fatal("cross-project retry leaked terminal-state conflict instead of not found")
	}

	sourceAfter, err := jobRepo.GetByIDForProject(ctx, ownerA, projectA.ID, sourceID)
	if err != nil {
		t.Fatalf("source job after rejected retries: %v", err)
	}
	if sourceAfter.State != jobs.StateFailed {
		t.Fatalf("source job mutated to %s", sourceAfter.State)
	}
}

func seedRenderJob(t *testing.T, jobRepo *JobRepository, ownerID, projectID, jobID uuid.UUID) {
	t.Helper()
	if _, err := jobRepo.Enqueue(context.Background(), jobs.EnqueueInput{
		ID:          jobID,
		OwnerID:     ownerID,
		ProjectID:   &projectID,
		Kind:        renderexport.JobKind,
		MaxAttempts: 2,
		Payload:     integrationRenderPayload(),
	}); err != nil {
		t.Fatalf("enqueue render job %s: %v", jobID, err)
	}
}

func seedTerminalRenderJob(t *testing.T, pool *pgxpool.Pool, jobRepo *JobRepository, ownerID, projectID, jobID uuid.UUID, state jobs.State) {
	t.Helper()
	seedRenderJob(t, jobRepo, ownerID, projectID, jobID)
	ctx := context.Background()
	if state == jobs.StateSucceeded {
		if _, err := pool.Exec(ctx, `
			UPDATE jobs
			SET state = 'succeeded', error_code = NULL, finished_at = now(), updated_at = now()
			WHERE id = $1 AND owner_id = $2 AND project_id = $3;
		`, jobID, ownerID, projectID); err != nil {
			t.Fatalf("mark succeeded render job %s: %v", jobID, err)
		}
		return
	}
	if _, err := pool.Exec(ctx, `
		UPDATE jobs
		SET state = $1, error_code = 'ERR_TEST', finished_at = now(), updated_at = now()
		WHERE id = $2 AND owner_id = $3 AND project_id = $4;
	`, string(state), jobID, ownerID, projectID); err != nil {
		t.Fatalf("mark terminal render job %s: %v", jobID, err)
	}
}

func integrationRenderPayload() json.RawMessage {
	return json.RawMessage(fmt.Sprintf(
		`{"snapshot_digest":"%s","snapshot_schema":%d,"profile_id":"%s"}`,
		strings.Repeat("a", 64),
		sceneeditor.SnapshotSchemaVersion,
		renderexport.LocalProfileID,
	))
}

func assertDescendingCreatedAtID(t *testing.T, items []renderexport.JobView) {
	t.Helper()
	for i := 1; i < len(items); i++ {
		prev := items[i-1]
		curr := items[i]
		if prev.CreatedAt.Before(curr.CreatedAt) {
			t.Fatalf("history not sorted by created_at desc: %s before %s", prev.CreatedAt, curr.CreatedAt)
		}
		if prev.CreatedAt.Equal(curr.CreatedAt) && prev.ID.String() <= curr.ID.String() {
			t.Fatalf("history tie-break failed for equal timestamps: %s <= %s", prev.ID, curr.ID)
		}
	}
}
