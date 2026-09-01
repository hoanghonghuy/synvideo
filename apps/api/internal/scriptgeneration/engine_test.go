package scriptgeneration_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/fake"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scriptgeneration"
)

func validProjectContext() scriptgeneration.ProjectContext {
	duration := 60
	return scriptgeneration.ProjectContext{
		ID:                    uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		ContentFormat:         project.ContentFormatShort,
		AspectRatio:           project.AspectRatio9x16,
		TargetDurationSeconds: &duration,
		Locale:                project.LocaleVI,
	}
}

func validProposalContext() scriptgeneration.ProposalContext {
	duration := 60
	return scriptgeneration.ProposalContext{
		Version:                  2,
		TitleOptions:             []string{"Chiến lược AI video ngắn", "Tối ưu hóa video bằng AI"},
		HookOptions:              []string{"Bạn có biết 80% người xem dừng lại ở 3 giây đầu?"},
		AudienceSummary:          "Nhà sáng tạo nội dung và marketer trẻ",
		ObjectiveSummary:         "Tăng tỷ lệ tương tác và chuyển đổi trên TikTok/Reels",
		NarrativeAngle:           "Kể câu chuyện chuyển đổi từ thất bại sang thành công",
		EstimatedDurationSeconds: &duration,
		FormatRationale:          "Định dạng dọc 9:16 tối ưu cho xem trên di động",
		Structure: []scriptgeneration.ProposalStructureItem{
			{Key: "hook", Title: "Mở đầu ấn tượng", Purpose: "Giữ chân người xem trong 3s đầu"},
			{Key: "problem", Title: "Vấn đề cốt lõi", Purpose: "Nêu nỗi đau của nhà sáng tạo nội dung"},
			{Key: "solution", Title: "Giải pháp SynVideo", Purpose: "Giới thiệu workflow tự động hóa"},
			{Key: "cta", Title: "Kêu gọi hành động", Purpose: "Thúc đẩy đăng ký trải nghiệm"},
		},
		VisualDirection:  "Khung hình hiện đại, tông màu sáng, đồ họa động nhanh",
		VoiceDirection:   "Giọng đọc năng động, tự tin, phát âm rõ ràng",
		MusicDirection:   "Nhịp điệu lo-fi upbeat, đẩy cao trào ở phần giải pháp",
		CaptionDirection: "Phụ đề chữ lớn ở giữa màn hình, hiệu ứng từ khoá nổi bật",
		CallToAction:     "Nhấn link ở bio để dùng thử SynVideo miễn phí",
		ResearchGaps:     []string{"Cần cập nhật số liệu thuật toán TikTok mới nhất Q3/2026"},
		Warnings:         []string{"Không hứa hẹn tăng trưởng 100% doanh thu trong 24h"},
	}
}

func validRequest() scriptgeneration.Request {
	return scriptgeneration.Request{
		Project:    validProjectContext(),
		Proposal:   validProposalContext(),
		ProviderID: "fake-provider",
		ModelID:    "fake-text-model",
	}
}

const sampleValidJSON = `{
  "sections": [
    {
      "key": "hook",
      "heading": "Mở đầu thu hút",
      "body": "Bạn có biết 80% người xem lướt qua video chỉ sau 3 giây đầu tiên? Đây là cách bạn giữ chân họ."
    },
    {
      "key": "problem",
      "heading": "Nỗi đau sáng tạo nội dung",
      "body": "Dành hàng giờ viết kịch bản nhưng video vẫn lẹt đẹt vài trăm view. Vấn đề không phải nội dung dở, mà là cấu trúc chưa đúng."
    },
    {
      "key": "solution",
      "heading": "Workflow đột phá cùng SynVideo",
      "body": "SynVideo giúp bạn chuẩn hóa brief, lên proposal và tạo kịch bản chuẩn cấu trúc chỉ trong vài phút."
    },
    {
      "key": "cta",
      "heading": "Hành động ngay",
      "body": "Đừng để ý tưởng tuyệt vời bị bỏ lỡ. Nhấn ngay vào bio để trải nghiệm SynVideo hoàn toàn miễn phí hôm nay!"
    }
  ],
  "estimated_duration_seconds": 60,
  "notes": "Nhấn mạnh từ khóa '3 giây đầu' ở đoạn mở đầu."
}`

type capturingGenerator struct {
	capturedRequest providers.TextGenerationRequest
	response        providers.TextGenerationResponse
	err             error
	delay           time.Duration
}

func (c *capturingGenerator) GenerateText(ctx context.Context, req providers.TextGenerationRequest) (providers.TextGenerationResponse, error) {
	c.capturedRequest = req
	if c.delay > 0 {
		select {
		case <-ctx.Done():
			return providers.TextGenerationResponse{}, ctx.Err()
		case <-time.After(c.delay):
		}
	}
	if err := ctx.Err(); err != nil {
		return providers.TextGenerationResponse{}, err
	}
	return c.response, c.err
}

func TestEngine_Generate_Success(t *testing.T) {
	generator := &capturingGenerator{
		response: providers.TextGenerationResponse{
			ProviderID: "fake-provider",
			ModelID:    "fake-text-model",
			Text:       sampleValidJSON,
		},
	}
	engine := scriptgeneration.NewWithGenerator(generator)

	req := validRequest()
	candidate, err := engine.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if candidate.SourceProposalVersion != 2 {
		t.Fatalf("expected SourceProposalVersion 2, got %d", candidate.SourceProposalVersion)
	}
	if len(candidate.Sections) != 4 {
		t.Fatalf("expected 4 sections, got %d", len(candidate.Sections))
	}
	if candidate.Sections[0].Key != "hook" || candidate.Sections[0].Heading != "Mở đầu thu hút" {
		t.Fatalf("unexpected section 0: %+v", candidate.Sections[0])
	}
	if candidate.EstimatedDurationSeconds == nil || *candidate.EstimatedDurationSeconds != 60 {
		t.Fatalf("expected EstimatedDurationSeconds 60, got %v", candidate.EstimatedDurationSeconds)
	}
	if candidate.Notes != "Nhấn mạnh từ khóa '3 giây đầu' ở đoạn mở đầu." {
		t.Fatalf("unexpected notes: %q", candidate.Notes)
	}
}

func TestEngine_Generate_PromptStructure(t *testing.T) {
	generator := &capturingGenerator{
		response: providers.TextGenerationResponse{
			ProviderID: "fake-provider",
			ModelID:    "fake-text-model",
			Text:       sampleValidJSON,
		},
	}
	engine := scriptgeneration.NewWithGenerator(generator)

	req := validRequest()
	_, err := engine.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if len(generator.capturedRequest.Messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(generator.capturedRequest.Messages))
	}

	userMessage := generator.capturedRequest.Messages[1].Content
	expectedSnippets := []string{
		"template_version: script_v1",
		"Chiến lược AI video ngắn",
		"Bạn có biết 80% người xem dừng lại ở 3 giây đầu?",
		"Nhà sáng tạo nội dung và marketer trẻ",
		"Tăng tỷ lệ tương tác và chuyển đổi trên TikTok/Reels",
		"Kể câu chuyện chuyển đổi từ thất bại sang thành công",
		"Khung hình hiện đại, tông màu sáng, đồ họa động nhanh",
		"Giọng đọc năng động, tự tin, phát âm rõ ràng",
		"Nhịp điệu lo-fi upbeat, đẩy cao trào ở phần giải pháp",
		"Phụ đề chữ lớn ở giữa màn hình, hiệu ứng từ khoá nổi bật",
		"Nhấn link ở bio để dùng thử SynVideo miễn phí",
		"content_format: short",
		"aspect_ratio: 9:16",
		"target_duration_seconds: 60",
		"locale: vi",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(userMessage, snippet) {
			t.Errorf("prompt missing expected snippet: %q", snippet)
		}
	}
}

func TestEngine_Generate_PromptResearchGapsAndWarnings(t *testing.T) {
	generator := &capturingGenerator{
		response: providers.TextGenerationResponse{
			ProviderID: "fake-provider",
			ModelID:    "fake-text-model",
			Text:       sampleValidJSON,
		},
	}
	engine := scriptgeneration.NewWithGenerator(generator)

	req := validRequest()
	_, err := engine.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	userMessage := generator.capturedRequest.Messages[1].Content
	if !strings.Contains(userMessage, "Cần cập nhật số liệu thuật toán TikTok mới nhất Q3/2026") {
		t.Errorf("prompt missing research gaps constraint")
	}
	if !strings.Contains(userMessage, "Không hứa hẹn tăng trưởng 100% doanh thu trong 24h") {
		t.Errorf("prompt missing warnings constraint")
	}
}

func TestEngine_Generate_LongFormPreserved(t *testing.T) {
	generator := &capturingGenerator{
		response: providers.TextGenerationResponse{
			ProviderID: "fake-provider",
			ModelID:    "fake-text-model",
			Text:       sampleValidJSON,
		},
	}
	engine := scriptgeneration.NewWithGenerator(generator)

	req := validRequest()
	req.Project.ContentFormat = project.ContentFormatLong
	duration := 600
	req.Project.TargetDurationSeconds = &duration

	_, err := engine.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	userMessage := generator.capturedRequest.Messages[1].Content
	if !strings.Contains(userMessage, "content_format: long") {
		t.Errorf("prompt missing long format")
	}
	if !strings.Contains(userMessage, "long-form") {
		t.Errorf("prompt missing long-form guidance")
	}
}

func TestEngine_Generate_MalformedAndTrailingJSON(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"empty string", ""},
		{"whitespace only", "   \n\t "},
		{"malformed json", `{"sections": [{"key": "hook", "body": "test"`},
		{"trailing json", sampleValidJSON + ` {"extra": true}`},
		{"array instead of object", `[{"key": "hook", "body": "test"}]`},
		{"scalar string", `"hello world"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			generator := &capturingGenerator{
				response: providers.TextGenerationResponse{
					ProviderID: "fake-provider",
					ModelID:    "fake-text-model",
					Text:       tc.raw,
				},
			}
			engine := scriptgeneration.NewWithGenerator(generator)

			_, err := engine.Generate(context.Background(), validRequest())
			if err == nil {
				t.Fatalf("expected error, got nil")
			}

			var genErr *scriptgeneration.Error
			if !errors.As(err, &genErr) {
				t.Fatalf("expected *scriptgeneration.Error, got: %T (%v)", err, err)
			}
			if genErr.Code != scriptgeneration.CodeInvalidOutput {
				t.Fatalf("expected CodeInvalidOutput, got: %v", genErr.Code)
			}
		})
	}
}

func TestEngine_Generate_DisallowUnknownFields(t *testing.T) {
	jsonWithUnknown := `{
		"sections": [
			{
				"key": "hook",
				"heading": "Intro",
				"body": "Body content here",
				"extra_field": "disallowed"
			}
		],
		"server_version": 999
	}`

	generator := &capturingGenerator{
		response: providers.TextGenerationResponse{
			ProviderID: "fake-provider",
			ModelID:    "fake-text-model",
			Text:       jsonWithUnknown,
		},
	}
	engine := scriptgeneration.NewWithGenerator(generator)

	_, err := engine.Generate(context.Background(), validRequest())
	if err == nil {
		t.Fatalf("expected error on unknown fields, got nil")
	}

	var genErr *scriptgeneration.Error
	if !errors.As(err, &genErr) || genErr.Code != scriptgeneration.CodeInvalidOutput {
		t.Fatalf("expected CodeInvalidOutput, got: %v", err)
	}
}

func TestEngine_Generate_SectionValidation(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{
			name: "zero sections",
			json: `{"sections": []}`,
		},
		{
			name: "invalid key format uppercase",
			json: `{"sections": [{"key": "Hook", "body": "Nội dung"}]}`,
		},
		{
			name: "invalid key format with spaces",
			json: `{"sections": [{"key": "hook section", "body": "Nội dung"}]}`,
		},
		{
			name: "duplicate keys",
			json: `{"sections": [{"key": "hook", "body": "1"}, {"key": "hook", "body": "2"}]}`,
		},
		{
			name: "empty body",
			json: `{"sections": [{"key": "hook", "body": "   "}]}`,
		},
		{
			name: "key longer than 64 chars",
			json: fmt.Sprintf(`{"sections": [{"key": "%s", "body": "valid body"}]}`, strings.Repeat("a", 65)),
		},
		{
			name: "heading exceeds 300 chars",
			json: fmt.Sprintf(`{"sections": [{"key": "hook", "heading": "%s", "body": "valid body"}]}`, strings.Repeat("a", 301)),
		},
		{
			name: "body exceeds 20000 chars",
			json: fmt.Sprintf(`{"sections": [{"key": "hook", "body": "%s"}]}`, strings.Repeat("a", 20001)),
		},
		{
			name: "notes exceeds 10000 chars",
			json: fmt.Sprintf(`{"sections": [{"key": "hook", "body": "valid body"}], "notes": "%s"}`, strings.Repeat("a", 10001)),
		},
		{
			name: "estimated_duration_seconds negative",
			json: `{"sections": [{"key": "hook", "body": "valid body"}], "estimated_duration_seconds": -1}`,
		},
		{
			name: "estimated_duration_seconds zero",
			json: `{"sections": [{"key": "hook", "body": "valid body"}], "estimated_duration_seconds": 0}`,
		},
		{
			name: "estimated_duration_seconds exceeds 43200",
			json: `{"sections": [{"key": "hook", "body": "valid body"}], "estimated_duration_seconds": 43201}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			generator := &capturingGenerator{
				response: providers.TextGenerationResponse{
					ProviderID: "fake-provider",
					ModelID:    "fake-text-model",
					Text:       tc.json,
				},
			}
			engine := scriptgeneration.NewWithGenerator(generator)

			_, err := engine.Generate(context.Background(), validRequest())
			if err == nil {
				t.Fatalf("expected validation error, got nil")
			}

			var genErr *scriptgeneration.Error
			if !errors.As(err, &genErr) || genErr.Code != scriptgeneration.CodeInvalidOutput {
				t.Fatalf("expected CodeInvalidOutput, got: %v", err)
			}
		})
	}
}

func TestEngine_Generate_MultibyteUnicodeLimits(t *testing.T) {
	rune3 := "ế"

	exact300Heading := strings.Repeat(rune3, 300)
	over300Heading := strings.Repeat(rune3, 301)

	exact20000Body := strings.Repeat(rune3, 20000)
	over20000Body := strings.Repeat(rune3, 20001)

	exact10000Notes := strings.Repeat(rune3, 10000)
	over10000Notes := strings.Repeat(rune3, 10001)

	t.Run("multibyte heading exactly 300 runes passes", func(t *testing.T) {
		validJSON := fmt.Sprintf(`{"sections": [{"key": "hook", "heading": "%s", "body": "Nội dung"}]}`, exact300Heading)
		generator := &capturingGenerator{
			response: providers.TextGenerationResponse{
				ProviderID: "fake-provider",
				ModelID:    "fake-text-model",
				Text:       validJSON,
			},
		}
		engine := scriptgeneration.NewWithGenerator(generator)
		candidate, err := engine.Generate(context.Background(), validRequest())
		if err != nil {
			t.Fatalf("expected pass for 300 runes heading, got error: %v", err)
		}
		if candidate.Sections[0].Heading != exact300Heading {
			t.Fatalf("heading mismatch")
		}
	})

	t.Run("multibyte heading 301 runes fails", func(t *testing.T) {
		invalidJSON := fmt.Sprintf(`{"sections": [{"key": "hook", "heading": "%s", "body": "Nội dung"}]}`, over300Heading)
		generator := &capturingGenerator{
			response: providers.TextGenerationResponse{
				ProviderID: "fake-provider",
				ModelID:    "fake-text-model",
				Text:       invalidJSON,
			},
		}
		engine := scriptgeneration.NewWithGenerator(generator)
		_, err := engine.Generate(context.Background(), validRequest())
		if err == nil {
			t.Fatalf("expected error for 301 runes heading, got nil")
		}
	})

	t.Run("multibyte body exactly 20000 runes passes", func(t *testing.T) {
		validJSON := fmt.Sprintf(`{"sections": [{"key": "hook", "body": "%s"}]}`, exact20000Body)
		generator := &capturingGenerator{
			response: providers.TextGenerationResponse{
				ProviderID: "fake-provider",
				ModelID:    "fake-text-model",
				Text:       validJSON,
			},
		}
		engine := scriptgeneration.NewWithGenerator(generator)
		candidate, err := engine.Generate(context.Background(), validRequest())
		if err != nil {
			t.Fatalf("expected pass for 20000 runes body, got error: %v", err)
		}
		if candidate.Sections[0].Body != exact20000Body {
			t.Fatalf("body mismatch")
		}
	})

	t.Run("multibyte body 20001 runes fails", func(t *testing.T) {
		invalidJSON := fmt.Sprintf(`{"sections": [{"key": "hook", "body": "%s"}]}`, over20000Body)
		generator := &capturingGenerator{
			response: providers.TextGenerationResponse{
				ProviderID: "fake-provider",
				ModelID:    "fake-text-model",
				Text:       invalidJSON,
			},
		}
		engine := scriptgeneration.NewWithGenerator(generator)
		_, err := engine.Generate(context.Background(), validRequest())
		if err == nil {
			t.Fatalf("expected error for 20001 runes body, got nil")
		}
	})

	t.Run("multibyte notes exactly 10000 runes passes", func(t *testing.T) {
		validJSON := fmt.Sprintf(`{"sections": [{"key": "hook", "body": "Nội dung"}], "notes": "%s"}`, exact10000Notes)
		generator := &capturingGenerator{
			response: providers.TextGenerationResponse{
				ProviderID: "fake-provider",
				ModelID:    "fake-text-model",
				Text:       validJSON,
			},
		}
		engine := scriptgeneration.NewWithGenerator(generator)
		candidate, err := engine.Generate(context.Background(), validRequest())
		if err != nil {
			t.Fatalf("expected pass for 10000 runes notes, got error: %v", err)
		}
		if candidate.Notes != exact10000Notes {
			t.Fatalf("notes mismatch")
		}
	})

	t.Run("multibyte notes 10001 runes fails", func(t *testing.T) {
		invalidJSON := fmt.Sprintf(`{"sections": [{"key": "hook", "body": "Nội dung"}], "notes": "%s"}`, over10000Notes)
		generator := &capturingGenerator{
			response: providers.TextGenerationResponse{
				ProviderID: "fake-provider",
				ModelID:    "fake-text-model",
				Text:       invalidJSON,
			},
		}
		engine := scriptgeneration.NewWithGenerator(generator)
		_, err := engine.Generate(context.Background(), validRequest())
		if err == nil {
			t.Fatalf("expected error for 10001 runes notes, got nil")
		}
	})
}

func TestEngine_Generate_ProviderErrors(t *testing.T) {
	t.Run("provider unavailable maps to CodeProviderUnavailable", func(t *testing.T) {
		engine := scriptgeneration.NewWithGenerator(scriptgeneration.FailingGenerator{
			Err: providers.ErrProviderUnavailable,
		})
		_, err := engine.Generate(context.Background(), validRequest())
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var genErr *scriptgeneration.Error
		if !errors.As(err, &genErr) || genErr.Code != scriptgeneration.CodeProviderUnavailable {
			t.Fatalf("expected CodeProviderUnavailable, got: %v", err)
		}
	})

	t.Run("provider execution failure maps to CodeProviderFailed", func(t *testing.T) {
		engine := scriptgeneration.NewWithGenerator(scriptgeneration.FailingGenerator{
			Err: providers.ErrProviderExecution,
		})
		_, err := engine.Generate(context.Background(), validRequest())
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var genErr *scriptgeneration.Error
		if !errors.As(err, &genErr) || genErr.Code != scriptgeneration.CodeProviderFailed {
			t.Fatalf("expected CodeProviderFailed, got: %v", err)
		}
	})
}

func TestEngine_Generate_ContextCancellationAndDeadline(t *testing.T) {
	t.Run("canceled context before call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		generator := &capturingGenerator{
			response: providers.TextGenerationResponse{Text: sampleValidJSON},
		}
		engine := scriptgeneration.NewWithGenerator(generator)
		_, err := engine.Generate(ctx, validRequest())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
	})

	t.Run("canceled context during in-flight call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		generator := &capturingGenerator{
			response: providers.TextGenerationResponse{Text: sampleValidJSON},
			delay:    50 * time.Millisecond,
		}
		engine := scriptgeneration.NewWithGenerator(generator)

		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		_, err := engine.Generate(ctx, validRequest())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
	})

	t.Run("deadline exceeded during in-flight call", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		generator := &capturingGenerator{
			response: providers.TextGenerationResponse{Text: sampleValidJSON},
			delay:    50 * time.Millisecond,
		}
		engine := scriptgeneration.NewWithGenerator(generator)

		_, err := engine.Generate(ctx, validRequest())
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded, got: %v", err)
		}
	})
}

func TestEngine_Generate_InputImmutability(t *testing.T) {
	generator := &capturingGenerator{
		response: providers.TextGenerationResponse{
			ProviderID: "fake-provider",
			ModelID:    "fake-text-model",
			Text:       sampleValidJSON,
		},
	}
	engine := scriptgeneration.NewWithGenerator(generator)

	req := validRequest()
	originalTitleOptions := append([]string(nil), req.Proposal.TitleOptions...)
	originalStructure := append([]scriptgeneration.ProposalStructureItem(nil), req.Proposal.Structure...)
	originalResearchGaps := append([]string(nil), req.Proposal.ResearchGaps...)
	originalWarnings := append([]string(nil), req.Proposal.Warnings...)
	originalDuration := *req.Project.TargetDurationSeconds

	_, err := engine.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	if len(req.Proposal.TitleOptions) != len(originalTitleOptions) || req.Proposal.TitleOptions[0] != originalTitleOptions[0] {
		t.Errorf("TitleOptions was mutated")
	}
	if len(req.Proposal.Structure) != len(originalStructure) || req.Proposal.Structure[0] != originalStructure[0] {
		t.Errorf("Structure was mutated")
	}
	if len(req.Proposal.ResearchGaps) != len(originalResearchGaps) || req.Proposal.ResearchGaps[0] != originalResearchGaps[0] {
		t.Errorf("ResearchGaps was mutated")
	}
	if len(req.Proposal.Warnings) != len(originalWarnings) || req.Proposal.Warnings[0] != originalWarnings[0] {
		t.Errorf("Warnings was mutated")
	}
	if *req.Project.TargetDurationSeconds != originalDuration {
		t.Errorf("TargetDurationSeconds was mutated")
	}
}

func TestEngine_New_WithRegistry(t *testing.T) {
	registry := providers.NewRegistry()
	textGen := fake.NewTextGenerator(sampleValidJSON)
	err := registry.Register(providers.Registration{
		Provider: providers.ProviderMetadata{
			ID:          "fake-provider",
			DisplayName: "Fake Provider",
		},
		Models: []providers.ModelRegistration{
			{
				Metadata: providers.ModelMetadata{
					ProviderID:            "fake-provider",
					ID:                    "fake-text-model",
					DisplayName:           "Fake Text Model",
					SupportedCapabilities: []providers.Capability{providers.CapabilityTextGeneration},
				},
				TextGenerator: textGen,
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to register fake provider: %v", err)
	}

	engine := scriptgeneration.New(registry)
	req := validRequest()
	candidate, err := engine.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success with registry, got: %v", err)
	}
	if candidate.SourceProposalVersion != 2 {
		t.Fatalf("expected SourceProposalVersion 2, got %d", candidate.SourceProposalVersion)
	}
}
