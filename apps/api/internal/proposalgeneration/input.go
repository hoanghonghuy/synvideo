package proposalgeneration

import (
	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativebrief"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

const PromptTemplateVersion = "ai_proposal_v1"

// ProjectContext carries the Project fields required for generation input.
type ProjectContext struct {
	ID                    uuid.UUID
	ContentFormat         project.ContentFormat
	AspectRatio           project.AspectRatio
	TargetDurationSeconds *int
	Locale                project.Locale
}

// BriefContext carries creator-authored Creative Brief fields for generation input.
type BriefContext struct {
	Revision            int
	SourceText          string
	TargetAudience      string
	Objective           string
	DesiredStyle        string
	Tone                string
	DistributionTargets []creativebrief.DistributionTarget
	CallToAction        string
	MustInclude         []string
	MustAvoid           []string
}

// Request is the generation-engine input defined by AI_PROPOSAL_GENERATION_V1.
type Request struct {
	Project    ProjectContext
	Brief      BriefContext
	ProviderID string
	ModelID    string
}
