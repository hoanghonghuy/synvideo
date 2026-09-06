package sceneeditor

import (
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
	CompositionSceneID uuid.UUID       `json:"composition_scene_id"`
	SceneKey           string          `json:"scene_key"`
	Reasons            []ReconcileReason `json:"reasons"`
	PreservesEdits     bool            `json:"preserves_edits"`
}

type ReconcilePreview struct {
	FromRevision         int               `json:"from_revision"`
	FromScenePlanVersion int               `json:"from_scene_plan_version"`
	ToScenePlanVersion   int               `json:"to_scene_plan_version"`
	Changes              []ReconcileChange `json:"changes"`
	AudioMixChanged      bool              `json:"audio_mix_changed"`
	Ambiguous            bool              `json:"ambiguous"`
}

func PreviewReconciliation(doc Document, candidate ReconcileCandidate) (ReconcilePreview, error) {
	if candidate.ScenePlanVersion < 1 {
		return ReconcilePreview{}, ValidationError{Fields: map[string]string{"scene_plan_version": "positive"}}
	}
	byKey := make(map[string]SceneCandidate, len(candidate.Scenes))
	duplicates := map[string]bool{}
	for _, scene := range candidate.Scenes {
		if scene.SceneKey == "" {
			return ReconcilePreview{}, ValidationError{Fields: map[string]string{"candidate.scene_key": "required"}}
		}
		if _, exists := byKey[scene.SceneKey]; exists {
			duplicates[scene.SceneKey] = true
		}
		byKey[scene.SceneKey] = scene
	}

	preview := ReconcilePreview{
		FromRevision: doc.Revision,
		FromScenePlanVersion: doc.ScenePlanVersion,
		ToScenePlanVersion: candidate.ScenePlanVersion,
		AudioMixChanged: !sameAudioMix(doc.AudioMix, candidate.AudioMix),
	}
	for _, scene := range doc.Scenes {
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
	return preview, nil
}

func ApplyReconciliation(doc Document, candidate ReconcileCandidate, expectedRevision int, now time.Time) (Document, error) {
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
	byKey := make(map[string]SceneCandidate, len(candidate.Scenes))
	for _, scene := range candidate.Scenes {
		byKey[scene.SceneKey] = scene
	}
	updated := doc
	updated.Scenes = cloneScenes(doc.Scenes)
	updated.ScenePlanVersion = candidate.ScenePlanVersion
	updated.AudioMix = cloneAudioMix(candidate.AudioMix)
	for i := range updated.Scenes {
		mapped := byKey[updated.Scenes[i].SceneKey]
		updated.Scenes[i].Visual = cloneVisual(mapped.Visual)
		updated.Scenes[i].Narration = cloneNarration(mapped.Narration)
		updated.Scenes[i].Caption = cloneCaption(mapped.Caption)
	}
	updated.Revision++
	updated.UpdatedAt = now
	if err := ValidateDocument(updated); err != nil {
		return Document{}, err
	}
	return updated, nil
}

func cloneVisual(in *VisualRef) *VisualRef {
	if in == nil { return nil }
	out := *in
	return &out
}

func cloneNarration(in *NarrationRef) *NarrationRef {
	if in == nil { return nil }
	out := *in
	return &out
}

func cloneCaption(in *CaptionRef) *CaptionRef {
	if in == nil { return nil }
	out := *in
	return &out
}

func sameVisual(a, b *VisualRef) bool {
	if a == nil || b == nil { return a == nil && b == nil }
	return *a == *b
}

func sameNarration(a, b *NarrationRef) bool {
	if a == nil || b == nil { return a == nil && b == nil }
	return *a == *b
}

func sameCaption(a, b *CaptionRef) bool {
	if a == nil || b == nil { return a == nil && b == nil }
	return *a == *b
}

func sameAudioMix(a, b *AudioMixRef) bool {
	if a == nil || b == nil { return a == nil && b == nil }
	return *a == *b
}
