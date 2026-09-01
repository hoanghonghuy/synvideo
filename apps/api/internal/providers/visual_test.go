package providers_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/fake"
)

func TestImageGenerationRequestValidationAndDeepCopy(t *testing.T) {
	tooLong := strings.Repeat("x", providers.MaxVisualPromptRunes+1)
	count := providers.MaxImageOutputs + 1
	for name, req := range map[string]providers.ImageGenerationRequest{
		"missing prompt":         {},
		"prompt too long":        {Prompt: tooLong},
		"output count too large": {Prompt: "a", OutputCount: &count},
		"invalid aspect ratio":   {Prompt: "a", AspectRatio: "4:3"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := req.Validate(); !errors.Is(err, providers.ErrInvalidRequest) {
				t.Fatalf("validation error = %v, want ErrInvalidRequest", err)
			}
		})
	}

	gen := fake.NewImageGenerator([]byte("image bytes"))
	req := providers.ImageGenerationRequest{
		ProviderID:      "fake",
		ModelID:         "image-v1",
		Prompt:          "before",
		ReferenceImages: []providers.BinaryInput{fake.NewBinaryInput("image/png", []byte("reference"))},
	}
	if _, err := gen.GenerateImage(context.Background(), req); err != nil {
		t.Fatalf("generate image: %v", err)
	}
	req.ReferenceImages[0] = fake.NewBinaryInput("image/jpeg", []byte("mutated"))
	req.Prompt = "after"
	requests := gen.Requests()
	if len(requests) != 1 || requests[0].Prompt != "before" || requests[0].ReferenceImages[0].MIMEType() != "image/png" {
		t.Fatalf("captured request was not deeply cloned: %#v", requests)
	}
}

func TestFakeImageGeneratorStreamsCanonicalOutputAndContext(t *testing.T) {
	gen := fake.NewImageGenerator([]byte("png data"))
	response, err := gen.GenerateImage(context.Background(), providers.ImageGenerationRequest{Prompt: "a still"})
	if err != nil {
		t.Fatalf("generate image: %v", err)
	}
	if len(response.Outputs) != 1 || response.Outputs[0].Binary.MIMEType() != "image/png" {
		t.Fatalf("unexpected image response: %#v", response)
	}
	stream, err := response.Outputs[0].Binary.Open(context.Background())
	if err != nil {
		t.Fatalf("open image: %v", err)
	}
	data, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read image: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close image: %v", err)
	}
	if string(data) != "png data" {
		t.Fatalf("image data = %q, want png data", data)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := gen.GenerateImage(ctx, providers.ImageGenerationRequest{Prompt: "cancelled"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled image error = %v, want context.Canceled", err)
	}
}

func TestFakeVideoGeneratorUsesOpaqueAsyncLifecycle(t *testing.T) {
	gen := fake.NewVideoGenerator([]byte("video bytes"))
	operation, err := gen.StartVideo(context.Background(), providers.VideoGenerationRequest{Prompt: "animate this"})
	if err != nil {
		t.Fatalf("start video: %v", err)
	}
	if operation.ID == "" || strings.Contains(operation.ID, "provider") || operation.State != providers.VideoOperationQueued {
		t.Fatalf("unexpected initial operation: %#v", operation)
	}
	if _, err := gen.OpenVideoResult(context.Background(), operation.ID); !errors.Is(err, providers.ErrResultUnavailable) {
		t.Fatalf("early result error = %v, want ErrResultUnavailable", err)
	}
	running, err := gen.GetVideoOperation(context.Background(), operation.ID)
	if err != nil || running.State != providers.VideoOperationRunning {
		t.Fatalf("running operation = %#v, error = %v", running, err)
	}
	succeeded, err := gen.GetVideoOperation(context.Background(), operation.ID)
	if err != nil || succeeded.State != providers.VideoOperationSucceeded || succeeded.Progress == nil || *succeeded.Progress != 100 {
		t.Fatalf("succeeded operation = %#v, error = %v", succeeded, err)
	}
	result, err := gen.OpenVideoResult(context.Background(), operation.ID)
	if err != nil {
		t.Fatalf("open video result: %v", err)
	}
	stream, err := result.Open(context.Background())
	if err != nil {
		t.Fatalf("open result stream: %v", err)
	}
	data, _ := io.ReadAll(stream)
	_ = stream.Close()
	if string(data) != "video bytes" {
		t.Fatalf("video data = %q, want video bytes", data)
	}
	if _, err := gen.GetVideoOperation(context.Background(), "opaque-missing"); !errors.Is(err, providers.ErrUnknownVideoOperation) {
		t.Fatalf("missing operation error = %v, want ErrUnknownVideoOperation", err)
	}
}

func TestRegistryResolvesVisualCapabilitiesIndependently(t *testing.T) {
	registry := providers.NewRegistry()
	textGen := fake.NewTextGenerator("text")
	imageGen := fake.NewImageGenerator([]byte("image"))
	videoGen := fake.NewVideoGenerator([]byte("video"))
	err := registry.Register(providers.Registration{
		Provider: providers.ProviderMetadata{ID: "multi", DisplayName: "Multi"},
		Models: []providers.ModelRegistration{{
			Metadata: providers.ModelMetadata{ProviderID: "multi", ID: "all-v1", DisplayName: "All V1", SupportedCapabilities: []providers.Capability{
				providers.CapabilityTextGeneration, providers.CapabilityImageGeneration, providers.CapabilityVideoGeneration,
			}},
			TextGenerator: textGen, ImageGenerator: imageGen, VideoGenerator: videoGen,
		}},
	})
	if err != nil {
		t.Fatalf("register multi-capability model: %v", err)
	}
	if resolved, _, err := registry.ResolveImageGenerator("multi", "all-v1"); err != nil || resolved != imageGen {
		t.Fatalf("resolve image = %v, error = %v", resolved, err)
	}
	if resolved, _, err := registry.ResolveVideoGenerator("multi", "all-v1"); err != nil || resolved != videoGen {
		t.Fatalf("resolve video = %v, error = %v", resolved, err)
	}
	if resolved, _, err := registry.ResolveTextGenerator("multi", "all-v1"); err != nil || resolved != textGen {
		t.Fatalf("legacy text resolve = %v, error = %v", resolved, err)
	}
}

func TestRegistryRejectsVisualCapabilityWithoutBinding(t *testing.T) {
	for capability, name := range map[providers.Capability]string{
		providers.CapabilityImageGeneration: "image",
		providers.CapabilityVideoGeneration: "video",
	} {
		t.Run(name, func(t *testing.T) {
			registry := providers.NewRegistry()
			err := registry.Register(providers.Registration{
				Provider: providers.ProviderMetadata{ID: "provider", DisplayName: "Provider"},
				Models: []providers.ModelRegistration{{
					Metadata: providers.ModelMetadata{ProviderID: "provider", ID: "model", DisplayName: "Model", SupportedCapabilities: []providers.Capability{capability}},
				}},
			})
			if err == nil {
				t.Fatal("expected missing binding validation error")
			}
		})
	}
}

func TestRegistryRejectsDuplicateCapabilityMetadata(t *testing.T) {
	err := providers.NewRegistry().Register(providers.Registration{
		Provider: providers.ProviderMetadata{ID: "provider", DisplayName: "Provider"},
		Models: []providers.ModelRegistration{{
			Metadata: providers.ModelMetadata{
				ProviderID: "provider", ID: "model", DisplayName: "Model",
				SupportedCapabilities: []providers.Capability{providers.CapabilityTextGeneration, providers.CapabilityTextGeneration},
			},
			TextGenerator: fake.NewTextGenerator("text"),
		}},
	})
	if err == nil {
		t.Fatal("expected duplicate capability validation error")
	}
}
