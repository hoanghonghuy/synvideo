package scenemedia

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

type fakeScenePlanReader struct {
	plans map[int]sceneplan.Plan
}

func (f fakeScenePlanReader) GetByVersion(_ context.Context, _ uuid.UUID, _ uuid.UUID, version int) (sceneplan.Plan, error) {
	plan, ok := f.plans[version]
	if !ok {
		return sceneplan.Plan{}, sceneplan.ErrNotFound
	}
	return plan, nil
}

type fakeAssetReader struct {
	assets map[uuid.UUID]mediaasset.MediaAsset
}

func (f fakeAssetReader) Get(_ context.Context, _ uuid.UUID, _ uuid.UUID, id uuid.UUID) (mediaasset.MediaAsset, error) {
	asset, ok := f.assets[id]
	if !ok {
		return mediaasset.MediaAsset{}, mediaasset.ErrNotFound
	}
	return asset, nil
}

type fakeBindingRepository struct {
	nextVersion map[string]int
	active      map[string]Binding
	history     map[string][]Binding
}

func newFakeBindingRepository() *fakeBindingRepository {
	return &fakeBindingRepository{nextVersion: map[string]int{}, active: map[string]Binding{}, history: map[string][]Binding{}}
}

func (f *fakeBindingRepository) AssignPrimaryVisual(_ context.Context, ownerID, projectID uuid.UUID, planVersion int, sceneKey string, assetID uuid.UUID) (Binding, error) {
	key := bindingKey(ownerID, projectID, planVersion, sceneKey)
	if active, ok := f.active[key]; ok && active.AssetID == assetID {
		return active, nil
	}
	f.nextVersion[key]++
	binding := Binding{
		ID:               uuid.New(),
		OwnerID:          ownerID,
		ProjectID:        projectID,
		ScenePlanVersion: planVersion,
		SceneKey:         sceneKey,
		Role:             RolePrimaryVisual,
		BindingVersion:   f.nextVersion[key],
		AssetID:          assetID,
		Status:           StatusActive,
		CreatedAt:        time.Now().UTC(),
	}
	if active, ok := f.active[key]; ok {
		active.Status = StatusSuperseded
		f.history[key] = append(f.history[key], active)
	}
	f.active[key] = binding
	f.history[key] = append(f.history[key], binding)
	return binding, nil
}

func (f *fakeBindingRepository) GetCurrent(_ context.Context, ownerID, projectID uuid.UUID, planVersion int, sceneKey string) (Binding, error) {
	binding, ok := f.active[bindingKey(ownerID, projectID, planVersion, sceneKey)]
	if !ok {
		return Binding{}, ErrNotFound
	}
	return binding, nil
}

func (f *fakeBindingRepository) ListCurrent(_ context.Context, ownerID, projectID uuid.UUID, planVersion int) ([]Binding, error) {
	result := make([]Binding, 0)
	prefix := bindingKey(ownerID, projectID, planVersion, "")
	for key, binding := range f.active {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			result = append(result, binding)
		}
	}
	return result, nil
}

func (f *fakeBindingRepository) ListHistory(_ context.Context, ownerID, projectID uuid.UUID, planVersion int, sceneKey string) ([]Binding, error) {
	items := f.history[bindingKey(ownerID, projectID, planVersion, sceneKey)]
	result := make([]Binding, len(items))
	for i := range items {
		result[len(items)-1-i] = items[i]
	}
	return result, nil
}

func TestServiceAssignPrimaryVisualValidatesPlanSceneAndAsset(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	plan := sceneplan.Plan{
		ProjectID: projectID,
		Version:   3,
		Status:    sceneplan.StatusApproved,
		Scenes: []sceneplan.Scene{
			{Key: "intro", PlannedSourceType: sceneplan.SourceTypeStock},
		},
	}
	assetID := uuid.New()
	asset := mediaasset.MediaAsset{ID: assetID, OwnerID: ownerID, ProjectID: projectID, Kind: mediaasset.KindImage}
	repo := newFakeBindingRepository()
	service := NewService(
		fakeScenePlanReader{plans: map[int]sceneplan.Plan{3: plan}},
		fakeAssetReader{assets: map[uuid.UUID]mediaasset.MediaAsset{assetID: asset}},
		repo,
	)

	got, err := service.AssignPrimaryVisual(context.Background(), project.Principal{OwnerID: ownerID}, projectID, 3, "intro", assetID)
	if err != nil {
		t.Fatalf("assign primary visual: %v", err)
	}
	if got.BindingVersion != 1 || got.Status != StatusActive || got.Role != RolePrimaryVisual {
		t.Fatalf("unexpected binding: %+v", got)
	}

	// Planned source intent does not constrain the actual visual asset origin.
	if asset.Origin == mediaasset.OriginStock {
		t.Fatal("test asset must prove planned source override")
	}

	replayed, err := service.AssignPrimaryVisual(context.Background(), project.Principal{OwnerID: ownerID}, projectID, 3, "intro", assetID)
	if err != nil {
		t.Fatalf("idempotent assignment: %v", err)
	}
	if replayed.ID != got.ID || replayed.BindingVersion != got.BindingVersion {
		t.Fatalf("same asset assignment created history: got=%+v first=%+v", replayed, got)
	}
}

func TestServiceAssignPrimaryVisualRejectsInvalidSelection(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	assetID := uuid.New()
	basePlan := sceneplan.Plan{ProjectID: projectID, Version: 1, Status: sceneplan.StatusApproved, Scenes: []sceneplan.Scene{{Key: "intro"}}}
	asset := mediaasset.MediaAsset{ID: assetID, OwnerID: ownerID, ProjectID: projectID, Kind: mediaasset.KindAudio}
	service := NewService(
		fakeScenePlanReader{plans: map[int]sceneplan.Plan{
			1: basePlan,
			2: {ProjectID: projectID, Version: 2, Status: sceneplan.StatusDraft, Scenes: []sceneplan.Scene{{Key: "intro"}}},
		}},
		fakeAssetReader{assets: map[uuid.UUID]mediaasset.MediaAsset{assetID: asset}},
		newFakeBindingRepository(),
	)
	principal := project.Principal{OwnerID: ownerID}

	cases := []struct {
		name  string
		plan  int
		scene string
		want  error
	}{
		{name: "draft plan", plan: 2, scene: "intro", want: ErrScenePlanNotApproved},
		{name: "unknown scene", plan: 1, scene: "missing", want: ErrSceneKeyNotFound},
		{name: "nonvisual asset", plan: 1, scene: "intro", want: ErrMediaAssetNotVisual},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.AssignPrimaryVisual(context.Background(), principal, projectID, tc.plan, tc.scene, assetID)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error=%v, want %v", err, tc.want)
			}
		})
	}
}

func TestServiceListCurrentPreservesPlanOrderAndUnboundScenes(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	assetID := uuid.New()
	plan := sceneplan.Plan{
		ProjectID: projectID,
		Version:   1,
		Status:    sceneplan.StatusApproved,
		Scenes: []sceneplan.Scene{
			{Key: "intro"},
			{Key: "middle"},
			{Key: "outro"},
		},
	}
	repo := newFakeBindingRepository()
	service := NewService(
		fakeScenePlanReader{plans: map[int]sceneplan.Plan{1: plan}},
		fakeAssetReader{assets: map[uuid.UUID]mediaasset.MediaAsset{assetID: {ID: assetID, OwnerID: ownerID, ProjectID: projectID, Kind: mediaasset.KindVideo}}},
		repo,
	)
	principal := project.Principal{OwnerID: ownerID}
	if _, err := service.AssignPrimaryVisual(context.Background(), principal, projectID, 1, "middle", assetID); err != nil {
		t.Fatalf("assign middle: %v", err)
	}

	items, err := service.ListCurrent(context.Background(), principal, projectID, 1)
	if err != nil {
		t.Fatalf("list current: %v", err)
	}
	if len(items) != 3 || items[0].Scene.Key != "intro" || items[1].Scene.Key != "middle" || items[2].Scene.Key != "outro" {
		t.Fatalf("unexpected scene order: %+v", items)
	}
	if items[0].Binding != nil || items[2].Binding != nil || items[1].Binding == nil {
		t.Fatalf("expected unbound scenes to be represented: %+v", items)
	}
}
