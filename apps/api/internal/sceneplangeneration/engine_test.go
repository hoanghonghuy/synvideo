package sceneplangeneration_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplangeneration"
)

type capturingGenerator struct {
	request  providers.TextGenerationRequest
	response providers.TextGenerationResponse
}

func (g *capturingGenerator) GenerateText(_ context.Context, request providers.TextGenerationRequest) (providers.TextGenerationResponse, error) {
	g.request = request
	return g.response, nil
}

func validRequest() sceneplangeneration.Request {
	duration := 600
	return sceneplangeneration.Request{
		Project: sceneplangeneration.ProjectContext{
			ID:                    uuid.MustParse("11111111-1111-4111-8111-111111111111"),
			ContentFormat:         project.ContentFormatLong,
			AspectRatio:           project.AspectRatio16x9,
			TargetDurationSeconds: &duration,
			Locale:                project.LocaleEN,
		},
		Script: sceneplangeneration.ScriptContext{
			Version:               7,
			SourceProposalVersion: 4,
			Sections: []sceneplangeneration.ScriptSection{
				{Key: "intro", Heading: "Introduction", Body: "First approved narration."},
				{Key: "proof", Heading: "Proof", Body: "Second approved narration."},
			},
			EstimatedDurationSeconds: &duration,
		},
		Proposal: sceneplangeneration.ProposalContext{
			Version:          4,
			VisualDirection:  "Documentary visuals with natural light",
			VoiceDirection:   "Calm, precise delivery",
			MusicDirection:   "Sparse ambient bed",
			CaptionDirection: "High contrast captions",
			Warnings:         []string{"Do not claim unverified results."},
			ResearchGaps:     []string{"Latest benchmark still needs verification."},
		},
		ProviderID: "fake-provider",
		ModelID:    "fake-text-model",
	}
}

const validScenePlanJSON = `{
  "scenes": [
    {
      "key": "intro-1",
      "script_section_key": "intro",
      "narration": "First approved narration.",
      "visual_instruction": "Show an establishing shot.",
      "planned_source_type": "stock",
      "expected_duration_seconds": 8,
      "caption_intent": "Emphasize the opening.",
      "transition_notes": "Cut on the beat."
    },
    {
      "key": "proof-1",
      "script_section_key": "proof",
      "narration": "Second approved narration.",
      "visual_instruction": "Show the evidence clearly.",
      "planned_source_type": "creator_media",
      "expected_duration_seconds": 12
    }
  ]
}`

func TestEngineGenerateValidCandidate(t *testing.T) {
	generator := &capturingGenerator{response: providers.TextGenerationResponse{Text: validScenePlanJSON}}
	candidate, err := sceneplangeneration.NewWithGenerator(generator).Generate(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if candidate.SourceScriptVersion != 7 || candidate.SourceProposalVersion != 4 {
		t.Fatalf("unexpected source versions: %+v", candidate)
	}
	if len(candidate.Scenes) != 2 || candidate.Scenes[0].Narration != "First approved narration." {
		t.Fatalf("unexpected candidate: %+v", candidate)
	}
	if candidate.Scenes[1].PlannedSourceType != sceneplangeneration.SourceTypeCreatorMedia {
		t.Fatalf("unexpected source type: %+v", candidate.Scenes[1])
	}
}

func TestEngineGeneratePromptContainsApprovedScriptAndProductionContext(t *testing.T) {
	generator := &capturingGenerator{response: providers.TextGenerationResponse{Text: validScenePlanJSON}}
	if _, err := sceneplangeneration.NewWithGenerator(generator).Generate(context.Background(), validRequest()); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(generator.request.Messages) < 2 {
		t.Fatalf("expected system and user messages, got %d", len(generator.request.Messages))
	}
	prompt := generator.request.Messages[1].Content
	for _, snippet := range []string{
		"template_version: scene_plan_v1",
		"approved narration",
		"First approved narration.",
		"Second approved narration.",
		"content_format: long",
		"aspect_ratio: 16:9",
		"target_duration_seconds: 600",
		"locale: en",
		"Documentary visuals with natural light",
		"Calm, precise delivery",
		"Sparse ambient bed",
		"High contrast captions",
		"Latest benchmark still needs verification.",
		"Do not claim unverified results.",
		"long-form",
	} {
		if !strings.Contains(prompt, snippet) {
			t.Errorf("prompt missing %q", snippet)
		}
	}
}

func generateWithJSON(raw string, request sceneplangeneration.Request) error {
	generator := &capturingGenerator{response: providers.TextGenerationResponse{Text: raw}}
	_, err := sceneplangeneration.NewWithGenerator(generator).Generate(context.Background(), request)
	return err
}

func requireInvalidOutput(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected GENERATION_INVALID_OUTPUT, got nil")
	}
	var generationErr *sceneplangeneration.Error
	if !errors.As(err, &generationErr) || generationErr.Code != sceneplangeneration.CodeInvalidOutput {
		t.Fatalf("expected GENERATION_INVALID_OUTPUT, got %T: %v", err, err)
	}
}

func sceneJSON(section, key, narration string) string {
	return fmt.Sprintf(`{"scenes":[{"key":%q,"script_section_key":%q,"narration":%q,"visual_instruction":"Show a relevant visual.","planned_source_type":"stock","expected_duration_seconds":5}]}`, key, section, narration)
}

func TestEngineGenerateRejectsMalformedTrailingAndUnknownJSON(t *testing.T) {
	cases := []string{
		"",
		"   \n\t",
		`{"scenes":[`,
		validScenePlanJSON + ` {"unexpected":true}`,
		`{"scenes": [{"key":"intro-1","script_section_key":"intro","narration":"First approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5,"extra":true}]}`,
		`{"scenes": [{"key":"intro-1","script_section_key":"intro","narration":"First approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5}],"server_version":2}`,
	}
	for index, raw := range cases {
		t.Run(fmt.Sprintf("case_%d", index), func(t *testing.T) {
			requireInvalidOutput(t, generateWithJSON(raw, validRequest()))
		})
	}
}

func TestEngineGenerateRejectsSceneValidationBoundaries(t *testing.T) {
	base := func(sceneFields string) string {
		return `{"scenes":[{"key":"intro-1","script_section_key":"intro","narration":"First approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5,` + sceneFields + `}]}`
	}
	cases := []string{
		`{"scenes":[]}`,
		base(`"key":"Bad Key"`),
		base(`"key":"` + strings.Repeat("a", 65) + `"`),
		base(`"planned_source_type":"unknown"`),
		base(`"expected_duration_seconds":0`),
		base(`"expected_duration_seconds":3601`),
		base(`"narration":"   "`),
		base(`"visual_instruction":"   "`),
		base(`"caption_intent":"` + strings.Repeat("x", 3001) + `"`),
		base(`"transition_notes":"` + strings.Repeat("x", 2001) + `"`),
		`{"scenes":[{"key":"intro-1","script_section_key":"intro","narration":"First approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5},{"key":"intro-1","script_section_key":"intro","narration":"First approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5},{"key":"proof-1","script_section_key":"proof","narration":"Second approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5}]}`,
	}
	for index, raw := range cases {
		t.Run(fmt.Sprintf("case_%d", index), func(t *testing.T) {
			requireInvalidOutput(t, generateWithJSON(raw, validRequest()))
		})
	}
}

func TestEngineGenerateRejectsMoreThan500Scenes(t *testing.T) {
	var builder strings.Builder
	builder.WriteString(`{"scenes":[`)
	for index := 0; index < 501; index++ {
		if index > 0 {
			builder.WriteByte(',')
		}
		fmt.Fprintf(&builder, `{"key":"intro-%d","script_section_key":"intro","narration":"First approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5}`, index)
	}
	builder.WriteString(`]}`)
	requireInvalidOutput(t, generateWithJSON(builder.String(), validRequest()))
}

func TestEngineGenerateUsesUnicodeCharacterLimits(t *testing.T) {
	t.Run("narration 20000 runes passes", func(t *testing.T) {
		body := strings.Repeat("ế", 20000)
		request := validRequest()
		request.Script.Sections[0].Body = body
		raw := fmt.Sprintf(`{"scenes":[{"key":"intro-1","script_section_key":"intro","narration":%q,"visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5},{"key":"proof-1","script_section_key":"proof","narration":"Second approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5}]}`, body)
		if err := generateWithJSON(raw, request); err != nil {
			t.Fatalf("expected 20000 Unicode runes to pass, got %v", err)
		}
	})

	t.Run("narration 20001 runes fails", func(t *testing.T) {
		body := strings.Repeat("ế", 20001)
		request := validRequest()
		request.Script.Sections[0].Body = body
		raw := fmt.Sprintf(`{"scenes":[{"key":"intro-1","script_section_key":"intro","narration":%q,"visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5},{"key":"proof-1","script_section_key":"proof","narration":"Second approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5}]}`, body)
		requireInvalidOutput(t, generateWithJSON(raw, request))
	})

	t.Run("visual instruction 5000 runes passes", func(t *testing.T) {
		visual := strings.Repeat("ế", 5000)
		raw := fmt.Sprintf(`{"scenes":[{"key":"intro-1","script_section_key":"intro","narration":"First approved narration.","visual_instruction":%q,"planned_source_type":"stock","expected_duration_seconds":5},{"key":"proof-1","script_section_key":"proof","narration":"Second approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5}]}`, visual)
		if err := generateWithJSON(raw, validRequest()); err != nil {
			t.Fatalf("expected 5000 Unicode runes to pass, got %v", err)
		}
	})

	t.Run("visual instruction 5001 runes fails", func(t *testing.T) {
		visual := strings.Repeat("ế", 5001)
		raw := fmt.Sprintf(`{"scenes":[{"key":"intro-1","script_section_key":"intro","narration":"First approved narration.","visual_instruction":%q,"planned_source_type":"stock","expected_duration_seconds":5},{"key":"proof-1","script_section_key":"proof","narration":"Second approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5}]}`, visual)
		requireInvalidOutput(t, generateWithJSON(raw, validRequest()))
	})
}

func TestEngineGenerateRejectsNarrationCoverageAndSectionOrderDrift(t *testing.T) {
	cases := map[string]string{
		"unknown section":          sceneJSON("missing", "missing-1", "First approved narration."),
		"missing section coverage": sceneJSON("intro", "intro-1", "First approved narration."),
		"omitted narration":        `{"scenes":[{"key":"intro-1","script_section_key":"intro","narration":"First","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5},{"key":"proof-1","script_section_key":"proof","narration":"Second approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5}]}`,
		"added narration":          `{"scenes":[{"key":"intro-1","script_section_key":"intro","narration":"First approved narration. Added.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5},{"key":"proof-1","script_section_key":"proof","narration":"Second approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5}]}`,
		"paraphrased narration":    `{"scenes":[{"key":"intro-1","script_section_key":"intro","narration":"Different approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5},{"key":"proof-1","script_section_key":"proof","narration":"Second approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5}]}`,
		"reordered sections":       `{"scenes":[{"key":"proof-1","script_section_key":"proof","narration":"Second approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5},{"key":"intro-1","script_section_key":"intro","narration":"First approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5}]}`,
		"non-contiguous sections":  `{"scenes":[{"key":"intro-1","script_section_key":"intro","narration":"First approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5},{"key":"proof-1","script_section_key":"proof","narration":"Second approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5},{"key":"intro-2","script_section_key":"intro","narration":"First approved narration.","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5}]}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) { requireInvalidOutput(t, generateWithJSON(raw, validRequest())) })
	}
}

func TestEngineGenerateAcceptsCanonicalWhitespaceSegmentation(t *testing.T) {
	raw := `{"scenes":[{"key":"intro-1","script_section_key":"intro","narration":" First   approved\n","visual_instruction":"Show it.","planned_source_type":"stock","expected_duration_seconds":5},{"key":"intro-2","script_section_key":"intro","narration":" narration. ","visual_instruction":"Continue it.","planned_source_type":"upload","expected_duration_seconds":5},{"key":"proof-1","script_section_key":"proof","narration":"Second\u00a0approved narration.","visual_instruction":"Show proof.","planned_source_type":"generated_image","expected_duration_seconds":5}]}`
	request := validRequest()
	request.Script.Sections[0].Body = "First approved narration."
	candidate, err := sceneplangeneration.NewWithGenerator(&capturingGenerator{response: providers.TextGenerationResponse{Text: raw}}).Generate(context.Background(), request)
	if err != nil {
		t.Fatalf("expected canonical whitespace segmentation to pass, got %v", err)
	}
	if len(candidate.Scenes) != 3 || candidate.Scenes[1].PlannedSourceType != sceneplangeneration.SourceTypeUpload {
		t.Fatalf("unexpected candidate: %+v", candidate)
	}
}

func TestEngineGenerateMapsProviderErrorsSafely(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code sceneplangeneration.Code
	}{
		{name: "unavailable", err: providers.ErrProviderUnavailable, code: sceneplangeneration.CodeProviderUnavailable},
		{name: "execution", err: providers.ErrProviderExecution, code: sceneplangeneration.CodeProviderFailed},
		{name: "secret-bearing", err: errors.New("upstream secret=do-not-leak"), code: sceneplangeneration.CodeProviderFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sceneplangeneration.NewWithGenerator(sceneplangeneration.FailingGenerator{Err: tc.err}).Generate(context.Background(), validRequest())
			var generationErr *sceneplangeneration.Error
			if !errors.As(err, &generationErr) || generationErr.Code != tc.code {
				t.Fatalf("expected %s, got %v", tc.code, err)
			}
			if strings.Contains(err.Error(), "do-not-leak") || strings.Contains(err.Error(), "secret") {
				t.Fatalf("provider detail leaked: %v", err)
			}
		})
	}
}

func TestEngineGenerateRegistryResolutionFailureIsSafe(t *testing.T) {
	registry := providers.NewRegistry()
	_, err := sceneplangeneration.New(registry).Generate(context.Background(), validRequest())
	var generationErr *sceneplangeneration.Error
	if !errors.As(err, &generationErr) || generationErr.Code != sceneplangeneration.CodeProviderUnavailable {
		t.Fatalf("expected provider unavailable, got %v", err)
	}
	if strings.Contains(err.Error(), "fake-provider") || strings.Contains(err.Error(), "fake-text-model") {
		t.Fatalf("provider lookup details leaked: %v", err)
	}
}

type delayedGenerator struct {
	response providers.TextGenerationResponse
	delay    time.Duration
}

func (g delayedGenerator) GenerateText(ctx context.Context, _ providers.TextGenerationRequest) (providers.TextGenerationResponse, error) {
	timer := time.NewTimer(g.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return providers.TextGenerationResponse{}, ctx.Err()
	case <-timer.C:
		return g.response, nil
	}
}

func TestEngineGeneratePropagatesCancellationAndDeadline(t *testing.T) {
	t.Run("before provider call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := sceneplangeneration.NewWithGenerator(delayedGenerator{delay: time.Second}).Generate(ctx, validRequest())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
	t.Run("during provider call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() { time.Sleep(10 * time.Millisecond); cancel() }()
		_, err := sceneplangeneration.NewWithGenerator(delayedGenerator{response: providers.TextGenerationResponse{Text: validScenePlanJSON}, delay: time.Second}).Generate(ctx, validRequest())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
	t.Run("deadline during provider call", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		_, err := sceneplangeneration.NewWithGenerator(delayedGenerator{response: providers.TextGenerationResponse{Text: validScenePlanJSON}, delay: time.Second}).Generate(ctx, validRequest())
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded, got %v", err)
		}
	})
}

func TestEngineGenerateDoesNotMutateNestedInputs(t *testing.T) {
	request := validRequest()
	before := request
	before.Project.TargetDurationSeconds = cloneInt(before.Project.TargetDurationSeconds)
	before.Script.EstimatedDurationSeconds = cloneInt(before.Script.EstimatedDurationSeconds)
	before.Script.Sections = append([]sceneplangeneration.ScriptSection(nil), before.Script.Sections...)
	before.Proposal.Warnings = append([]string(nil), before.Proposal.Warnings...)
	before.Proposal.ResearchGaps = append([]string(nil), before.Proposal.ResearchGaps...)

	generator := &capturingGenerator{response: providers.TextGenerationResponse{Text: validScenePlanJSON}}
	if _, err := sceneplangeneration.NewWithGenerator(generator).Generate(context.Background(), request); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !reflect.DeepEqual(request, before) {
		t.Fatalf("request mutated after success:\nbefore=%+v\nafter=%+v", before, request)
	}

	generator.response.Text = `{"scenes":[]}`
	if _, err := sceneplangeneration.NewWithGenerator(generator).Generate(context.Background(), request); err == nil {
		t.Fatal("expected invalid output")
	}
	if !reflect.DeepEqual(request, before) {
		t.Fatalf("request mutated after failure:\nbefore=%+v\nafter=%+v", before, request)
	}
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func TestEngineGenerateRejectsMismatchedProposalVersion(t *testing.T) {
	request := validRequest()
	request.Proposal.Version++
	requireInvalidOutput(t, generateWithJSON(validScenePlanJSON, request))
}
