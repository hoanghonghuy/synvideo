package proposalgeneration

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativebrief"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

type textGenerator interface {
	GenerateText(ctx context.Context, req providers.TextGenerationRequest) (providers.TextGenerationResponse, error)
}

// Engine transforms Project + Creative Brief context into a validated Proposal candidate.
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

func New(registry *providers.Registry) *Engine {
	return &Engine{
		resolve: func(providerID, modelID string) (textGenerator, error) {
			generator, _, err := registry.ResolveTextGenerator(providers.ProviderID(providerID), providers.ModelID(modelID))
			return generator, err
		},
	}
}

func NewWithGenerator(generator textGenerator) *Engine {
	return &Engine{
		resolve: func(_, _ string) (textGenerator, error) {
			return generator, nil
		},
	}
}

func (e *Engine) Generate(ctx context.Context, req Request) (Candidate, error) {
	if err := ctx.Err(); err != nil {
		return Candidate{}, err
	}

	generator, err := e.resolve(req.ProviderID, req.ModelID)
	if err != nil {
		return Candidate{}, providerResolutionError(req.ProviderID, req.ModelID, err)
	}

	projectInput := cloneProjectContext(req.Project)
	briefInput := cloneBriefContext(req.Brief)

	prompt := buildPrompt(projectInput, briefInput)
	response, err := generator.GenerateText(ctx, providers.TextGenerationRequest{
		ProviderID: providers.ProviderID(req.ProviderID),
		ModelID:    providers.ModelID(req.ModelID),
		Messages: []providers.TextMessage{
			{Role: "system", Content: "Return only JSON matching the requested proposal schema."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return Candidate{}, mapProviderError(err)
	}

	candidate, err := parseCandidate(response.Text, briefInput.Revision)
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

func cloneBriefContext(input BriefContext) BriefContext {
	cloned := input
	cloned.DistributionTargets = append([]creativebrief.DistributionTarget(nil), input.DistributionTargets...)
	cloned.MustInclude = append([]string(nil), input.MustInclude...)
	cloned.MustAvoid = append([]string(nil), input.MustAvoid...)
	return cloned
}

func formatDistributionTargets(values []creativebrief.DistributionTarget) string {
	if len(values) == 0 {
		return ""
	}
	items := make([]string, len(values))
	for i, value := range values {
		items[i] = string(value)
	}
	return strings.Join(items, ", ")
}

func buildPrompt(projectCtx ProjectContext, briefCtx BriefContext) string {
	var buffer bytes.Buffer
	fmt.Fprintf(&buffer, "template_version: %s\n\n", PromptTemplateVersion)

	buffer.WriteString("Creator-provided facts (do not invent or rewrite as recommendations):\n")
	fmt.Fprintf(&buffer, "source_text: %s\n", briefCtx.SourceText)
	fmt.Fprintf(&buffer, "target_audience: %s\n", briefCtx.TargetAudience)
	fmt.Fprintf(&buffer, "objective: %s\n", briefCtx.Objective)
	fmt.Fprintf(&buffer, "desired_style: %s\n", briefCtx.DesiredStyle)
	fmt.Fprintf(&buffer, "tone: %s\n", briefCtx.Tone)
	fmt.Fprintf(&buffer, "distribution_targets: %s\n", formatDistributionTargets(briefCtx.DistributionTargets))
	fmt.Fprintf(&buffer, "call_to_action: %s\n", briefCtx.CallToAction)
	writeListSection(&buffer, "must_include", briefCtx.MustInclude)
	writeListSection(&buffer, "must_avoid", briefCtx.MustAvoid)

	buffer.WriteString("\nProject context (preserve format and duration intent):\n")
	fmt.Fprintf(&buffer, "content_format: %s\n", projectCtx.ContentFormat)
	fmt.Fprintf(&buffer, "aspect_ratio: %s\n", projectCtx.AspectRatio)
	if projectCtx.TargetDurationSeconds != nil {
		fmt.Fprintf(&buffer, "target_duration_seconds: %d\n", *projectCtx.TargetDurationSeconds)
	} else {
		buffer.WriteString("target_duration_seconds: null\n")
	}
	fmt.Fprintf(&buffer, "locale: %s\n", projectCtx.Locale)
	writeFormatGuidance(&buffer, projectCtx.ContentFormat)

	buffer.WriteString("\nAI recommendations requested:\n")
	buffer.WriteString("Produce editorial recommendations only. Respond with JSON containing exactly these editable proposal fields:\n")
	buffer.WriteString("title_options, hook_options, audience_summary, objective_summary, narrative_angle, estimated_duration_seconds, format_rationale, structure, visual_direction, voice_direction, music_direction, caption_direction, call_to_action, research_gaps, warnings.\n")
	buffer.WriteString("Do not include server-controlled fields, provider metadata, or creator facts copied verbatim as recommendations.\n")

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
