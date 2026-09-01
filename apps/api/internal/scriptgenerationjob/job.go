package scriptgenerationjob

import (
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scriptgeneration"
)

const (
	JobKind       = "script_generation_v1"
	SchemaVersion = "script_generation_job_v1"
)

type Payload struct {
	SchemaVersion string                           `json:"schema_version"`
	ProviderID    string                           `json:"provider_id"`
	ModelID       string                           `json:"model_id"`
	Project       scriptgeneration.ProjectContext  `json:"project"`
	Proposal      scriptgeneration.ProposalContext `json:"proposal"`
}

type Result struct {
	ScriptVersion int `json:"script_version"`
}
