package proposalgeneration_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativebrief"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/proposalgeneration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/fake"
)

func TestGenerateReturnsValidatedCandidateWithSourceBriefRevision(t *testing.T) {
	engine := newTestEngine(t, validProviderJSON())

	candidate, err := engine.Generate(context.Background(), sampleRequest(project.ContentFormatShort, nil))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if candidate.SourceBriefRevision != 3 {
		t.Fatalf("source brief revision = %d, want 3", candidate.SourceBriefRevision)
	}
	if len(candidate.TitleOptions) != 1 || candidate.TitleOptions[0] != "Launch video" {
		t.Fatalf("title options = %#v", candidate.TitleOptions)
	}
	if len(candidate.Structure) != 1 || candidate.Structure[0].Key != "opening" {
		t.Fatalf("structure = %#v", candidate.Structure)
	}
}

func TestPromptIncludesCreatorConstraintsAndProjectContext(t *testing.T) {
	duration := 90
	textGen := fake.NewTextGenerator(validProviderJSON())
	engine := proposalgeneration.NewWithGenerator(textGen)

	req := sampleRequest(project.ContentFormatShort, &duration)
	req.Brief.MustInclude = []string{"Brand logo"}
	req.Brief.MustAvoid = []string{"Loud music"}

	if _, err := engine.Generate(context.Background(), req); err != nil {
		t.Fatalf("generate: %v", err)
	}

	prompt := textGen.Requests()[0].Messages[1].Content
	for _, want := range []string{
		"ai_proposal_v1",
		"Creator-provided facts",
		"Brand logo",
		"Loud music",
		"short",
		"target_duration_seconds: 90",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestGenerateRejectsMalformedJSON(t *testing.T) {
	engine := newTestEngine(t, `{not-json`)

	_, err := engine.Generate(context.Background(), sampleRequest(project.ContentFormatShort, nil))
	if !errors.Is(err, proposalgeneration.ErrGenerationInvalidOutput) {
		t.Fatalf("error = %v, want ErrGenerationInvalidOutput", err)
	}
}

func TestGenerateRejectsContractInvalidCandidate(t *testing.T) {
	invalid := validProviderPayload()
	invalid["title_options"] = []string{}
	body, err := json.Marshal(invalid)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	engine := newTestEngine(t, string(body))
	_, err = engine.Generate(context.Background(), sampleRequest(project.ContentFormatShort, nil))
	if !errors.Is(err, proposalgeneration.ErrGenerationInvalidOutput) {
		t.Fatalf("error = %v, want ErrGenerationInvalidOutput", err)
	}
}

func TestGenerateMapsProviderFailureToSafeError(t *testing.T) {
	engine := proposalgeneration.NewWithGenerator(proposalgeneration.FailingGenerator{Err: providers.NewExecutionError(errors.New("provider failed: api_key=super-secret-value"))})

	_, err := engine.Generate(context.Background(), sampleRequest(project.ContentFormatShort, nil))
	if !errors.Is(err, proposalgeneration.ErrGenerationProviderFailed) {
		t.Fatalf("error = %v, want ErrGenerationProviderFailed", err)
	}
	genErr, ok := err.(*proposalgeneration.Error)
	if !ok {
		t.Fatalf("expected proposalgeneration.Error, got %T", err)
	}
	if strings.Contains(genErr.PresentationMessage(), "super-secret-value") {
		t.Fatalf("presentation leaked secret: %q", genErr.PresentationMessage())
	}
}

func TestGeneratePropagatesContextCancellation(t *testing.T) {
	textGen := fake.NewTextGenerator(validProviderJSON()).WithDelay(200 * time.Millisecond)
	engine := proposalgeneration.NewWithGenerator(textGen)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := engine.Generate(ctx, sampleRequest(project.ContentFormatShort, nil))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestGenerateDoesNotMutateInput(t *testing.T) {
	engine := newTestEngine(t, validProviderJSON())
	req := sampleRequest(project.ContentFormatShort, nil)
	beforeBrief := cloneBrief(req.Brief)
	beforeProject := req.Project

	if _, err := engine.Generate(context.Background(), req); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !briefsEqual(req.Brief, beforeBrief) {
		t.Fatal("creative brief input was mutated")
	}
	if req.Project != beforeProject {
		t.Fatal("project input was mutated")
	}
}

func TestLongFormPromptPreservesLongFormatContext(t *testing.T) {
	duration := 3600
	textGen := fake.NewTextGenerator(validProviderJSON())
	engine := proposalgeneration.NewWithGenerator(textGen)

	req := sampleRequest(project.ContentFormatLong, &duration)
	if _, err := engine.Generate(context.Background(), req); err != nil {
		t.Fatalf("generate: %v", err)
	}

	prompt := textGen.Requests()[0].Messages[1].Content
	if !strings.Contains(prompt, "content_format: long") {
		t.Fatalf("prompt missing long-form context:\n%s", prompt)
	}
	if !strings.Contains(prompt, "target_duration_seconds: 3600") {
		t.Fatalf("prompt missing long duration context:\n%s", prompt)
	}
	if strings.Contains(strings.ToLower(prompt), "short-form only") {
		t.Fatalf("prompt incorrectly assumes short-form only:\n%s", prompt)
	}
}

func newTestEngine(t *testing.T, response string) *proposalgeneration.Engine {
	t.Helper()
	return proposalgeneration.NewWithGenerator(fake.NewTextGenerator(response))
}

func sampleRequest(format project.ContentFormat, duration *int) proposalgeneration.Request {
	return proposalgeneration.Request{
		Project: proposalgeneration.ProjectContext{
			ID:                    uuid.MustParse("11111111-1111-4111-8111-111111111111"),
			ContentFormat:         format,
			AspectRatio:           project.AspectRatio9x16,
			TargetDurationSeconds: duration,
			Locale:                project.LocaleVI,
		},
		Brief: proposalgeneration.BriefContext{
			Revision:            3,
			SourceText:          "Launch a product teaser",
			TargetAudience:      "Creators",
			Objective:           "Awareness",
			DesiredStyle:        "Bold",
			Tone:                "Energetic",
			DistributionTargets: []creativebrief.DistributionTarget{creativebrief.DistributionTargetYouTube},
			CallToAction:        "Subscribe",
			MustInclude:         []string{"Product name"},
			MustAvoid:           []string{"Negative tone"},
		},
		ProviderID: "synvideo-lab",
		ModelID:    "lab-text-v1",
	}
}

func cloneBrief(brief proposalgeneration.BriefContext) proposalgeneration.BriefContext {
	cloned := brief
	cloned.DistributionTargets = append([]creativebrief.DistributionTarget(nil), brief.DistributionTargets...)
	cloned.MustInclude = append([]string(nil), brief.MustInclude...)
	cloned.MustAvoid = append([]string(nil), brief.MustAvoid...)
	return cloned
}

func briefsEqual(left, right proposalgeneration.BriefContext) bool {
	leftClone := cloneBrief(left)
	rightClone := cloneBrief(right)
	leftClone.DistributionTargets = append([]creativebrief.DistributionTarget(nil), leftClone.DistributionTargets...)
	rightClone.DistributionTargets = append([]creativebrief.DistributionTarget(nil), rightClone.DistributionTargets...)
	return leftClone.Revision == rightClone.Revision &&
		leftClone.SourceText == rightClone.SourceText &&
		leftClone.TargetAudience == rightClone.TargetAudience &&
		leftClone.Objective == rightClone.Objective &&
		leftClone.DesiredStyle == rightClone.DesiredStyle &&
		leftClone.Tone == rightClone.Tone &&
		leftClone.CallToAction == rightClone.CallToAction &&
		strings.Join(mustIncludeKey(leftClone.MustInclude), "|") == strings.Join(mustIncludeKey(rightClone.MustInclude), "|") &&
		strings.Join(mustIncludeKey(leftClone.MustAvoid), "|") == strings.Join(mustIncludeKey(rightClone.MustAvoid), "|") &&
		strings.Join(distributionKey(leftClone.DistributionTargets), "|") == strings.Join(distributionKey(rightClone.DistributionTargets), "|")
}

func mustIncludeKey(values []string) []string {
	return append([]string(nil), values...)
}

func distributionKey(values []creativebrief.DistributionTarget) []string {
	items := make([]string, len(values))
	for i, value := range values {
		items[i] = string(value)
	}
	return items
}

func validProviderJSON() string {
	body, err := json.Marshal(validProviderPayload())
	if err != nil {
		panic(err)
	}
	return string(body)
}

func validProviderPayload() map[string]any {
	return map[string]any{
		"title_options":     []string{"Launch video"},
		"hook_options":      []string{"What if your next video wrote itself?"},
		"audience_summary":  "Creators who want faster ideation",
		"objective_summary": "Increase awareness before launch",
		"narrative_angle":   "Show the product as a creative partner",
		"structure": []map[string]string{
			{"key": "opening", "title": "Opening", "purpose": "Establish the creator pain point"},
		},
	}
}
