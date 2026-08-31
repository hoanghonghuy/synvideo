package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/migrations"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

func TestProjectRepositoryIntegrationOwnerIsolation(t *testing.T) {
	pool := integrationPool(t)
	repository := NewProjectRepository(pool)
	ownerA := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	ownerB := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	projectA1, err := repository.Create(context.Background(), ownerA, validIntegrationCreateInput("Owner A Project 1"))
	if err != nil {
		t.Fatalf("create project A1: %v", err)
	}
	projectA2, err := repository.Create(context.Background(), ownerA, validIntegrationCreateInput("Owner A Project 2"))
	if err != nil {
		t.Fatalf("create project A2: %v", err)
	}
	projectB1, err := repository.Create(context.Background(), ownerB, validIntegrationCreateInput("Owner B Project 1"))
	if err != nil {
		t.Fatalf("create project B1: %v", err)
	}

	// List isolation
	listA, err := repository.List(context.Background(), ownerA, project.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("list owner A: %v", err)
	}
	if len(listA.Projects) != 2 {
		t.Fatalf("expected 2 projects for owner A, got %d", len(listA.Projects))
	}
	for _, p := range listA.Projects {
		if p.OwnerID != ownerA {
			t.Fatalf("expected project owner %s, got %s", ownerA, p.OwnerID)
		}
		if p.ID == projectB1.ID {
			t.Fatalf("owner A list contained owner B project %s", projectB1.ID)
		}
		if p.ID != projectA1.ID && p.ID != projectA2.ID {
			t.Fatalf("unexpected project %s in owner A list", p.ID)
		}
	}

	listB, err := repository.List(context.Background(), ownerB, project.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("list owner B: %v", err)
	}
	if len(listB.Projects) != 1 {
		t.Fatalf("expected 1 project for owner B, got %d", len(listB.Projects))
	}
	if listB.Projects[0].ID != projectB1.ID {
		t.Fatalf("expected owner B project %s, got %s", projectB1.ID, listB.Projects[0].ID)
	}

	// Get isolation
	if _, err := repository.Get(context.Background(), ownerA, projectB1.ID); err != project.ErrNotFound {
		t.Fatalf("expected owner A get owner B project to return not found, got %v", err)
	}
	if _, err := repository.Get(context.Background(), ownerB, projectA1.ID); err != project.ErrNotFound {
		t.Fatalf("expected owner B get owner A project to return not found, got %v", err)
	}

	// Update isolation
	title := "Should not update"
	if _, err := repository.Update(context.Background(), ownerA, projectB1.ID, project.UpdateInput{Title: &title}); err != project.ErrNotFound {
		t.Fatalf("expected owner A update owner B project to return not found, got %v", err)
	}
	if _, err := repository.Update(context.Background(), ownerB, projectA1.ID, project.UpdateInput{Title: &title}); err != project.ErrNotFound {
		t.Fatalf("expected owner B update owner A project to return not found, got %v", err)
	}
}

func TestProjectRepositoryIntegrationPaginationTieBreak(t *testing.T) {
	pool := integrationPool(t)
	repository := NewProjectRepository(pool)
	ownerID := uuid.MustParse("33333333-3333-4333-8333-333333333333")

	createdIDs := make([]uuid.UUID, 4)
	for i := 0; i < 4; i++ {
		item, err := repository.Create(context.Background(), ownerID, validIntegrationCreateInput("TieBreak Project"))
		if err != nil {
			t.Fatalf("create project %d: %v", i, err)
		}
		createdIDs[i] = item.ID
	}

	// Force identical updated_at across all 4 projects to strictly test the (updated_at, id) tie-breaker
	fixedTime := time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(context.Background(), `UPDATE projects SET updated_at = $1 WHERE owner_id = $2`, fixedTime, ownerID.String()); err != nil {
		t.Fatalf("force identical updated_at: %v", err)
	}

	firstPage, err := repository.List(context.Background(), ownerID, project.ListOptions{Limit: 2})
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(firstPage.Projects) != 2 {
		t.Fatalf("expected 2 first page projects, got %d", len(firstPage.Projects))
	}
	if firstPage.NextCursor == nil {
		t.Fatal("expected next cursor on page 1")
	}
	if firstPage.Projects[0].ID.String() <= firstPage.Projects[1].ID.String() {
		t.Fatalf("expected descending id ordering within identical timestamp, got %s <= %s",
			firstPage.Projects[0].ID, firstPage.Projects[1].ID)
	}

	secondPage, err := repository.List(context.Background(), ownerID, project.ListOptions{
		Limit:  2,
		Cursor: firstPage.NextCursor,
	})
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if len(secondPage.Projects) != 2 {
		t.Fatalf("expected 2 second page projects, got %d", len(secondPage.Projects))
	}
	if secondPage.Projects[0].ID.String() <= secondPage.Projects[1].ID.String() {
		t.Fatalf("expected descending id ordering on second page, got %s <= %s",
			secondPage.Projects[0].ID, secondPage.Projects[1].ID)
	}
	if firstPage.Projects[1].ID.String() <= secondPage.Projects[0].ID.String() {
		t.Fatalf("expected page 1 last id %s > page 2 first id %s",
			firstPage.Projects[1].ID, secondPage.Projects[0].ID)
	}

	seenIDs := make(map[uuid.UUID]bool)
	for _, p := range append(firstPage.Projects, secondPage.Projects...) {
		if seenIDs[p.ID] {
			t.Fatalf("duplicate project id %s encountered across pages", p.ID)
		}
		seenIDs[p.ID] = true
	}
	if len(seenIDs) != 4 {
		t.Fatalf("expected 4 unique projects across pages, got %d", len(seenIDs))
	}

	if secondPage.NextCursor != nil {
		t.Fatalf("expected nil next cursor on last page, got %v", secondPage.NextCursor)
	}
}

func TestProjectRepositoryUpdateNoOpPreservesUpdatedAt(t *testing.T) {
	pool := integrationPool(t)
	repository := NewProjectRepository(pool)
	ownerID := uuid.MustParse("44444444-4444-4444-8444-444444444444")

	created, err := repository.Create(context.Background(), ownerID, validIntegrationCreateInput("No-op target"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Update with empty input
	updated, err := repository.Update(context.Background(), ownerID, created.ID, project.UpdateInput{})
	if err != nil {
		t.Fatalf("update project: %v", err)
	}
	if !updated.UpdatedAt.Equal(created.UpdatedAt) {
		t.Fatalf("expected updated_at to remain %v, got %v", created.UpdatedAt, updated.UpdatedAt)
	}
}

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("SYNVIDEO_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SYNVIDEO_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE creative_briefs, projects`); err != nil {
		t.Fatalf("truncate projects: %v", err)
	}

	return pool
}

func validIntegrationCreateInput(title string) project.CreateInput {
	duration := 120
	return project.CreateInput{
		Title:                 title,
		Description:           "Integration test project",
		ContentFormat:         project.ContentFormatLong,
		AspectRatio:           project.AspectRatio16x9,
		TargetDurationSeconds: &duration,
		Locale:                project.LocaleVI,
	}
}
