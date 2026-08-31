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
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	otherOwnerID := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	created, err := repository.Create(context.Background(), ownerID, validIntegrationCreateInput("Owner project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if _, err := repository.Get(context.Background(), otherOwnerID, created.ID); err != project.ErrNotFound {
		t.Fatalf("expected other owner get to return not found, got %v", err)
	}

	title := "Should not update"
	if _, err := repository.Update(context.Background(), otherOwnerID, created.ID, project.UpdateInput{Title: &title}); err != project.ErrNotFound {
		t.Fatalf("expected other owner update to return not found, got %v", err)
	}
}

func TestProjectRepositoryIntegrationPaginationIsStable(t *testing.T) {
	pool := integrationPool(t)
	repository := NewProjectRepository(pool)
	ownerID := uuid.MustParse("33333333-3333-4333-8333-333333333333")

	for _, title := range []string{"Mot", "Hai", "Ba"} {
		if _, err := repository.Create(context.Background(), ownerID, validIntegrationCreateInput(title)); err != nil {
			t.Fatalf("create project %q: %v", title, err)
		}
		time.Sleep(5 * time.Millisecond)
	}

	firstPage, err := repository.List(context.Background(), ownerID, project.ListOptions{Limit: 2})
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(firstPage.Projects) != 2 {
		t.Fatalf("expected 2 first page projects, got %d", len(firstPage.Projects))
	}
	if firstPage.NextCursor == nil {
		t.Fatal("expected next cursor")
	}
	if !firstPage.Projects[0].UpdatedAt.After(firstPage.Projects[1].UpdatedAt) &&
		firstPage.Projects[0].ID.String() <= firstPage.Projects[1].ID.String() {
		t.Fatalf("expected deterministic descending order, got %#v", firstPage.Projects)
	}

	secondPage, err := repository.List(context.Background(), ownerID, project.ListOptions{
		Limit:  2,
		Cursor: firstPage.NextCursor,
	})
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if len(secondPage.Projects) != 1 {
		t.Fatalf("expected 1 second page project, got %d", len(secondPage.Projects))
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
	if _, err := pool.Exec(ctx, `TRUNCATE projects`); err != nil {
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
