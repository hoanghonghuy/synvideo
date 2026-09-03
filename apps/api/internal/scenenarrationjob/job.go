package scenenarrationjob

import "github.com/google/uuid"

const (
	JobKind                      = "scene_narration_generation_v1"
	SchemaVersion                = "scene_narration_generation_job_v1"
	MaxGeneratedAudioBytes int64 = 50 << 20
	DefaultMaxChunkRunes         = 4000
)

type Payload struct {
	SchemaVersion    string `json:"schema_version"`
	ProviderID       string `json:"provider_id"`
	ModelID          string `json:"model_id"`
	VoiceID          string `json:"voice_id"`
	Format           string `json:"format,omitempty"`
	ScenePlanVersion int    `json:"scene_plan_version"`
	SceneKey         string `json:"scene_key"`
	NarrationText    string `json:"narration_text"`
	Locale           string `json:"locale,omitempty"`
	AssignCurrent    bool   `json:"assign_current"`
}

type Result struct {
	MediaAssetID      uuid.UUID `json:"media_asset_id"`
	DurationSeconds   float64   `json:"duration_seconds"`
	AssignedNarration bool      `json:"assigned_narration"`
}
