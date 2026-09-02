package sceneplangenerationjob

import (
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplangeneration"
)

const (
	JobKind       = "scene_plan_generation_v1"
	SchemaVersion = "scene_plan_generation_job_v1"
)

type Payload struct {
	SchemaVersion string                              `json:"schema_version"`
	ProviderID    string                              `json:"provider_id"`
	ModelID       string                              `json:"model_id"`
	Project       sceneplangeneration.ProjectContext  `json:"project"`
	Script        sceneplangeneration.ScriptContext   `json:"script"`
	Proposal      sceneplangeneration.ProposalContext `json:"proposal"`
}

type Result struct {
	ScenePlanVersion int `json:"scene_plan_version"`
}
