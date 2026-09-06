package audiomix

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

type memoryRepository struct{ docs []Document }

func (r *memoryRepository) GetLatest(_ context.Context, ownerID, projectID uuid.UUID) (Document, error) {
	for i := len(r.docs) - 1; i >= 0; i-- {
		if r.docs[i].OwnerID == ownerID && r.docs[i].ProjectID == projectID {
			return r.docs[i], nil
		}
	}
	return Document{}, ErrNotFound
}

func (r *memoryRepository) GetRevision(_ context.Context, ownerID, projectID uuid.UUID, revision int) (Document, error) {
	for _, doc := range r.docs {
		if doc.OwnerID == ownerID && doc.ProjectID == projectID && doc.Revision == revision {
			return doc, nil
		}
	}
	return Document{}, ErrNotFound
}

func (r *memoryRepository) ListHistory(_ context.Context, ownerID, projectID uuid.UUID) ([]Document, error) {
	var result []Document
	for _, doc := range r.docs {
		if doc.OwnerID == ownerID && doc.ProjectID == projectID {
			result = append(result, doc)
		}
	}
	return result, nil
}

func (r *memoryRepository) CreateInitial(_ context.Context, doc Document) (Document, error) {
	if _, err := r.GetLatest(context.Background(), doc.OwnerID, doc.ProjectID); err == nil {
		return Document{}, ErrConflict
	}
	r.docs = append(r.docs, doc)
	return doc, nil
}

func (r *memoryRepository) CreateRevision(_ context.Context, doc Document, expectedRevision int) (Document, error) {
	latest, err := r.GetLatest(context.Background(), doc.OwnerID, doc.ProjectID)
	if err != nil {
		return Document{}, err
	}
	if latest.Revision != expectedRevision || doc.ID != latest.ID || doc.Revision != expectedRevision+1 {
		return Document{}, ErrConflict
	}
	r.docs = append(r.docs, doc)
	return doc, nil
}

type planRepository struct{ plans []sceneplan.Plan }

func (r planRepository) ListVersions(context.Context, uuid.UUID, uuid.UUID) ([]sceneplan.Plan, error) {
	return append([]sceneplan.Plan(nil), r.plans...), nil
}

type narrationRepository struct{ byPlan map[int][]scenenarration.Binding }

func (r *narrationRepository) ListActiveForPlan(_ context.Context, _ uuid.UUID, _ uuid.UUID, planVersion int) ([]scenenarration.Binding, error) {
	return append([]scenenarration.Binding(nil), r.byPlan[planVersion]...), nil
}

type assetRepository struct{ assets map[uuid.UUID]mediaasset.MediaAsset }

func (r *assetRepository) Get(_ context.Context, ownerID, projectID, assetID uuid.UUID) (mediaasset.MediaAsset, error) {
	asset, ok := r.assets[assetID]
	if !ok || asset.OwnerID != ownerID || asset.ProjectID != projectID {
		return mediaasset.MediaAsset{}, mediaasset.ErrNotFound
	}
	return asset, nil
}

type serviceFixture struct {
	ownerID    uuid.UUID
	projectID  uuid.UUID
	principal  project.Principal
	musicID    uuid.UUID
	bindings   *narrationRepository
	assets     *assetRepository
	repo       *memoryRepository
	service    *Service
	baseConfig Config
}

func newServiceFixture(t *testing.T) serviceFixture {
	t.Helper()
	ownerID := uuid.New()
	projectID := uuid.New()
	musicID := uuid.New()
	sceneOneAudio := uuid.New()
	sceneTwoAudio := uuid.New()
	plan := sceneplan.Plan{
		ProjectID: projectID,
		Version:   4,
		Status:    sceneplan.StatusApproved,
		Scenes: []sceneplan.Scene{
			{Key: "scene-1", Narration: "First"},
			{Key: "scene-2", Narration: "Second"},
		},
	}
	bindings := &narrationRepository{byPlan: map[int][]scenenarration.Binding{
		4: {
			{ID: uuid.New(), OwnerID: ownerID, ProjectID: projectID, ScenePlanVersion: 4, SceneKey: "scene-1", AssetID: sceneOneAudio, Status: scenenarration.StatusActive},
			{ID: uuid.New(), OwnerID: ownerID, ProjectID: projectID, ScenePlanVersion: 4, SceneKey: "scene-2", AssetID: sceneTwoAudio, Status: scenenarration.StatusActive},
		},
	}}
	assets := &assetRepository{assets: map[uuid.UUID]mediaasset.MediaAsset{
		musicID:       audioAsset(ownerID, projectID, musicID, 90),
		sceneOneAudio: audioAsset(ownerID, projectID, sceneOneAudio, 12.5),
		sceneTwoAudio: audioAsset(ownerID, projectID, sceneTwoAudio, 17.5),
	}}
	repo := &memoryRepository{}
	service := NewService(repo, planRepository{plans: []sceneplan.Plan{plan}}, bindings, assets)
	service.now = func() time.Time { return time.Unix(1_800_000_000, 0).UTC() }
	return serviceFixture{
		ownerID: ownerID, projectID: projectID, principal: project.Principal{OwnerID: ownerID}, musicID: musicID,
		bindings: bindings, assets: assets, repo: repo, service: service,
		baseConfig: Config{LoopPolicy: LoopToTarget, MusicGainDB: -10, NarrationGainDB: 0, Ducking: Ducking{Enabled: true, ReductionDB: 8, AttackMS: 100, ReleaseMS: 250}},
	}
}

func audioAsset(ownerID, projectID, assetID uuid.UUID, seconds float64) mediaasset.MediaAsset {
	metadata, _ := json.Marshal(map[string]any{"duration_seconds": seconds})
	return mediaasset.MediaAsset{ID: assetID, OwnerID: ownerID, ProjectID: projectID, Kind: mediaasset.KindAudio, Metadata: metadata}
}

func TestServiceCreateBuildsProjectNarrationLineage(t *testing.T) {
	f := newServiceFixture(t)
	view, err := f.service.Create(context.Background(), f.principal, f.projectID, CreateInput{MusicAssetID: f.musicID, Config: f.baseConfig})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if view.State != StateCurrent || view.ScenePlanVersion != 4 || view.NarrationLineageID == uuid.Nil {
		t.Fatalf("unexpected view: %+v", view)
	}
	if view.NarrationDurationMS != 30_000 {
		t.Fatalf("narration duration = %d, want 30000", view.NarrationDurationMS)
	}
	if len(f.repo.docs) != 1 {
		t.Fatalf("persisted revisions = %d, want 1", len(f.repo.docs))
	}
}

func TestServiceNarrationReplacementMarksExistingMixStale(t *testing.T) {
	f := newServiceFixture(t)
	created, err := f.service.Create(context.Background(), f.principal, f.projectID, CreateInput{MusicAssetID: f.musicID, Config: f.baseConfig})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	replacementID := uuid.New()
	f.assets.assets[replacementID] = audioAsset(f.ownerID, f.projectID, replacementID, 13)
	f.bindings.byPlan[4][0].ID = uuid.New()
	f.bindings.byPlan[4][0].AssetID = replacementID
	view, err := f.service.Get(context.Background(), f.principal, f.projectID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if view.State != StateStale {
		t.Fatalf("state = %s, want STALE", view.State)
	}
	if view.NarrationLineageID != created.NarrationLineageID {
		t.Fatal("read silently rebound persisted narration lineage")
	}
}

func TestServiceUpdateStaleMixDoesNotRebindNarration(t *testing.T) {
	f := newServiceFixture(t)
	created, err := f.service.Create(context.Background(), f.principal, f.projectID, CreateInput{MusicAssetID: f.musicID, Config: f.baseConfig})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	f.bindings.byPlan[4][0].ID = uuid.New()
	updatedConfig := f.baseConfig
	updatedConfig.MusicGainDB = -15
	view, err := f.service.Update(context.Background(), f.principal, f.projectID, UpdateInput{ExpectedRevision: created.Revision, MusicAssetID: f.musicID, Config: updatedConfig})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if view.State != StateStale || view.NarrationLineageID != created.NarrationLineageID {
		t.Fatalf("stale update rebound lineage or state: %+v", view)
	}
}

func TestServiceRebindNarrationIsExplicitAndVersioned(t *testing.T) {
	f := newServiceFixture(t)
	created, err := f.service.Create(context.Background(), f.principal, f.projectID, CreateInput{MusicAssetID: f.musicID, Config: f.baseConfig})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	f.bindings.byPlan[4][0].ID = uuid.New()
	stale, err := f.service.Get(context.Background(), f.principal, f.projectID)
	if err != nil || stale.State != StateStale {
		t.Fatalf("expected stale before rebind: view=%+v err=%v", stale, err)
	}
	rebound, err := f.service.RebindNarration(context.Background(), f.principal, f.projectID, created.Revision)
	if err != nil {
		t.Fatalf("RebindNarration: %v", err)
	}
	if rebound.State != StateCurrent || rebound.Revision != created.Revision+1 || rebound.NarrationLineageID == created.NarrationLineageID {
		t.Fatalf("unexpected rebound view: %+v", rebound)
	}
}

func TestServiceMissingMusicIsBrokenAndCannotSnapshot(t *testing.T) {
	f := newServiceFixture(t)
	created, err := f.service.Create(context.Background(), f.principal, f.projectID, CreateInput{MusicAssetID: f.musicID, Config: f.baseConfig})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	delete(f.assets.assets, created.MusicAssetID)
	view, err := f.service.Get(context.Background(), f.principal, f.projectID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if view.State != StateBroken {
		t.Fatalf("state = %s, want BROKEN", view.State)
	}
	if _, err := f.service.Snapshot(context.Background(), f.principal, f.projectID); !errors.Is(err, ErrBroken) {
		t.Fatalf("Snapshot error = %v, want ErrBroken", err)
	}
}

func TestServiceOptimisticRevisionConflict(t *testing.T) {
	f := newServiceFixture(t)
	created, err := f.service.Create(context.Background(), f.principal, f.projectID, CreateInput{MusicAssetID: f.musicID, Config: f.baseConfig})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, err = f.service.Update(context.Background(), f.principal, f.projectID, UpdateInput{ExpectedRevision: created.Revision + 1, MusicAssetID: f.musicID, Config: f.baseConfig})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Update error = %v, want ErrConflict", err)
	}
}
