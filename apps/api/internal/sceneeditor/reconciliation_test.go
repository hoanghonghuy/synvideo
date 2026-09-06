package sceneeditor

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestReconciliationPreservesLocalIdentityAndPresentation(t *testing.T) {
	now := time.Date(2026, 9, 6, 5, 30, 0, 0, time.UTC)
	visual := &VisualRef{AssetID: uuid.New(), BindingID: uuid.New()}
	oldNarration := &NarrationRef{AssetID: uuid.New(), BindingID: uuid.New(), LineageID: uuid.New(), DurationMS: 1_500}
	newNarration := &NarrationRef{AssetID: uuid.New(), BindingID: uuid.New(), LineageID: uuid.New(), DurationMS: 1_800}
	sceneID := uuid.New()
	doc, err := NewDocument(uuid.New(), uuid.New(), uuid.New(), 1, []Scene{{
		ID: sceneID, SceneKey: "intro", Visual: visual, Narration: oldNarration, DurationMS: 2_000,
		VisualTreatment: VisualTreatment{Fit: FitCover, Scale: 1.25, PositionX: .1, PositionY: -.1},
		TransitionOut:   Transition{Kind: TransitionFade, DurationMS: 300}, Notes: "creator note",
	}}, nil, now)
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	candidate := ReconcileCandidate{ScenePlanVersion: 2, Scenes: []SceneCandidate{{SceneKey: "intro", Visual: visual, Narration: newNarration}}}
	preview, err := PreviewReconciliation(doc, candidate)
	if err != nil {
		t.Fatalf("PreviewReconciliation: %v", err)
	}
	if preview.Ambiguous || len(preview.Changes) != 1 || !preview.Changes[0].PreservesEdits {
		t.Fatalf("preview=%+v", preview)
	}

	updated, err := ApplyReconciliation(doc, candidate, 1, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ApplyReconciliation: %v", err)
	}
	if updated.ID != doc.ID || updated.Scenes[0].ID != sceneID {
		t.Fatal("composition and local scene identity must remain stable")
	}
	if updated.Revision != 2 || updated.ScenePlanVersion != 2 {
		t.Fatalf("revision=%d plan=%d", updated.Revision, updated.ScenePlanVersion)
	}
	if updated.Scenes[0].Narration.LineageID != newNarration.LineageID {
		t.Fatal("new narration lineage was not applied")
	}
	if updated.Scenes[0].Notes != "creator note" || updated.Scenes[0].VisualTreatment.Scale != 1.25 {
		t.Fatal("unrelated creator presentation edits were not preserved")
	}
}

func TestReconciliationRejectsMissingOrAmbiguousSceneKey(t *testing.T) {
	now := time.Now().UTC()
	doc, err := NewDocument(uuid.New(), uuid.New(), uuid.New(), 1, []Scene{{
		ID: uuid.New(), SceneKey: "intro", DurationMS: 1_000,
		VisualTreatment: VisualTreatment{Fit: FitContain, Scale: 1}, TransitionOut: Transition{Kind: TransitionCut},
	}}, nil, now)
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	for name, candidate := range map[string]ReconcileCandidate{
		"removed":   {ScenePlanVersion: 2, Scenes: []SceneCandidate{{SceneKey: "other"}}},
		"duplicate": {ScenePlanVersion: 2, Scenes: []SceneCandidate{{SceneKey: "intro"}, {SceneKey: "intro"}}},
	} {
		t.Run(name, func(t *testing.T) {
			preview, err := PreviewReconciliation(doc, candidate)
			if err != nil {
				t.Fatalf("PreviewReconciliation: %v", err)
			}
			if !preview.Ambiguous {
				t.Fatalf("preview=%+v want ambiguous", preview)
			}
			if _, err := ApplyReconciliation(doc, candidate, 1, now); !errors.Is(err, ErrAmbiguousMapping) {
				t.Fatalf("err=%v want ambiguous mapping", err)
			}
		})
	}
}

func TestReconciliationRejectsStaleWriter(t *testing.T) {
	doc, err := NewDocument(uuid.New(), uuid.New(), uuid.New(), 1, []Scene{{
		ID: uuid.New(), SceneKey: "intro", DurationMS: 1_000,
		VisualTreatment: VisualTreatment{Fit: FitContain, Scale: 1}, TransitionOut: Transition{Kind: TransitionCut},
	}}, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}
	candidate := ReconcileCandidate{ScenePlanVersion: 1, Scenes: []SceneCandidate{{SceneKey: "intro"}}}
	if _, err := ApplyReconciliation(doc, candidate, 0, time.Now().UTC()); !errors.Is(err, ErrConflict) {
		t.Fatalf("err=%v want conflict", err)
	}
}
