package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

func TestSceneEditorRepositoryPostgresIntegration(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	projectRepo := NewProjectRepository(pool)
	repo := NewSceneEditorRepository(pool)

	ownerID := uuid.New()
	proj, err := projectRepo.Create(ctx, ownerID, validIntegrationCreateInput("Scene Editor Test Project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	now := time.Now().UTC()
	doc, err := sceneeditor.NewDocument(uuid.New(), ownerID, proj.ID, 1, []sceneeditor.Scene{{
		ID: uuid.New(), SceneKey: "intro", DurationMS: 2_000,
		VisualTreatment: sceneeditor.VisualTreatment{Fit: sceneeditor.FitContain, Scale: 1},
		TransitionOut:   sceneeditor.Transition{Kind: sceneeditor.TransitionCut},
	}}, nil, now)
	if err != nil {
		t.Fatalf("new document: %v", err)
	}

	created, err := repo.CreateInitial(ctx, doc)
	if err != nil {
		t.Fatalf("create initial: %v", err)
	}
	if created.ID != doc.ID || created.Revision != 1 || created.Scenes[0].ID != doc.Scenes[0].ID {
		t.Fatalf("unexpected initial: %+v", created)
	}

	revision2 := created
	revision2.Revision = 2
	revision2.Scenes = append([]sceneeditor.Scene(nil), created.Scenes...)
	revision2.Scenes[0].Notes = "creator note"
	revision2.UpdatedAt = now.Add(time.Second)
	updated, err := repo.CreateRevision(ctx, revision2, 1)
	if err != nil {
		t.Fatalf("create revision 2: %v", err)
	}
	if updated.Revision != 2 || updated.Scenes[0].Notes != "creator note" {
		t.Fatalf("unexpected revision 2: %+v", updated)
	}

	stale := updated
	stale.Revision = 2
	stale.UpdatedAt = now.Add(2 * time.Second)
	if _, err := repo.CreateRevision(ctx, stale, 1); !errors.Is(err, sceneeditor.ErrConflict) {
		t.Fatalf("stale write err=%v want conflict", err)
	}

	latest, err := repo.GetLatest(ctx, ownerID, proj.ID)
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if latest.Revision != 2 || latest.Scenes[0].Notes != "creator note" {
		t.Fatalf("latest=%+v", latest)
	}

	if _, err := repo.GetLatest(ctx, uuid.New(), proj.ID); !errors.Is(err, sceneeditor.ErrNotFound) {
		t.Fatalf("cross-owner read err=%v want not found", err)
	}

	snapshot, err := sceneeditor.NewSnapshot(latest, sceneeditor.StateCurrent)
	if err != nil {
		t.Fatalf("new snapshot: %v", err)
	}
	persisted, err := repo.CreateSnapshot(ctx, ownerID, snapshot)
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if persisted.Digest != snapshot.Digest || persisted.Revision != 2 {
		t.Fatalf("snapshot=%+v", persisted)
	}

	again, err := repo.CreateSnapshot(ctx, ownerID, snapshot)
	if err != nil {
		t.Fatalf("idempotent snapshot: %v", err)
	}
	if again.Digest != snapshot.Digest {
		t.Fatalf("digest=%s want %s", again.Digest, snapshot.Digest)
	}
}
