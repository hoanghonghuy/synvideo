package sceneplan_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

func TestServiceRequiresAuthenticatedOwnerAndValidScope(t *testing.T) {
	repo := &fakeRepository{}
	service := sceneplan.NewService(repo)
	projectID := uuid.New()
	input := sceneplan.CreateDraftInput{SourceScriptVersion: 1, Content: serviceValidContent()}

	if _, err := service.CreateDraft(context.Background(), project.Principal{}, projectID, input); !errors.Is(err, sceneplan.ErrUnauthenticated) {
		t.Fatalf("create unauthenticated error = %v", err)
	}
	if _, err := service.CreateDraft(context.Background(), project.Principal{OwnerID: uuid.New()}, uuid.Nil, input); !errors.Is(err, sceneplan.ErrInvalidInput) {
		t.Fatalf("create invalid project error = %v", err)
	}
}

func TestServiceValidatesBeforeDelegatingAndPassesOwner(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	repo := &fakeRepository{}
	service := sceneplan.NewService(repo)

	created, err := service.CreateDraft(context.Background(), project.Principal{OwnerID: ownerID}, projectID, sceneplan.CreateDraftInput{
		SourceScriptVersion: 2,
		Content: sceneplan.Content{Scenes: []sceneplan.Scene{{
			Key: "scene-1", ScriptSectionKey: "intro", Narration: "hello", VisualInstruction: "  visual  ", PlannedSourceType: sceneplan.SourceTypeStock, ExpectedDurationSeconds: 4,
		}}},
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if repo.ownerID != ownerID || repo.projectID != projectID {
		t.Fatalf("repository scope = owner %s/project %s", repo.ownerID, repo.projectID)
	}
	if created.Scenes[0].VisualInstruction != "visual" {
		t.Fatalf("expected normalized content before delegation, got %q", created.Scenes[0].VisualInstruction)
	}

	invalid := sceneplan.CreateDraftInput{SourceScriptVersion: 0, Content: serviceValidContent()}
	if _, err := service.CreateDraft(context.Background(), project.Principal{OwnerID: ownerID}, projectID, invalid); err == nil {
		t.Fatal("expected invalid input before repository call")
	}
}

type fakeRepository struct {
	ownerID   uuid.UUID
	projectID uuid.UUID
}

func (r *fakeRepository) CreateDraft(_ context.Context, ownerID, projectID uuid.UUID, input sceneplan.CreateDraftInput) (sceneplan.Plan, error) {
	r.ownerID, r.projectID = ownerID, projectID
	return sceneplan.Plan{ProjectID: projectID, Scenes: input.Scenes}, nil
}

func (r *fakeRepository) UpdateDraft(context.Context, uuid.UUID, uuid.UUID, int, sceneplan.PutInput) (sceneplan.Plan, error) {
	return sceneplan.Plan{}, nil
}

func (r *fakeRepository) Approve(context.Context, uuid.UUID, uuid.UUID, int, int) (sceneplan.Plan, error) {
	return sceneplan.Plan{}, nil
}

func (r *fakeRepository) GetByVersion(context.Context, uuid.UUID, uuid.UUID, int) (sceneplan.Plan, error) {
	return sceneplan.Plan{}, nil
}

func (r *fakeRepository) ListVersions(context.Context, uuid.UUID, uuid.UUID) ([]sceneplan.Plan, error) {
	return nil, nil
}

func serviceValidContent() sceneplan.Content {
	return sceneplan.Content{Scenes: []sceneplan.Scene{{
		Key: "intro", ScriptSectionKey: "intro", Narration: "hello", VisualInstruction: "visual", PlannedSourceType: sceneplan.SourceTypeStock, ExpectedDurationSeconds: 5,
	}}}
}
