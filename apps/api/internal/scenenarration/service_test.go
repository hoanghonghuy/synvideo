package scenenarration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

type fakeNarrationRepo struct {
	bindings []scenenarration.Binding
}

func (r *fakeNarrationRepo) GetActive(_ context.Context, ownerID, projectID uuid.UUID, planVersion int, sceneKey string) (scenenarration.Binding, error) {
	for _, b := range r.bindings {
		if b.OwnerID == ownerID && b.ProjectID == projectID && b.ScenePlanVersion == planVersion && b.SceneKey == sceneKey && b.Status == scenenarration.StatusActive {
			return b, nil
		}
	}
	return scenenarration.Binding{}, scenenarration.ErrNotFound
}

func (r *fakeNarrationRepo) ListActiveForPlan(_ context.Context, ownerID, projectID uuid.UUID, planVersion int) ([]scenenarration.Binding, error) {
	var res []scenenarration.Binding
	for _, b := range r.bindings {
		if b.OwnerID == ownerID && b.ProjectID == projectID && b.ScenePlanVersion == planVersion && b.Status == scenenarration.StatusActive {
			res = append(res, b)
		}
	}
	return res, nil
}

func (r *fakeNarrationRepo) Assign(_ context.Context, ownerID, projectID uuid.UUID, planVersion int, sceneKey string, assetID uuid.UUID) (scenenarration.Binding, error) {
	var maxVersion int
	var activeIdx = -1
	now := time.Now().UTC()

	for i, b := range r.bindings {
		if b.OwnerID == ownerID && b.ProjectID == projectID && b.ScenePlanVersion == planVersion && b.SceneKey == sceneKey {
			if b.BindingVersion > maxVersion {
				maxVersion = b.BindingVersion
			}
			if b.Status == scenenarration.StatusActive {
				activeIdx = i
			}
		}
	}

	if activeIdx >= 0 {
		if r.bindings[activeIdx].AssetID == assetID {
			return r.bindings[activeIdx], nil // idempotent
		}
		r.bindings[activeIdx].Status = scenenarration.StatusSuperseded
		r.bindings[activeIdx].SupersededAt = &now
	}

	newBinding := scenenarration.Binding{
		ID:               uuid.New(),
		OwnerID:          ownerID,
		ProjectID:        projectID,
		ScenePlanVersion: planVersion,
		SceneKey:         sceneKey,
		Role:             scenenarration.RoleNarration,
		BindingVersion:   maxVersion + 1,
		AssetID:          assetID,
		Status:           scenenarration.StatusActive,
		CreatedAt:        now,
	}
	r.bindings = append(r.bindings, newBinding)
	return newBinding, nil
}

func (r *fakeNarrationRepo) ListHistory(_ context.Context, ownerID, projectID uuid.UUID, planVersion int, sceneKey string) ([]scenenarration.Binding, error) {
	var res []scenenarration.Binding
	for i := len(r.bindings) - 1; i >= 0; i-- {
		b := r.bindings[i]
		if b.OwnerID == ownerID && b.ProjectID == projectID && b.ScenePlanVersion == planVersion && b.SceneKey == sceneKey {
			res = append(res, b)
		}
	}
	return res, nil
}

type fakePlanRepo struct {
	plans map[int]sceneplan.Plan
}

func (r *fakePlanRepo) GetByVersion(_ context.Context, _, projectID uuid.UUID, version int) (sceneplan.Plan, error) {
	p, ok := r.plans[version]
	if !ok || p.ProjectID != projectID {
		return sceneplan.Plan{}, sceneplan.ErrNotFound
	}
	return p, nil
}

type fakeAssetRepo struct {
	assets map[uuid.UUID]mediaasset.MediaAsset
}

func (r *fakeAssetRepo) Get(_ context.Context, ownerID, projectID, assetID uuid.UUID) (mediaasset.MediaAsset, error) {
	a, ok := r.assets[assetID]
	if !ok || a.OwnerID != ownerID || a.ProjectID != projectID {
		return mediaasset.MediaAsset{}, mediaasset.ErrNotFound
	}
	return a, nil
}

func TestSceneNarrationService(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	projectID := uuid.New()
	principal := project.Principal{OwnerID: ownerID}

	planRepo := &fakePlanRepo{
		plans: map[int]sceneplan.Plan{
			1: {
				ProjectID: projectID,
				Version:   1,
				Status:    sceneplan.StatusApproved,
				Scenes: []sceneplan.Scene{
					{Key: "sc-1", Narration: "Scene 1 narration"},
					{Key: "sc-2", Narration: "Scene 2 narration"},
				},
			},
			2: {
				ProjectID: projectID,
				Version:   2,
				Status:    sceneplan.StatusDraft,
				Scenes: []sceneplan.Scene{
					{Key: "sc-1", Narration: "Draft scene 1"},
				},
			},
		},
	}

	audioAsset1ID := uuid.New()
	audioAsset2ID := uuid.New()
	imageAssetID := uuid.New()

	assetRepo := &fakeAssetRepo{
		assets: map[uuid.UUID]mediaasset.MediaAsset{
			audioAsset1ID: {
				ID:        audioAsset1ID,
				OwnerID:   ownerID,
				ProjectID: projectID,
				Kind:      mediaasset.KindAudio,
				Origin:    mediaasset.OriginGeneratedAudio,
				MimeType:  "audio/mpeg",
			},
			audioAsset2ID: {
				ID:        audioAsset2ID,
				OwnerID:   ownerID,
				ProjectID: projectID,
				Kind:      mediaasset.KindAudio,
				Origin:    mediaasset.OriginGeneratedAudio,
				MimeType:  "audio/mpeg",
			},
			imageAssetID: {
				ID:        imageAssetID,
				OwnerID:   ownerID,
				ProjectID: projectID,
				Kind:      mediaasset.KindImage,
				Origin:    mediaasset.OriginGeneratedImage,
				MimeType:  "image/png",
			},
		},
	}

	narrationRepo := &fakeNarrationRepo{}
	svc := scenenarration.NewService(narrationRepo, planRepo, assetRepo)

	t.Run("Assign fails if plan is not approved", func(t *testing.T) {
		_, err := svc.AssignNarration(ctx, principal, projectID, 2, "sc-1", audioAsset1ID)
		if !errors.Is(err, scenenarration.ErrScenePlanNotApproved) {
			t.Fatalf("expected ErrScenePlanNotApproved, got %v", err)
		}
	})

	t.Run("Assign fails if scene key not in plan", func(t *testing.T) {
		_, err := svc.AssignNarration(ctx, principal, projectID, 1, "unknown-scene", audioAsset1ID)
		if !errors.Is(err, scenenarration.ErrSceneKeyNotFound) {
			t.Fatalf("expected ErrSceneKeyNotFound, got %v", err)
		}
	})

	t.Run("Assign fails if asset is not audio", func(t *testing.T) {
		_, err := svc.AssignNarration(ctx, principal, projectID, 1, "sc-1", imageAssetID)
		if !errors.Is(err, scenenarration.ErrMediaAssetNotAudio) {
			t.Fatalf("expected ErrMediaAssetNotAudio, got %v", err)
		}
	})

	t.Run("Assign creates active binding v1", func(t *testing.T) {
		b, err := svc.AssignNarration(ctx, principal, projectID, 1, "sc-1", audioAsset1ID)
		if err != nil {
			t.Fatalf("unexpected assign error: %v", err)
		}
		if b.BindingVersion != 1 || b.Status != scenenarration.StatusActive || b.AssetID != audioAsset1ID {
			t.Fatalf("unexpected binding: %+v", b)
		}
	})

	t.Run("Assigning same asset is idempotent", func(t *testing.T) {
		b, err := svc.AssignNarration(ctx, principal, projectID, 1, "sc-1", audioAsset1ID)
		if err != nil {
			t.Fatalf("unexpected assign error: %v", err)
		}
		if b.BindingVersion != 1 || b.Status != scenenarration.StatusActive {
			t.Fatalf("unexpected binding on idempotent assign: %+v", b)
		}
	})

	t.Run("Assigning alternative asset supersedes old and increments version", func(t *testing.T) {
		b2, err := svc.AssignNarration(ctx, principal, projectID, 1, "sc-1", audioAsset2ID)
		if err != nil {
			t.Fatalf("unexpected assign error: %v", err)
		}
		if b2.BindingVersion != 2 || b2.Status != scenenarration.StatusActive || b2.AssetID != audioAsset2ID {
			t.Fatalf("unexpected binding v2: %+v", b2)
		}

		history, err := svc.ListHistory(ctx, principal, projectID, 1, "sc-1")
		if err != nil {
			t.Fatalf("unexpected list history error: %v", err)
		}
		if len(history) != 2 {
			t.Fatalf("expected 2 history items, got %d", len(history))
		}
		if history[0].BindingVersion != 2 || history[0].Status != scenenarration.StatusActive {
			t.Fatalf("expected latest active first in history, got %+v", history[0])
		}
		if history[1].BindingVersion != 1 || history[1].Status != scenenarration.StatusSuperseded {
			t.Fatalf("expected previous superseded second in history, got %+v", history[1])
		}
	})

	t.Run("ListCurrent returns all scenes in plan order with bindings", func(t *testing.T) {
		current, err := svc.ListCurrent(ctx, principal, projectID, 1)
		if err != nil {
			t.Fatalf("unexpected list current error: %v", err)
		}
		if len(current) != 2 {
			t.Fatalf("expected 2 scenes, got %d", len(current))
		}
		if current[0].Scene.Key != "sc-1" || current[0].Binding == nil || current[0].Binding.AssetID != audioAsset2ID {
			t.Fatalf("unexpected scene 1 current: %+v", current[0])
		}
		if current[1].Scene.Key != "sc-2" || current[1].Binding != nil {
			t.Fatalf("unexpected scene 2 current (should be unbound): %+v", current[1])
		}
	})
}
