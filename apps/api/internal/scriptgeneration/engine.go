package scriptgeneration

import (
	"bytes"
	"context"
	"fmt"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

type textGenerator interface {
	GenerateText(ctx context.Context, req providers.TextGenerationRequest) (providers.TextGenerationResponse, error)
}

// Engine transforms Project + approved Proposal context into a validated Script candidate.
type Engine struct {
	resolve func(providerID, modelID string) (textGenerator, error)
}

// FailingGenerator is a test double that always returns a provider error.
type FailingGenerator struct {
	Err error
}

func (g FailingGenerator) GenerateText(ctx context.Context, _ providers.TextGenerationRequest) (providers.TextGenerationResponse, error) {
	if err := ctx.Err(); err != nil {
		return providers.TextGenerationResponse{}, err
	}
	return providers.TextGenerationResponse{}, g.Err
}

// New creates an Engine backed by a provider registry.
func New(registry *providers.Registry) *Engine {
	return &Engine{
		resolve: func(providerID, modelID string) (textGenerator, error) {
			generator, _, err := registry.ResolveTextGenerator(providers.ProviderID(providerID), providers.ModelID(modelID))
			return generator, err
		},
	}
}

// NewWithGenerator creates an Engine using an explicit textGenerator instance.
func NewWithGenerator(generator textGenerator) *Engine {
	return &Engine{
		resolve: func(_, _ string) (textGenerator, error) {
			return generator, nil
		},
	}
}

// Generate executes Script candidate generation from Project and approved Proposal inputs.
func (e *Engine) Generate(ctx context.Context, req Request) (Candidate, error) {
	if err := ctx.Err(); err != nil {
		return Candidate{}, err
	}

	generator, err := e.resolve(req.ProviderID, req.ModelID)
	if err != nil {
		return Candidate{}, providerResolutionError(req.ProviderID, req.ModelID, err)
	}

	projectInput := cloneProjectContext(req.Project)
	proposalInput := cloneProposalContext(req.Proposal)

	prompt := buildPrompt(projectInput, proposalInput)
	response, err := generator.GenerateText(ctx, providers.TextGenerationRequest{
		ProviderID: providers.ProviderID(req.ProviderID),
		ModelID:    providers.ModelID(req.ModelID),
		Messages: []providers.TextMessage{
			{Role: "system", Content: "Return only JSON matching the requested script schema."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return Candidate{}, mapProviderError(err)
	}

	candidate, err := parseCandidate(response.Text, proposalInput.Version)
	if err != nil {
		return Candidate{}, err
	}
	return candidate, nil
}

func providerResolutionError(providerID, modelID string, err error) error {
	return newProviderUnavailableError(fmt.Errorf("resolve %q/%q: %w", providerID, modelID, err))
}

func cloneProjectContext(input ProjectContext) ProjectContext {
	cloned := input
	if input.TargetDurationSeconds != nil {
		value := *input.TargetDurationSeconds
		cloned.TargetDurationSeconds = &value
	}
	return cloned
}

func cloneProposalContext(input ProposalContext) ProposalContext {
	cloned := input
	if input.EstimatedDurationSeconds != nil {
		value := *input.EstimatedDurationSeconds
		cloned.EstimatedDurationSeconds = &value
	}
	cloned.TitleOptions = append([]string(nil), input.TitleOptions...)
	cloned.HookOptions = append([]string(nil), input.HookOptions...)
	cloned.Structure = append([]ProposalStructureItem(nil), input.Structure...)
	cloned.ResearchGaps = append([]string(nil), input.ResearchGaps...)
	cloned.Warnings = append([]string(nil), input.Warnings...)
	return cloned
}

func buildPrompt(projectCtx ProjectContext, proposalCtx ProposalContext) string {
	var buffer bytes.Buffer
	fmt.Fprintf(&buffer, "template_version: %s\n\n", PromptTemplateVersion)

	buffer.WriteString("Approved Proposal editorial direction (creator-approved baseline for script narration):\n")
	writeListSection(&buffer, "title_options", proposalCtx.TitleOptions)
	writeListSection(&buffer, "hook_options", proposalCtx.HookOptions)
	fmt.Fprintf(&buffer, "audience_summary: %s\n", proposalCtx.AudienceSummary)
	fmt.Fprintf(&buffer, "objective_summary: %s\n", proposalCtx.ObjectiveSummary)
	fmt.Fprintf(&buffer, "narrative_angle: %s\n", proposalCtx.NarrativeAngle)
	if proposalCtx.EstimatedDurationSeconds != nil {
		fmt.Fprintf(&buffer, "estimated_duration_seconds: %d\n", *proposalCtx.EstimatedDurationSeconds)
	} else {
		buffer.WriteString("estimated_duration_seconds: null\n")
	}
	fmt.Fprintf(&buffer, "format_rationale: %s\n", proposalCtx.FormatRationale)

	buffer.WriteString("\nProposal structure guidance (use as section flow foundation):\n")
	if len(proposalCtx.Structure) == 0 {
		buffer.WriteString("structure: []\n")
	} else {
		for _, item := range proposalCtx.Structure {
			fmt.Fprintf(&buffer, "- key: %s\n  title: %s\n  purpose: %s\n", item.Key, item.Title, item.Purpose)
		}
	}

	buffer.WriteString("\nProduction directions:\n")
	fmt.Fprintf(&buffer, "visual_direction: %s\n", proposalCtx.VisualDirection)
	fmt.Fprintf(&buffer, "voice_direction: %s\n", proposalCtx.VoiceDirection)
	fmt.Fprintf(&buffer, "music_direction: %s\n", proposalCtx.MusicDirection)
	fmt.Fprintf(&buffer, "caption_direction: %s\n", proposalCtx.CaptionDirection)
	fmt.Fprintf(&buffer, "call_to_action: %s\n", proposalCtx.CallToAction)

	buffer.WriteString("\nStrict factual constraints and warnings (do NOT silently invent factual answers):\n")
	writeListSection(&buffer, "research_gaps", proposalCtx.ResearchGaps)
	writeListSection(&buffer, "warnings", proposalCtx.Warnings)

	buffer.WriteString("\nProject context (preserve format, duration, and locale intent):\n")
	fmt.Fprintf(&buffer, "content_format: %s\n", projectCtx.ContentFormat)
	fmt.Fprintf(&buffer, "aspect_ratio: %s\n", projectCtx.AspectRatio)
	if projectCtx.TargetDurationSeconds != nil {
		fmt.Fprintf(&buffer, "target_duration_seconds: %d\n", *projectCtx.TargetDurationSeconds)
	} else {
		buffer.WriteString("target_duration_seconds: null\n")
	}
	fmt.Fprintf(&buffer, "locale: %s\n", projectCtx.Locale)
	writeFormatGuidance(&buffer, projectCtx.ContentFormat)

	buffer.WriteString("\nScript generation requirements:\n")
	buffer.WriteString("Write complete, coherent script narration/text organized into ordered sections.\n")
	buffer.WriteString("Respond with JSON containing exactly these editable script fields:\n")
	buffer.WriteString("- sections: array of objects with {\"key\": \"slug-key\", \"heading\": \"Optional section heading\", \"body\": \"Full narration text for this section\"}\n")
	buffer.WriteString("- estimated_duration_seconds: optional integer (1..43200)\n")
	buffer.WriteString("- notes: optional string for production or editorial guidance\n")
	buffer.WriteString("Do not include server-controlled fields, provider metadata, or invented facts violating research gaps.\n")

	return buffer.String()
}

func writeListSection(buffer *bytes.Buffer, label string, values []string) {
	if len(values) == 0 {
		fmt.Fprintf(buffer, "%s: []\n", label)
		return
	}
	for _, value := range values {
		fmt.Fprintf(buffer, "%s: %s\n", label, value)
	}
}

func writeFormatGuidance(buffer *bytes.Buffer, format project.ContentFormat) {
	switch format {
	case project.ContentFormatLong:
		buffer.WriteString("Use long-form editorial structure depth; do not collapse this into short-form-only assumptions.\n")
	case project.ContentFormatShort:
		buffer.WriteString("Optimize recommendations for short-form pacing while preserving the declared duration intent.\n")
	default:
		buffer.WriteString("Respect the declared content format without forcing short-form-only structure.\n")
	}
}
