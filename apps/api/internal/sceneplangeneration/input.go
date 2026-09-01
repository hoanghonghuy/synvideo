package sceneplangeneration

import (
	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

const PromptTemplateVersion = "scene_plan_v1"

type ProjectContext struct {
	ID                    uuid.UUID
	ContentFormat         project.ContentFormat
	AspectRatio           project.AspectRatio
	TargetDurationSeconds *int
	Locale                project.Locale
}

type ScriptSection struct {
	Key     string
	Heading string
	Body    string
}

type ScriptContext struct {
	Version                  int
	SourceProposalVersion    int
	Sections                 []ScriptSection
	EstimatedDurationSeconds *int
	Notes                    string
}

type ProposalContext struct {
	Version          int
	VisualDirection  string
	VoiceDirection   string
	MusicDirection   string
	CaptionDirection string
	Warnings         []string
	ResearchGaps     []string
}

type Request struct {
	Project    ProjectContext
	Script     ScriptContext
	Proposal   ProposalContext
	ProviderID string
	ModelID    string
}
