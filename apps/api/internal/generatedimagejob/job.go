package generatedimagejob

import "github.com/google/uuid"

const (
	JobKind                      = "generated_image_acquisition_v1"
	SchemaVersion                = "generated_image_acquisition_job_v1"
	MaxGeneratedImageBytes int64 = 20 << 20
)

type Payload struct {
	SchemaVersion       string `json:"schema_version"`
	ProviderID          string `json:"provider_id"`
	ModelID             string `json:"model_id"`
	ScenePlanVersion    int    `json:"scene_plan_version"`
	SceneKey            string `json:"scene_key"`
	Prompt              string `json:"prompt"`
	AspectRatio         string `json:"aspect_ratio,omitempty"`
	AssignPrimaryVisual bool   `json:"assign_primary_visual"`
}

type Result struct {
	MediaAssetID          uuid.UUID `json:"media_asset_id"`
	AssignedPrimaryVisual bool      `json:"assigned_primary_visual"`
}
