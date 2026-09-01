package sceneplangeneration

import (
	"bytes"
	"context"
	"fmt"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

type textGenerator interface {
	GenerateText(context.Context, providers.TextGenerationRequest) (providers.TextGenerationResponse, error)
}

type Engine struct {
	resolve func(providerID, modelID string) (textGenerator, error)
}

type FailingGenerator struct{ Err error }

func (g FailingGenerator) GenerateText(ctx context.Context, _ providers.TextGenerationRequest) (providers.TextGenerationResponse, error) {
	if err := ctx.Err(); err != nil {
		return providers.TextGenerationResponse{}, err
	}
	return providers.TextGenerationResponse{}, g.Err
}

func New(registry *providers.Registry) *Engine {
	return &Engine{resolve: func(providerID, modelID string) (textGenerator, error) {
		generator, _, err := registry.ResolveTextGenerator(providers.ProviderID(providerID), providers.ModelID(modelID))
		return generator, err
	}}
}

func NewWithGenerator(generator textGenerator) *Engine {
	return &Engine{resolve: func(_, _ string) (textGenerator, error) { return generator, nil }}
}

func (e *Engine) Generate(ctx context.Context, req Request) (Candidate, error) {
	if err := ctx.Err(); err != nil {
		return Candidate{}, err
	}

	generator, err := e.resolve(req.ProviderID, req.ModelID)
	if err != nil {
		return Candidate{}, newProviderUnavailableError(fmt.Errorf("resolve provider: %w", err))
	}

	projectInput := cloneProjectContext(req.Project)
	scriptInput := cloneScriptContext(req.Script)
	proposalInput := cloneProposalContext(req.Proposal)
	if proposalInput.Version != scriptInput.SourceProposalVersion {
		return Candidate{}, newInvalidOutputError(fmt.Errorf("proposal version does not match script source"))
	}

	response, err := generator.GenerateText(ctx, providers.TextGenerationRequest{
		ProviderID: providers.ProviderID(req.ProviderID),
		ModelID:    providers.ModelID(req.ModelID),
		Messages: []providers.TextMessage{
			{Role: "system", Content: "Return only JSON matching the requested scene plan schema."},
			{Role: "user", Content: buildPrompt(projectInput, scriptInput, proposalInput)},
		},
	})
	if err != nil {
		return Candidate{}, mapProviderError(err)
	}

	candidate, err := parseCandidate(response.Text, scriptInput, proposalInput.Version)
	if err != nil {
		return Candidate{}, err
	}
	return candidate, nil
}

func cloneProjectContext(input ProjectContext) ProjectContext {
	cloned := input
	if input.TargetDurationSeconds != nil {
		value := *input.TargetDurationSeconds
		cloned.TargetDurationSeconds = &value
	}
	return cloned
}

func cloneScriptContext(input ScriptContext) ScriptContext {
	cloned := input
	if input.EstimatedDurationSeconds != nil {
		value := *input.EstimatedDurationSeconds
		cloned.EstimatedDurationSeconds = &value
	}
	cloned.Sections = append([]ScriptSection(nil), input.Sections...)
	return cloned
}

func cloneProposalContext(input ProposalContext) ProposalContext {
	cloned := input
	cloned.Warnings = append([]string(nil), input.Warnings...)
	cloned.ResearchGaps = append([]string(nil), input.ResearchGaps...)
	return cloned
}

func buildPrompt(projectCtx ProjectContext, scriptCtx ScriptContext, proposalCtx ProposalContext) string {
	var buffer bytes.Buffer
	fmt.Fprintf(&buffer, "template_version: %s\n\n", PromptTemplateVersion)

	buffer.WriteString("Approved Script narration (must be preserved exactly; segment it but do not rewrite, add, omit, or paraphrase):\n")
	for _, section := range scriptCtx.Sections {
		fmt.Fprintf(&buffer, "- section_key: %s\n  heading: %s\n  approved_narration: %s\n", section.Key, section.Heading, section.Body)
	}
	if scriptCtx.EstimatedDurationSeconds != nil {
		fmt.Fprintf(&buffer, "script_estimated_duration_seconds: %d\n", *scriptCtx.EstimatedDurationSeconds)
	} else {
		buffer.WriteString("script_estimated_duration_seconds: null\n")
	}
	fmt.Fprintf(&buffer, "script_notes: %s\n", scriptCtx.Notes)

	buffer.WriteString("\nProposal production direction (planning constraints only):\n")
	fmt.Fprintf(&buffer, "proposal_version: %d\n", proposalCtx.Version)
	fmt.Fprintf(&buffer, "visual_direction: %s\n", proposalCtx.VisualDirection)
	fmt.Fprintf(&buffer, "voice_direction: %s\n", proposalCtx.VoiceDirection)
	fmt.Fprintf(&buffer, "music_direction: %s\n", proposalCtx.MusicDirection)
	fmt.Fprintf(&buffer, "caption_direction: %s\n", proposalCtx.CaptionDirection)
	writeListSection(&buffer, "warnings", proposalCtx.Warnings)
	writeListSection(&buffer, "research_gaps", proposalCtx.ResearchGaps)
	buffer.WriteString("Do not invent unsupported factual visuals when warnings or research gaps apply.\n")

	buffer.WriteString("\nProject context (preserve all declared production intent):\n")
	fmt.Fprintf(&buffer, "content_format: %s\n", projectCtx.ContentFormat)
	fmt.Fprintf(&buffer, "aspect_ratio: %s\n", projectCtx.AspectRatio)
	if projectCtx.TargetDurationSeconds != nil {
		fmt.Fprintf(&buffer, "target_duration_seconds: %d\n", *projectCtx.TargetDurationSeconds)
	} else {
		buffer.WriteString("target_duration_seconds: null\n")
	}
	fmt.Fprintf(&buffer, "locale: %s\n", projectCtx.Locale)
	writeFormatGuidance(&buffer, projectCtx.ContentFormat)

	buffer.WriteString("\nScene plan requirements:\n")
	buffer.WriteString("Return strict JSON containing exactly {\"scenes\": [{\"key\": \"slug-key\", \"script_section_key\": \"approved-section-key\", \"narration\": \"approved narration segment\", \"visual_instruction\": \"production instruction\", \"planned_source_type\": \"stock|upload|creator_media|generated_image|generated_video\", \"expected_duration_seconds\": 1, \"caption_intent\": \"optional\", \"transition_notes\": \"optional\"}]}.\n")
	buffer.WriteString("Every Script section must be represented in original order and contiguous. Use as many scenes as the approved Script requires, including long-form scripts; do not force short-form pacing. Request planning instructions only, not actual media generation.\n")
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
	if format == project.ContentFormatLong {
		buffer.WriteString("Use long-form scene structure and pacing; do not collapse this into short-form-only assumptions.\n")
		return
	}
	buffer.WriteString("Respect the declared content format without forcing short-form-only structure.\n")
}
