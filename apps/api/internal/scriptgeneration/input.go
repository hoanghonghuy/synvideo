package scriptgeneration

import (
	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

const PromptTemplateVersion = "script_v1"

// ProjectContext carries the Project fields required for script generation input.
type ProjectContext struct {
	ID                    uuid.UUID
	ContentFormat         project.ContentFormat
	AspectRatio           project.AspectRatio
	TargetDurationSeconds *int
	Locale                project.Locale
}

// ProposalStructureItem carries one ordered structure entry from the approved Proposal.
type ProposalStructureItem struct {
	Key     string
	Title   string
	Purpose string
}

// ProposalContext carries creator-approved editorial Proposal fields for script generation input.
type ProposalContext struct {
	Version                  int
	TitleOptions             []string
	HookOptions              []string
	AudienceSummary          string
	ObjectiveSummary         string
	NarrativeAngle           string
	EstimatedDurationSeconds *int
	FormatRationale          string
	Structure                []ProposalStructureItem
	VisualDirection          string
	VoiceDirection           string
	MusicDirection           string
	CaptionDirection         string
	CallToAction             string
	ResearchGaps             []string
	Warnings                 []string
}

// Request is the generation-engine input defined by SCRIPT_GENERATION_V1.
type Request struct {
	Project    ProjectContext
	Proposal   ProposalContext
	ProviderID string
	ModelID    string
}
