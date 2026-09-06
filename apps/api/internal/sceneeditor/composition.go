package sceneeditor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
)

type State string

type TransitionKind string

type FitMode string

const (
	StateCurrent State = "CURRENT"
	StateStale   State = "STALE"
	StateBroken  State = "BROKEN"

	TransitionCut       TransitionKind = "cut"
	TransitionFade      TransitionKind = "fade"
	TransitionCrossfade TransitionKind = "crossfade"

	FitContain FitMode = "contain"
	FitCover   FitMode = "cover"

	SnapshotSchemaVersion = 1
)

var (
	ErrInvalidInput     = errors.New("scene editor input is invalid")
	ErrConflict         = errors.New("scene editor revision conflict")
	ErrLastScene        = errors.New("scene editor requires at least one scene")
	ErrSceneNotFound    = errors.New("scene editor scene not found")
	ErrSnapshotBlocked  = errors.New("scene editor snapshot blocked by dependency state")
	ErrAmbiguousMapping = errors.New("scene editor reconciliation mapping is ambiguous")
)

type ValidationError struct{ Fields map[string]string }

func (e ValidationError) Error() string { return "scene editor validation failed" }

type Crop struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type VisualTreatment struct {
	Fit       FitMode `json:"fit"`
	Crop      *Crop   `json:"crop,omitempty"`
	PositionX float64 `json:"position_x"`
	PositionY float64 `json:"position_y"`
	Scale     float64 `json:"scale"`
	MuteVideo bool    `json:"mute_video"`
}

type Transition struct {
	Kind       TransitionKind `json:"kind"`
	DurationMS int64          `json:"duration_ms"`
}

type VisualRef struct {
	AssetID   uuid.UUID `json:"asset_id"`
	BindingID uuid.UUID `json:"binding_id"`
}

type NarrationRef struct {
	AssetID    uuid.UUID `json:"asset_id"`
	BindingID  uuid.UUID `json:"binding_id"`
	LineageID  uuid.UUID `json:"lineage_id"`
	DurationMS int64     `json:"duration_ms"`
}

type CaptionRef struct {
	DocumentID uuid.UUID `json:"document_id"`
	Revision   int       `json:"revision"`
	LineageID  uuid.UUID `json:"lineage_id"`
	LastEndMS  int64     `json:"last_end_ms"`
}

type AudioMixRef struct {
	DocumentID         uuid.UUID `json:"document_id"`
	Revision           int       `json:"revision"`
	MusicAssetID       uuid.UUID `json:"music_asset_id"`
	NarrationLineageID uuid.UUID `json:"narration_lineage_id"`
}

type Scene struct {
	ID              uuid.UUID       `json:"id"`
	SceneKey        string          `json:"scene_key"`
	Visual          *VisualRef      `json:"visual,omitempty"`
	Narration       *NarrationRef   `json:"narration,omitempty"`
	Caption         *CaptionRef     `json:"caption,omitempty"`
	DurationMS      int64           `json:"duration_ms"`
	VisualTreatment VisualTreatment `json:"visual_treatment"`
	TransitionOut   Transition      `json:"transition_out"`
	Notes           string          `json:"notes,omitempty"`
}

type Document struct {
	ID               uuid.UUID    `json:"id"`
	OwnerID          uuid.UUID    `json:"-"`
	ProjectID        uuid.UUID    `json:"project_id"`
	Revision         int          `json:"revision"`
	ScenePlanVersion int          `json:"scene_plan_version"`
	Scenes           []Scene      `json:"scenes"`
	AudioMix         *AudioMixRef `json:"audio_mix,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
}

type DependencyState struct {
	State  State  `json:"state"`
	Reason string `json:"reason,omitempty"`
}

type View struct {
	Document
	State State `json:"state"`
}

type Snapshot struct {
	SchemaVersion    int          `json:"schema_version"`
	CompositionID    uuid.UUID    `json:"composition_id"`
	Revision         int          `json:"revision"`
	ProjectID        uuid.UUID    `json:"project_id"`
	ScenePlanVersion int          `json:"scene_plan_version"`
	Scenes           []Scene      `json:"scenes"`
	AudioMix         *AudioMixRef `json:"audio_mix,omitempty"`
	Digest           string       `json:"digest"`
}

func NewDocument(id, ownerID, projectID uuid.UUID, scenePlanVersion int, scenes []Scene, audioMix *AudioMixRef, now time.Time) (Document, error) {
	doc := Document{
		ID: id, OwnerID: ownerID, ProjectID: projectID, Revision: 1,
		ScenePlanVersion: scenePlanVersion, Scenes: cloneScenes(scenes), AudioMix: cloneAudioMix(audioMix),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := ValidateDocument(doc); err != nil {
		return Document{}, err
	}
	return doc, nil
}

func ValidateDocument(doc Document) error {
	fields := map[string]string{}
	if doc.ID == uuid.Nil {
		fields["id"] = "required"
	}
	if doc.OwnerID == uuid.Nil {
		fields["owner_id"] = "required"
	}
	if doc.ProjectID == uuid.Nil {
		fields["project_id"] = "required"
	}
	if doc.Revision < 1 {
		fields["revision"] = "positive"
	}
	if doc.ScenePlanVersion < 1 {
		fields["scene_plan_version"] = "positive"
	}
	if len(doc.Scenes) == 0 {
		fields["scenes"] = "at_least_one"
	}

	seenIDs := map[uuid.UUID]struct{}{}
	for i, scene := range doc.Scenes {
		prefix := "scenes[" + itoa(i) + "]"
		if scene.ID == uuid.Nil {
			fields[prefix+".id"] = "required"
		}
		if _, exists := seenIDs[scene.ID]; exists {
			fields[prefix+".id"] = "duplicate"
		}
		seenIDs[scene.ID] = struct{}{}
		if scene.SceneKey == "" {
			fields[prefix+".scene_key"] = "required"
		}
		if scene.DurationMS <= 0 {
			fields[prefix+".duration_ms"] = "positive"
		}
		if scene.Narration != nil {
			if scene.Narration.AssetID == uuid.Nil || scene.Narration.BindingID == uuid.Nil || scene.Narration.LineageID == uuid.Nil {
				fields[prefix+".narration"] = "complete_lineage_required"
			}
			if scene.Narration.DurationMS <= 0 {
				fields[prefix+".narration.duration_ms"] = "positive"
			}
			if scene.DurationMS < scene.Narration.DurationMS {
				fields[prefix+".duration_ms"] = "must_cover_narration"
			}
		}
		if scene.Caption != nil {
			if scene.Caption.DocumentID == uuid.Nil || scene.Caption.Revision < 1 || scene.Caption.LineageID == uuid.Nil {
				fields[prefix+".caption"] = "complete_lineage_required"
			}
			if scene.Caption.LastEndMS < 0 {
				fields[prefix+".caption.last_end_ms"] = "non_negative"
			}
			if scene.DurationMS < scene.Caption.LastEndMS {
				fields[prefix+".duration_ms"] = "must_cover_captions"
			}
		}
		if scene.Visual != nil && (scene.Visual.AssetID == uuid.Nil || scene.Visual.BindingID == uuid.Nil) {
			fields[prefix+".visual"] = "complete_binding_required"
		}
		if scene.VisualTreatment.Fit != FitContain && scene.VisualTreatment.Fit != FitCover {
			fields[prefix+".visual_treatment.fit"] = "invalid"
		}
		if scene.VisualTreatment.Scale <= 0 {
			fields[prefix+".visual_treatment.scale"] = "positive"
		}
		if scene.VisualTreatment.Crop != nil && !validNormalizedCrop(*scene.VisualTreatment.Crop) {
			fields[prefix+".visual_treatment.crop"] = "normalized_rectangle_required"
		}
		if err := validateTransition(scene.TransitionOut, scene.DurationMS); err != nil {
			fields[prefix+".transition_out"] = err.Error()
		}
	}
	if doc.AudioMix != nil {
		if doc.AudioMix.DocumentID == uuid.Nil || doc.AudioMix.Revision < 1 || doc.AudioMix.MusicAssetID == uuid.Nil || doc.AudioMix.NarrationLineageID == uuid.Nil {
			fields["audio_mix"] = "complete_lineage_required"
		}
	}
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func Reorder(doc Document, sceneID uuid.UUID, to int, expectedRevision int, now time.Time) (Document, error) {
	if err := checkRevision(doc, expectedRevision); err != nil {
		return Document{}, err
	}
	if to < 0 || to >= len(doc.Scenes) {
		return Document{}, ValidationError{Fields: map[string]string{"to": "out_of_range"}}
	}
	from := sceneIndex(doc.Scenes, sceneID)
	if from < 0 {
		return Document{}, ErrSceneNotFound
	}
	if from == to {
		return bump(doc, now), nil
	}
	scenes := cloneScenes(doc.Scenes)
	moved := scenes[from]
	scenes = append(scenes[:from], scenes[from+1:]...)
	scenes = append(scenes, Scene{})
	copy(scenes[to+1:], scenes[to:])
	scenes[to] = moved
	doc.Scenes = scenes
	return bump(doc, now), nil
}

func Duplicate(doc Document, sceneID, newSceneID uuid.UUID, expectedRevision int, now time.Time) (Document, error) {
	if err := checkRevision(doc, expectedRevision); err != nil {
		return Document{}, err
	}
	if newSceneID == uuid.Nil {
		return Document{}, ValidationError{Fields: map[string]string{"new_scene_id": "required"}}
	}
	if sceneIndex(doc.Scenes, newSceneID) >= 0 {
		return Document{}, ValidationError{Fields: map[string]string{"new_scene_id": "duplicate"}}
	}
	idx := sceneIndex(doc.Scenes, sceneID)
	if idx < 0 {
		return Document{}, ErrSceneNotFound
	}
	scenes := cloneScenes(doc.Scenes)
	copyScene := scenes[idx]
	copyScene.ID = newSceneID
	scenes = append(scenes, Scene{})
	copy(scenes[idx+2:], scenes[idx+1:])
	scenes[idx+1] = copyScene
	doc.Scenes = scenes
	return bump(doc, now), nil
}

func Remove(doc Document, sceneID uuid.UUID, expectedRevision int, now time.Time) (Document, error) {
	if err := checkRevision(doc, expectedRevision); err != nil {
		return Document{}, err
	}
	if len(doc.Scenes) == 1 {
		return Document{}, ErrLastScene
	}
	idx := sceneIndex(doc.Scenes, sceneID)
	if idx < 0 {
		return Document{}, ErrSceneNotFound
	}
	scenes := cloneScenes(doc.Scenes)
	doc.Scenes = append(scenes[:idx], scenes[idx+1:]...)
	return bump(doc, now), nil
}

func StateForDependencies(states ...DependencyState) State {
	result := StateCurrent
	for _, dep := range states {
		if dep.State == StateBroken {
			return StateBroken
		}
		if dep.State == StateStale {
			result = StateStale
		}
	}
	return result
}

func NewSnapshot(doc Document, state State) (Snapshot, error) {
	if state != StateCurrent {
		return Snapshot{}, ErrSnapshotBlocked
	}
	if err := ValidateDocument(doc); err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{
		SchemaVersion:    SnapshotSchemaVersion,
		CompositionID:    doc.ID,
		Revision:         doc.Revision,
		ProjectID:        doc.ProjectID,
		ScenePlanVersion: doc.ScenePlanVersion,
		Scenes:           cloneScenes(doc.Scenes),
		AudioMix:         cloneAudioMix(doc.AudioMix),
	}
	digest, err := snapshotDigest(snapshot)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.Digest = digest
	return snapshot, nil
}

func snapshotDigest(snapshot Snapshot) (string, error) {
	canonical := snapshot
	canonical.Digest = ""
	payload, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func SortedSceneKeys(doc Document) []string {
	keys := make([]string, 0, len(doc.Scenes))
	for _, scene := range doc.Scenes {
		keys = append(keys, scene.SceneKey)
	}
	sort.Strings(keys)
	return keys
}

func checkRevision(doc Document, expected int) error {
	if expected != doc.Revision {
		return ErrConflict
	}
	return nil
}

func bump(doc Document, now time.Time) Document {
	doc.Revision++
	doc.UpdatedAt = now
	return doc
}

func sceneIndex(scenes []Scene, id uuid.UUID) int {
	for i := range scenes {
		if scenes[i].ID == id {
			return i
		}
	}
	return -1
}

func cloneScenes(in []Scene) []Scene {
	out := make([]Scene, len(in))
	copy(out, in)
	for i := range out {
		if in[i].Visual != nil {
			v := *in[i].Visual
			out[i].Visual = &v
		}
		if in[i].Narration != nil {
			n := *in[i].Narration
			out[i].Narration = &n
		}
		if in[i].Caption != nil {
			c := *in[i].Caption
			out[i].Caption = &c
		}
		if in[i].VisualTreatment.Crop != nil {
			c := *in[i].VisualTreatment.Crop
			out[i].VisualTreatment.Crop = &c
		}
	}
	return out
}

func cloneAudioMix(in *AudioMixRef) *AudioMixRef {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func validNormalizedCrop(c Crop) bool {
	return c.X >= 0 && c.Y >= 0 && c.Width > 0 && c.Height > 0 && c.X <= 1 && c.Y <= 1 && c.Width <= 1 && c.Height <= 1 && c.X+c.Width <= 1 && c.Y+c.Height <= 1
}

func validateTransition(t Transition, sceneDurationMS int64) error {
	switch t.Kind {
	case TransitionCut:
		if t.DurationMS != 0 {
			return errors.New("cut_requires_zero_duration")
		}
	case TransitionFade, TransitionCrossfade:
		if t.DurationMS <= 0 {
			return errors.New("positive_duration_required")
		}
		if t.DurationMS > sceneDurationMS {
			return errors.New("must_fit_scene_duration")
		}
	default:
		return errors.New("invalid_kind")
	}
	return nil
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
