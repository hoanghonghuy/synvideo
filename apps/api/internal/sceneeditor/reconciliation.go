package sceneeditor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ReconcileReason string

const (
	ReconcileScenePlanChanged ReconcileReason = "SCENE_PLAN_CHANGED"
	ReconcileVisualChanged    ReconcileReason = "VISUAL_CHANGED"
	ReconcileNarrationChanged ReconcileReason = "NARRATION_CHANGED"
	ReconcileCaptionChanged   ReconcileReason = "CAPTION_CHANGED"
	ReconcileAudioMixChanged  ReconcileReason = "AUDIO_MIX_CHANGED"
	ReconcileMissingSource    ReconcileReason = "MISSING_SOURCE"
	ReconcileSceneAdded       ReconcileReason = "SCENE_ADDED"
)

type SceneCandidate struct {
	SceneKey  string        `json:"scene_key"`
	Visual    *VisualRef    `json:"visual,omitempty"`
	Narration *NarrationRef `json:"narration,omitempty"`
	Caption   *CaptionRef   `json:"caption,omitempty"`
}

type ReconcileCandidate struct {
	ScenePlanVersion int              `json:"scene_plan_version"`
	Scenes           []SceneCandidate `json:"scenes"`
	AudioMix         *AudioMixRef     `json:"audio_mix,omitempty"`
}

type ReconcileChange struct {
	CompositionSceneID uuid.UUID         `json:"composition_scene_id"`
	SceneKey           string            `json:"scene_key"`
	Reasons            []ReconcileReason `json:"reasons"`
	PreservesEdits     bool              `json:"preserves_edits"`
}

type ReconcilePreview struct {
	FromRevision         int               `json:"from_revision"`
	FromScenePlanVersion int               `json:"from_scene_plan_version"`
	ToScenePlanVersion   int               `json:"to_scene_plan_version"`
	Changes              []ReconcileChange `json:"changes"`
	AudioMixChanged      bool              `json:"audio_mix_changed"`
	Ambiguous            bool              `json:"ambiguous"`
	PreviewDigest        string            `json:"preview_digest"`
}

type reconcilePreviewDigestPayload struct {
	FromRevision         int                `json:"from_revision"`
	FromScenePlanVersion int                `json:"from_scene_plan_version"`
	ToScenePlanVersion   int                `json:"to_scene_plan_version"`
	Candidate            ReconcileCandidate `json:"candidate"`
}

func ReconcilePreviewDigest(fromRevision, fromScenePlanVersion, toScenePlanVersion int, candidate ReconcileCandidate) (string, error) {
	payload := reconcilePreviewDigestPayload{
		FromRevision:         fromRevision,
		FromScenePlanVersion: fromScenePlanVersion,
		ToScenePlanVersion:   toScenePlanVersion,
		Candidate:            candidate,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func PreviewReconciliation(doc Document, candidate ReconcileCandidate) (ReconcilePreview, error) {
	if candidate.ScenePlanVersion < 1 {
		return ReconcilePreview{}, ValidationError{Fields: map[string]string{"scene_plan_version": "positive"}}
	}
	byKey, duplicates, err := indexCandidateScenes(candidate.Scenes)
	if err != nil {
		return ReconcilePreview{}, err
	}

	preview := ReconcilePreview{
		FromRevision:         doc.Revision,
		FromScenePlanVersion: doc.ScenePlanVersion,
		ToScenePlanVersion:   candidate.ScenePlanVersion,
		AudioMixChanged:      !sameAudioMix(doc.AudioMix, candidate.AudioMix),
	}
	compositionKeys := map[string]struct{}{}
	for _, scene := range doc.Scenes {
		compositionKeys[scene.SceneKey] = struct{}{}
		change := ReconcileChange{CompositionSceneID: scene.ID, SceneKey: scene.SceneKey, PreservesEdits: true}
		candidateScene, exists := byKey[scene.SceneKey]
		if !exists || duplicates[scene.SceneKey] {
			change.Reasons = append(change.Reasons, ReconcileMissingSource)
			change.PreservesEdits = false
			preview.Ambiguous = true
			preview.Changes = append(preview.Changes, change)
			continue
		}
		if candidate.ScenePlanVersion != doc.ScenePlanVersion {
			change.Reasons = append(change.Reasons, ReconcileScenePlanChanged)
		}
		if !sameVisual(scene.Visual, candidateScene.Visual) {
			change.Reasons = append(change.Reasons, ReconcileVisualChanged)
		}
		if !sameNarration(scene.Narration, candidateScene.Narration) {
			change.Reasons = append(change.Reasons, ReconcileNarrationChanged)
		}
		if !sameCaption(scene.Caption, candidateScene.Caption) {
			change.Reasons = append(change.Reasons, ReconcileCaptionChanged)
		}
		if len(change.Reasons) > 0 {
			preview.Changes = append(preview.Changes, change)
		}
	}

	for _, candidateScene := range candidate.Scenes {
		if duplicates[candidateScene.SceneKey] {
			continue
		}
		if _, exists := compositionKeys[candidateScene.SceneKey]; exists {
			continue
		}
		preview.Changes = append(preview.Changes, ReconcileChange{
			SceneKey:       candidateScene.SceneKey,
			Reasons:        []ReconcileReason{ReconcileSceneAdded},
			PreservesEdits: true,
		})
	}

	digest, err := ReconcilePreviewDigest(doc.Revision, doc.ScenePlanVersion, candidate.ScenePlanVersion, candidate)
	if err != nil {
		return ReconcilePreview{}, err
	}
	preview.PreviewDigest = digest
	return preview, nil
}

func ApplyReconciliation(doc Document, candidate ReconcileCandidate, expectedRevision int, now time.Time, newID func() uuid.UUID) (Document, error) {
	if doc.Revision != expectedRevision {
		return Document{}, ErrConflict
	}
	preview, err := PreviewReconciliation(doc, candidate)
	if err != nil {
		return Document{}, err
	}
	if preview.Ambiguous {
		return Document{}, ErrAmbiguousMapping
	}
	if newID == nil {
		newID = uuid.New
	}

	byKey, _, err := indexCandidateScenes(candidate.Scenes)
	if err != nil {
		return Document{}, err
	}
	compositionByKey := make(map[string]Scene, len(doc.Scenes))
	for _, scene := range doc.Scenes {
		compositionByKey[scene.SceneKey] = scene
	}

	updated := doc
	updated.Scenes = make([]Scene, 0, len(candidate.Scenes))
	updated.ScenePlanVersion = candidate.ScenePlanVersion
	updated.AudioMix = cloneAudioMix(candidate.AudioMix)
	for _, candidateScene := range candidate.Scenes {
		mapped := byKey[candidateScene.SceneKey]
		if existing, ok := compositionByKey[candidateScene.SceneKey]; ok {
			scene := existing
			scene.Visual = cloneVisual(mapped.Visual)
			scene.Narration = cloneNarration(mapped.Narration)
			scene.Caption = cloneCaption(mapped.Caption)
			updated.Scenes = append(updated.Scenes, scene)
			continue
		}
		updated.Scenes = append(updated.Scenes, newSceneFromCandidate(mapped, newID()))
	}
	updated.Revision++
	updated.UpdatedAt = now
	if err := ValidateDocument(updated); err != nil {
		return Document{}, err
	}
	return updated, nil
}

func indexCandidateScenes(scenes []SceneCandidate) (map[string]SceneCandidate, map[string]bool, error) {
	byKey := make(map[string]SceneCandidate, len(scenes))
	duplicates := map[string]bool{}
	for _, scene := range scenes {
		if scene.SceneKey == "" {
			return nil, nil, ValidationError{Fields: map[string]string{"candidate.scene_key": "required"}}
		}
		if _, exists := byKey[scene.SceneKey]; exists {
			duplicates[scene.SceneKey] = true
		}
		byKey[scene.SceneKey] = scene
	}
	return byKey, duplicates, nil
}

func newSceneFromCandidate(candidate SceneCandidate, id uuid.UUID) Scene {
	durationMS := int64(2_000)
	if candidate.Narration != nil && candidate.Narration.DurationMS > durationMS {
		durationMS = candidate.Narration.DurationMS
	}
	if candidate.Caption != nil && candidate.Caption.LastEndMS > durationMS {
		durationMS = candidate.Caption.LastEndMS
	}
	return Scene{
		ID:              id,
		SceneKey:        candidate.SceneKey,
		Visual:          cloneVisual(candidate.Visual),
		Narration:       cloneNarration(candidate.Narration),
		Caption:         cloneCaption(candidate.Caption),
		DurationMS:      durationMS,
		VisualTreatment: VisualTreatment{Fit: FitContain, Scale: 1},
		TransitionOut:   Transition{Kind: TransitionCut},
	}
}

func cloneVisual(in *VisualRef) *VisualRef {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneNarration(in *NarrationRef) *NarrationRef {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneCaption(in *CaptionRef) *CaptionRef {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func sameVisual(a, b *VisualRef) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameNarration(a, b *NarrationRef) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameCaption(a, b *CaptionRef) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameAudioMix(a, b *AudioMixRef) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
