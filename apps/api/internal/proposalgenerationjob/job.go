package proposalgenerationjob

import (
	"github.com/hoanghonghuy/synvideo/apps/api/internal/proposalgeneration"
)

const (
	JobKind       = "creative_proposal_generation_v1"
	SchemaVersion = "ai_proposal_generation_job_v1"
)

type Payload struct {
	SchemaVersion string                            `json:"schema_version"`
	ProviderID    string                            `json:"provider_id"`
	ModelID       string                            `json:"model_id"`
	Project       proposalgeneration.ProjectContext `json:"project"`
	Brief         proposalgeneration.BriefContext   `json:"brief"`
}

type Result struct {
	ProposalVersion int `json:"proposal_version"`
}
