package captions

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

type captionRepoFake struct {
	latest     Document
	created    Document
	createErr  error
	createCall int
}

func (f *captionRepoFake) GetLatest(context.Context, uuid.UUID, uuid.UUID, int, string) (Document, error) {
	if f.latest.ID == uuid.Nil {
		return Document{}, ErrNotFound
	}
	return f.latest, nil
}

func (f *captionRepoFake) GetRevision(context.Context, uuid.UUID, uuid.UUID, int, string, int) (Document, error) {
	return Document{}, ErrNotFound
}

func (f *captionRepoFake) ListHistory(context.Context, uuid.UUID, uuid.UUID, int, string) ([]Document, error) {
	if f.latest.ID == uuid.Nil {
		return []Document{}, nil
	}
	return []Document{f.latest}, nil
}

func (f *captionRepoFake) CreateInitial(_ context.Context, doc Document) (Document, error) {
	f.createCall++
	if f.createErr != nil {
		return Document{}, f.createErr
	}
	f.created = doc
	f.latest = doc
	return doc, nil
}

func (f *captionRepoFake) CreateRevision(_ context.Context, doc Document, expectedRevision int) (Document, error) {
	f.createCall++
	if f.createErr != nil {
		return Document{}, f.createErr
	}
	if f.latest.Revision != expectedRevision {
		return Document{}, ErrConflict
	}
	f.created = doc
	f.latest = doc
	return doc, nil
}

type planRepoFake struct{ plan sceneplan.Plan }

func (f planRepoFake) GetByVersion(context.Context, uuid.UUID, uuid.UUID, int) (sceneplan.Plan, error) {
	if f.plan.Version == 0 {
		return sceneplan.Plan{}, sceneplan.ErrNotFound
	}
	return f.plan, nil
}

type narrationRepoFake struct {
	binding scenenarration.Binding
	err     error
}

func (f narrationRepoFake) GetActive(context.Context, uuid.UUID, uuid.UUID, int, string) (scenenarration.Binding, error) {
	if f.err != nil {
		return scenenarration.Binding{}, f.err
	}
	return f.binding, nil
}

type assetRepoFake struct{ asset mediaasset.MediaAsset }

func (f assetRepoFake) Get(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (mediaasset.MediaAsset, error) {
	if f.asset.ID == uuid.Nil {
		return mediaasset.MediaAsset{}, mediaasset.ErrNotFound
	}
	return f.asset, nil
}

func TestGetMarksCaptionStaleWhenNarrationBindingChanges(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	oldBindingID := uuid.New()
	newBindingID := uuid.New()
	assetID := uuid.New()
	repo := &captionRepoFake{latest: testDocument(ownerID, projectID, oldBindingID, assetID, 1)}
	service := testService(repo, ownerID, projectID, newBindingID, assetID, 4.2)

	view, err := service.Get(context.Background(), project.Principal{OwnerID: ownerID}, projectID, 1, "scene-1")
	if err != nil {
		t.Fatal(err)
	}
	if view.State != StateStale {
		t.Fatalf("expected stale after narration lineage change, got %s", view.State)
	}
}

func TestUpdateRejectsStaleWriterWithoutCreatingRevision(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	bindingID := uuid.New()
	assetID := uuid.New()
	repo := &captionRepoFake{latest: testDocument(ownerID, projectID, bindingID, assetID, 3)}
	service := testService(repo, ownerID, projectID, bindingID, assetID, 4.2)

	_, err := service.Update(context.Background(), project.Principal{OwnerID: ownerID}, projectID, 1, "scene-1", UpdateInput{
		ExpectedRevision: 2,
		Segments:         repo.latest.Segments,
		Style:            repo.latest.Style,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected revision conflict, got %v", err)
	}
	if repo.createCall != 0 {
		t.Fatalf("stale writer created %d revisions", repo.createCall)
	}
}

func TestRebuildCreatesNewRevisionBoundToCurrentNarration(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	oldBindingID := uuid.New()
	oldAssetID := uuid.New()
	newBindingID := uuid.New()
	newAssetID := uuid.New()
	repo := &captionRepoFake{latest: testDocument(ownerID, projectID, oldBindingID, oldAssetID, 2)}
	service := testService(repo, ownerID, projectID, newBindingID, newAssetID, 5.25)

	view, err := service.Rebuild(context.Background(), project.Principal{OwnerID: ownerID}, projectID, 1, "scene-1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if view.State != StateCurrent || view.Revision != 3 {
		t.Fatalf("expected current revision 3, got state=%s revision=%d", view.State, view.Revision)
	}
	if view.SourceBindingID != newBindingID || view.SourceAssetID != newAssetID || view.SourceDurationMS != 5_250 {
		t.Fatalf("rebuild did not bind exact current source: %#v", view.Document)
	}
	if len(view.Segments) != 1 || view.Segments[0].StartMS != 0 || view.Segments[0].EndMS != 5_250 || view.Segments[0].Text != "Current narration" {
		t.Fatalf("unexpected rebuilt segments: %#v", view.Segments)
	}
}

func testDocument(ownerID, projectID, bindingID, assetID uuid.UUID, revision int) Document {
	return Document{
		ID:                 uuid.New(),
		OwnerID:            ownerID,
		ProjectID:          projectID,
		ScenePlanVersion:   1,
		SceneKey:           "scene-1",
		Revision:           revision,
		SourceBindingID:    bindingID,
		SourceAssetID:      assetID,
		SourceDurationMS:   4_200,
		Segments:           []Segment{{ID: uuid.New(), Text: "Old caption", StartMS: 0, EndMS: 4_200}},
		Style:              DefaultStyle(),
	}
}

func testService(repo Repository, ownerID, projectID, bindingID, assetID uuid.UUID, durationSeconds float64) *Service {
	metadata, _ := json.Marshal(map[string]float64{"duration_seconds": durationSeconds})
	return NewService(
		repo,
		planRepoFake{plan: sceneplan.Plan{
			ProjectID: projectID,
			Version:   1,
			Status:    sceneplan.StatusApproved,
			Scenes:    []sceneplan.Scene{{Key: "scene-1", Narration: "Current narration"}},
		}},
		narrationRepoFake{binding: scenenarration.Binding{
			ID:               bindingID,
			OwnerID:          ownerID,
			ProjectID:        projectID,
			ScenePlanVersion: 1,
			SceneKey:         "scene-1",
			AssetID:          assetID,
			Status:           scenenarration.StatusActive,
		}},
		assetRepoFake{asset: mediaasset.MediaAsset{
			ID:        assetID,
			OwnerID:   ownerID,
			ProjectID: projectID,
			Kind:      mediaasset.KindAudio,
			Metadata:  metadata,
		}},
	)
}
