package scenevideojob

import (
	"errors"

	"github.com/google/uuid"
)

const (
	JobKind                      = "scene_video_generation_v1"
	SchemaVersion                = "scene_video_generation_job_v1"
	MaxGeneratedVideoBytes int64 = 2 << 30
)

var ErrCheckpointNotFound = errors.New("scene video operation checkpoint not found")

type OperationState string

const (
	OperationStateSubmitted OperationState = "submitted"
	OperationStateAmbiguous OperationState = "ambiguous"
)

type OperationCheckpoint struct {
	JobID               uuid.UUID
	ProjectID           uuid.UUID
	ExternalOperationID string
	State               OperationState
}

type Payload struct {
	SchemaVersion       string `json:"schema_version"`
	ProviderID          string `json:"provider_id"`
	ModelID             string `json:"model_id"`
	ScenePlanVersion    int    `json:"scene_plan_version"`
	SceneKey            string `json:"scene_key"`
	Prompt              string `json:"prompt"`
	AspectRatio         string `json:"aspect_ratio,omitempty"`
	DurationSeconds     *int   `json:"duration_seconds,omitempty"`
	AssignPrimaryVisual bool   `json:"assign_primary_visual"`
}

type Result struct {
	MediaAssetID          uuid.UUID `json:"media_asset_id"`
	ExternalOperationID   string    `json:"external_operation_id"`
	AssignedPrimaryVisual bool      `json:"assigned_primary_visual"`
}
