package sceneeditor

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCompositionReorderDuplicateRemoveAndSnapshot(t *testing.T) {
	now := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	ownerID := uuid.New()
	projectID := uuid.New()
	firstID := uuid.New()
	secondID := uuid.New()
	visualID := uuid.New()
	bindingID := uuid.New()
	narrationAssetID := uuid.New()
	narrationBindingID := uuid.New()
	lineageID := uuid.New()

	scenes := []Scene{
		{
			ID:              firstID,
			SceneKey:        "scene-a",
			Visual:          &VisualRef{AssetID: visualID, BindingID: bindingID},
			Narration:       &NarrationRef{AssetID: narrationAssetID, BindingID: narrationBindingID, LineageID: lineageID, DurationMS: 4_000},
			DurationMS:      5_000,
			VisualTreatment: VisualTreatment{Fit: FitCover, Scale: 1},
			TransitionOut:   Transition{Kind: TransitionCut},
		},
		{
			ID:              secondID,
			SceneKey:        "scene-b",
			DurationMS:      3_000,
			VisualTreatment: VisualTreatment{Fit: FitContain, Scale: 1},
			TransitionOut:   Transition{Kind: TransitionFade, DurationMS: 500},
		},
	}

	doc, err := NewDocument(uuid.New(), ownerID, projectID, 7, scenes, nil, now)
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	reordered, err := Reorder(doc, secondID, 0, 1, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("Reorder: %v", err)
	}
	if reordered.Revision != 2 {
		t.Fatalf("revision=%d want 2", reordered.Revision)
	}
	if reordered.Scenes[0].ID != secondID || reordered.Scenes[1].ID != firstID {
		t.Fatalf("unexpected order: %#v", reordered.Scenes)
	}

	duplicateID := uuid.New()
	duplicated, err := Duplicate(reordered, firstID, duplicateID, 2, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	if duplicated.Revision != 3 {
		t.Fatalf("revision=%d want 3", duplicated.Revision)
	}
	if duplicated.Scenes[2].ID != duplicateID {
		t.Fatalf("duplicate id=%s want %s", duplicated.Scenes[2].ID, duplicateID)
	}
	if duplicated.Scenes[2].SceneKey != "scene-a" {
		t.Fatalf("duplicate scene key=%q", duplicated.Scenes[2].SceneKey)
	}
	if duplicated.Scenes[2].Narration == duplicated.Scenes[1].Narration {
		t.Fatal("duplicate must not alias narration pointer")
	}

	removed, err := Remove(duplicated, secondID, 3, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if removed.Revision != 4 || len(removed.Scenes) != 2 {
		t.Fatalf("revision=%d scenes=%d", removed.Revision, len(removed.Scenes))
	}
	if removed.Scenes[0].ID != firstID || removed.Scenes[1].ID != duplicateID {
		t.Fatalf("unexpected scenes after remove: %#v", removed.Scenes)
	}

	snapshot, err := NewSnapshot(removed, StateCurrent)
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	if snapshot.Digest == "" {
		t.Fatal("snapshot digest must be populated")
	}
	if snapshot.CompositionID != removed.ID || snapshot.Revision != removed.Revision {
		t.Fatalf("snapshot identity mismatch")
	}

	removed.Scenes[0].Notes = "later edit"
	if snapshot.Scenes[0].Notes != "" {
		t.Fatal("snapshot must be immutable from later document mutation")
	}
}

func TestCompositionRejectsStaleWriterAndLastSceneRemoval(t *testing.T) {
	now := time.Now().UTC()
	doc, err := NewDocument(uuid.New(), uuid.New(), uuid.New(), 1, []Scene{{
		ID: uuid.New(), SceneKey: "scene-a", DurationMS: 1_000,
		VisualTreatment: VisualTreatment{Fit: FitContain, Scale: 1},
		TransitionOut:   Transition{Kind: TransitionCut},
	}}, nil, now)
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	if _, err := Reorder(doc, doc.Scenes[0].ID, 0, 0, now); err != ErrConflict {
		t.Fatalf("stale writer err=%v want %v", err, ErrConflict)
	}
	if _, err := Remove(doc, doc.Scenes[0].ID, 1, now); err != ErrLastScene {
		t.Fatalf("remove err=%v want %v", err, ErrLastScene)
	}
}

func TestCompositionDurationAndDependencyValidation(t *testing.T) {
	lineageID := uuid.New()
	base := Scene{
		ID: uuid.New(), SceneKey: "scene-a", DurationMS: 2_000,
		Narration:       &NarrationRef{AssetID: uuid.New(), BindingID: uuid.New(), LineageID: lineageID, DurationMS: 2_500},
		VisualTreatment: VisualTreatment{Fit: FitContain, Scale: 1},
		TransitionOut:   Transition{Kind: TransitionCut},
	}
	_, err := NewDocument(uuid.New(), uuid.New(), uuid.New(), 1, []Scene{base}, nil, time.Now().UTC())
	validation, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("err=%T %v want ValidationError", err, err)
	}
	if validation.Fields["scenes[0].duration_ms"] != "must_cover_narration" {
		t.Fatalf("fields=%v", validation.Fields)
	}

	if got := StateForDependencies(DependencyState{State: StateCurrent}, DependencyState{State: StateStale}); got != StateStale {
		t.Fatalf("state=%s want STALE", got)
	}
	if got := StateForDependencies(DependencyState{State: StateStale}, DependencyState{State: StateBroken}); got != StateBroken {
		t.Fatalf("state=%s want BROKEN", got)
	}
}

func TestSnapshotBlockedWhenDependenciesNotCurrent(t *testing.T) {
	doc, err := NewDocument(uuid.New(), uuid.New(), uuid.New(), 1, []Scene{{
		ID: uuid.New(), SceneKey: "scene-a", DurationMS: 1_000,
		VisualTreatment: VisualTreatment{Fit: FitContain, Scale: 1},
		TransitionOut:   Transition{Kind: TransitionCut},
	}}, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}
	if _, err := NewSnapshot(doc, StateStale); err != ErrSnapshotBlocked {
		t.Fatalf("err=%v want %v", err, ErrSnapshotBlocked)
	}
	if _, err := NewSnapshot(doc, StateBroken); err != ErrSnapshotBlocked {
		t.Fatalf("err=%v want %v", err, ErrSnapshotBlocked)
	}
}

func TestNormalizedCropAndTransitionValidation(t *testing.T) {
	base := Scene{
		ID: uuid.New(), SceneKey: "scene-a", DurationMS: 1_000,
		VisualTreatment: VisualTreatment{Fit: FitCover, Scale: 1, Crop: &Crop{X: .75, Y: 0, Width: .5, Height: 1}},
		TransitionOut:   Transition{Kind: TransitionCrossfade, DurationMS: 2_000},
	}
	_, err := NewDocument(uuid.New(), uuid.New(), uuid.New(), 1, []Scene{base}, nil, time.Now().UTC())
	validation, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("err=%T %v want ValidationError", err, err)
	}
	if validation.Fields["scenes[0].visual_treatment.crop"] == "" {
		t.Fatalf("missing crop validation: %v", validation.Fields)
	}
	if validation.Fields["scenes[0].transition_out"] != "must_fit_scene_duration" {
		t.Fatalf("missing transition validation: %v", validation.Fields)
	}
}
