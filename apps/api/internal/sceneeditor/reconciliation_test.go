package sceneeditor

import (
	"context"
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

	updated, err := ApplyReconciliation(doc, candidate, 1, now.Add(time.Minute), uuid.New)
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

func TestReconciliationAddsScenesFromPlanCandidate(t *testing.T) {
	now := time.Now().UTC()
	doc, err := NewDocument(uuid.New(), uuid.New(), uuid.New(), 1, []Scene{{
		ID: uuid.New(), SceneKey: "intro", DurationMS: 2_000,
		VisualTreatment: VisualTreatment{Fit: FitContain, Scale: 1}, TransitionOut: Transition{Kind: TransitionCut},
	}}, nil, now)
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	candidate := ReconcileCandidate{ScenePlanVersion: 2, Scenes: []SceneCandidate{
		{SceneKey: "intro"},
		{SceneKey: "main"},
	}}
	preview, err := PreviewReconciliation(doc, candidate)
	if err != nil {
		t.Fatalf("PreviewReconciliation: %v", err)
	}
	if preview.Ambiguous {
		t.Fatalf("preview=%+v want non-ambiguous add", preview)
	}
	if len(preview.Changes) != 2 {
		t.Fatalf("changes=%+v want intro update and main add", preview.Changes)
	}
	if preview.Changes[1].SceneKey != "main" || preview.Changes[1].Reasons[0] != ReconcileSceneAdded {
		t.Fatalf("add change=%+v", preview.Changes[1])
	}

	updated, err := ApplyReconciliation(doc, candidate, 1, now, uuid.New)
	if err != nil {
		t.Fatalf("ApplyReconciliation: %v", err)
	}
	if len(updated.Scenes) != 2 || updated.Scenes[0].SceneKey != "intro" || updated.Scenes[1].SceneKey != "main" {
		t.Fatalf("scenes=%+v", updated.Scenes)
	}
	if updated.Scenes[0].ID == updated.Scenes[1].ID {
		t.Fatal("added scene must receive a new identity")
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
			if _, err := ApplyReconciliation(doc, candidate, 1, now, uuid.New); !errors.Is(err, ErrAmbiguousMapping) {
				t.Fatalf("err=%v want ambiguous mapping", err)
			}
		})
	}
}

func TestReconciliationRejectsRemovedAndRekeyedScenesAsAmbiguous(t *testing.T) {
	now := time.Now().UTC()
	doc, err := NewDocument(uuid.New(), uuid.New(), uuid.New(), 1, []Scene{
		{
			ID: uuid.New(), SceneKey: "intro", DurationMS: 1_000,
			VisualTreatment: VisualTreatment{Fit: FitContain, Scale: 1}, TransitionOut: Transition{Kind: TransitionCut},
		},
		{
			ID: uuid.New(), SceneKey: "main", DurationMS: 1_000,
			VisualTreatment: VisualTreatment{Fit: FitContain, Scale: 1}, TransitionOut: Transition{Kind: TransitionCut},
		},
	}, nil, now)
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	for name, candidate := range map[string]ReconcileCandidate{
		"removed": {ScenePlanVersion: 2, Scenes: []SceneCandidate{{SceneKey: "intro"}}},
		"rekeyed": {ScenePlanVersion: 2, Scenes: []SceneCandidate{{SceneKey: "hook"}}},
	} {
		t.Run(name, func(t *testing.T) {
			preview, err := PreviewReconciliation(doc, candidate)
			if err != nil {
				t.Fatalf("PreviewReconciliation: %v", err)
			}
			if !preview.Ambiguous {
				t.Fatalf("preview=%+v want ambiguous", preview)
			}
		})
	}
}

func TestReconciliationRejectsStalePreviewDigestWhenCandidateChanges(t *testing.T) {
	now := time.Now().UTC()
	doc, err := NewDocument(uuid.New(), uuid.New(), uuid.New(), 1, []Scene{{
		ID: uuid.New(), SceneKey: "intro", DurationMS: 1_000,
		VisualTreatment: VisualTreatment{Fit: FitContain, Scale: 1}, TransitionOut: Transition{Kind: TransitionCut},
	}}, nil, now)
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	candidate := ReconcileCandidate{ScenePlanVersion: 2, Scenes: []SceneCandidate{{SceneKey: "intro"}}}
	resolver := staticResolver{states: []DependencyState{{State: StateCurrent}}}
	service := NewService(&memoryRepository{latest: doc}, resolver, uuid.New, func() time.Time { return now })
	preview, err := service.PreviewReconcile(t.Context(), doc.OwnerID, doc.ProjectID, candidate)
	if err != nil {
		t.Fatalf("PreviewReconcile: %v", err)
	}

	drifted := candidate
	drifted.Scenes = []SceneCandidate{{SceneKey: "intro"}, {SceneKey: "main"}}
	digest, err := ReconcilePreviewDigest(doc.Revision, doc.ScenePlanVersion, drifted.ScenePlanVersion, drifted, mustDependencyFingerprint(t, resolver.states))
	if err != nil {
		t.Fatalf("ReconcilePreviewDigest: %v", err)
	}
	if digest == preview.PreviewDigest {
		t.Fatal("drifted candidate digest must differ from preview digest")
	}

	_, err = service.Reconcile(t.Context(), doc.OwnerID, doc.ProjectID, ReconcileInput{
		ExpectedRevision: 1,
		PreviewDigest:    preview.PreviewDigest,
		Candidate:        drifted,
	})
	if !errors.Is(err, ErrPreviewStale) {
		t.Fatalf("err=%v want preview stale", err)
	}
}

func TestServiceReconcileRejectsUpstreamDriftWithUnchangedCandidate(t *testing.T) {
	now := time.Now().UTC()
	ownerID := uuid.New()
	projectID := uuid.New()
	doc, err := NewDocument(uuid.New(), ownerID, projectID, 1, []Scene{{
		ID: uuid.New(), SceneKey: "intro", DurationMS: 2_000,
		VisualTreatment: VisualTreatment{Fit: FitContain, Scale: 1}, TransitionOut: Transition{Kind: TransitionCut},
	}}, nil, now)
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	resolver := &mutableResolver{states: []DependencyState{{State: StateCurrent, Reason: "PLAN_V2_CURRENT"}}}
	service := NewService(&memoryRepository{latest: doc}, resolver, uuid.New, func() time.Time { return now })
	candidate := ReconcileCandidate{ScenePlanVersion: 2, Scenes: []SceneCandidate{{SceneKey: "intro"}}}

	preview, err := service.PreviewReconcile(t.Context(), ownerID, projectID, candidate)
	if err != nil {
		t.Fatalf("PreviewReconcile: %v", err)
	}
	if preview.PreviewDigest == "" {
		t.Fatal("preview digest required")
	}

	resolver.states = []DependencyState{{State: StateStale, Reason: "SCENE_PLAN_SUPERSEDED"}}

	_, err = service.Reconcile(t.Context(), ownerID, projectID, ReconcileInput{
		ExpectedRevision: 1,
		PreviewDigest:    preview.PreviewDigest,
		Candidate:        candidate,
	})
	if !errors.Is(err, ErrPreviewStale) {
		t.Fatalf("err=%v want preview stale", err)
	}
}

func TestReconciliationRejectsDuplicateCandidateOnlySceneKeys(t *testing.T) {
	now := time.Now().UTC()
	doc, err := NewDocument(uuid.New(), uuid.New(), uuid.New(), 1, []Scene{{
		ID: uuid.New(), SceneKey: "intro", DurationMS: 1_000,
		VisualTreatment: VisualTreatment{Fit: FitContain, Scale: 1}, TransitionOut: Transition{Kind: TransitionCut},
	}}, nil, now)
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	candidate := ReconcileCandidate{ScenePlanVersion: 2, Scenes: []SceneCandidate{
		{SceneKey: "intro"},
		{SceneKey: "main"},
		{SceneKey: "main"},
	}}
	preview, err := PreviewReconciliation(doc, candidate)
	if err != nil {
		t.Fatalf("PreviewReconciliation: %v", err)
	}
	if !preview.Ambiguous {
		t.Fatalf("preview=%+v want ambiguous duplicate candidate-only keys", preview)
	}
	if _, err := ApplyReconciliation(doc, candidate, 1, now, uuid.New); !errors.Is(err, ErrAmbiguousMapping) {
		t.Fatalf("err=%v want ambiguous mapping", err)
	}
}

type mutableResolver struct {
	states []DependencyState
}

func (r *mutableResolver) State(context.Context, uuid.UUID, Document) ([]DependencyState, error) {
	return r.states, nil
}

func mustDependencyFingerprint(t *testing.T, states []DependencyState) string {
	fingerprint, err := DependencyFingerprint(states)
	if err != nil {
		t.Fatalf("DependencyFingerprint: %v", err)
	}
	return fingerprint
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
	if _, err := ApplyReconciliation(doc, candidate, 0, time.Now().UTC(), uuid.New); !errors.Is(err, ErrConflict) {
		t.Fatalf("err=%v want conflict", err)
	}
}
