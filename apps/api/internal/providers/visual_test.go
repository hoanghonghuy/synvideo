package providers_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/fake"
)

type mutableBinaryInput struct {
	mime         string
	data         []byte
	reportedSize int64
}

func (b *mutableBinaryInput) MIMEType() string { return b.mime }
func (b *mutableBinaryInput) Size() int64 {
	if b.reportedSize != 0 {
		return b.reportedSize
	}
	return int64(len(b.data))
}
func (b *mutableBinaryInput) Open(context.Context) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(string(b.data))), nil
}

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

func TestVisualPortPayloadsDoNotDuplicateResolvedProviderIdentity(t *testing.T) {
	for _, payload := range []reflect.Type{
		reflect.TypeOf(providers.ImageGenerationRequest{}),
		reflect.TypeOf(providers.VideoGenerationRequest{}),
		reflect.TypeOf(providers.ImageGenerationResponse{}),
	} {
		for _, field := range []string{"ProviderID", "ModelID"} {
			if _, ok := payload.FieldByName(field); ok {
				t.Errorf("%s unexpectedly contains resolved identity field %s", payload.Name(), field)
			}
		}
	}
}

func TestFakeVisualGeneratorsSnapshotMutableBinaryInputsWithoutOptionalCloner(t *testing.T) {
	imageInput := &mutableBinaryInput{mime: "image/png", data: []byte("before")}
	imageGen := fake.NewImageGenerator([]byte("image"))
	if _, err := imageGen.GenerateImage(context.Background(), providers.ImageGenerationRequest{Prompt: "a", ReferenceImages: []providers.BinaryInput{imageInput}}); err != nil {
		t.Fatalf("generate image: %v", err)
	}
	imageInput.mime = "image/jpeg"
	imageInput.data[0] = 'X'
	imageRequests := imageGen.Requests()
	if imageRequests[0].ReferenceImages[0].MIMEType() != "image/png" {
		t.Fatalf("image fake retained mutable MIME input: %q", imageRequests[0].ReferenceImages[0].MIMEType())
	}
	imageStream, err := imageRequests[0].ReferenceImages[0].Open(context.Background())
	if err != nil {
		t.Fatalf("open captured image reference: %v", err)
	}
	imageData, _ := io.ReadAll(imageStream)
	_ = imageStream.Close()
	if string(imageData) != "before" {
		t.Fatalf("image fake retained mutable bytes: %q", imageData)
	}

	videoInput := &mutableBinaryInput{mime: "image/png", data: []byte("before")}
	videoGen := fake.NewVideoGenerator([]byte("video"))
	if _, err := videoGen.StartVideo(context.Background(), providers.VideoGenerationRequest{Prompt: "a", ReferenceImage: videoInput}); err != nil {
		t.Fatalf("start video: %v", err)
	}
	videoInput.mime = "image/jpeg"
	videoInput.data[0] = 'X'
	videoRequests := videoGen.Requests()
	if videoRequests[0].ReferenceImage.MIMEType() != "image/png" {
		t.Fatalf("video fake retained mutable MIME input: %q", videoRequests[0].ReferenceImage.MIMEType())
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

func TestVisualGenerationEnforcesCapabilitySpecificResultMIME(t *testing.T) {
	imageGen := fake.NewImageGenerator([]byte("not an image")).WithMIMEType("video/mp4")
	if _, err := imageGen.GenerateImage(context.Background(), providers.ImageGenerationRequest{Prompt: "a still"}); !errors.Is(err, providers.ErrMalformedResponse) {
		t.Fatalf("image wrong-family error = %v, want ErrMalformedResponse", err)
	}

	videoGen := fake.NewVideoGenerator([]byte("not a video")).WithMIMEType("image/png")
	operation, err := videoGen.StartVideo(context.Background(), providers.VideoGenerationRequest{Prompt: "a video"})
	if err != nil {
		t.Fatalf("start video: %v", err)
	}
	if _, err := videoGen.GetVideoOperation(context.Background(), operation.ID); err != nil {
		t.Fatalf("poll running: %v", err)
	}
	if _, err := videoGen.GetVideoOperation(context.Background(), operation.ID); err != nil {
		t.Fatalf("poll succeeded: %v", err)
	}
	if _, err := videoGen.OpenVideoResult(context.Background(), operation.ID); !errors.Is(err, providers.ErrMalformedResponse) {
		t.Fatalf("video wrong-family error = %v, want ErrMalformedResponse", err)
	}
}

func TestVisualGenerationReferenceBoundsFailureCancellationAndFailedOperation(t *testing.T) {
	tooLarge := &mutableBinaryInput{mime: "image/png", data: []byte("small")}
	tooLargeSize := int64(providers.MaxReferenceImageBytes + 1)
	tooLarge.reportedSize = tooLargeSize
	if err := (providers.ImageGenerationRequest{Prompt: "a", ReferenceImages: []providers.BinaryInput{tooLarge}}).Validate(); !errors.Is(err, providers.ErrInvalidRequest) {
		t.Fatalf("oversized reference error = %v, want ErrInvalidRequest", err)
	}
	if err := (providers.ImageGenerationRequest{Prompt: "a", ReferenceImages: []providers.BinaryInput{&mutableBinaryInput{mime: "application/octet-stream", data: []byte("x")}}}).Validate(); !errors.Is(err, providers.ErrInvalidRequest) {
		t.Fatalf("invalid reference MIME error = %v, want ErrInvalidRequest", err)
	}

	videoGen := fake.NewVideoGenerator([]byte("video")).WithDelay(20 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := videoGen.StartVideo(ctx, providers.VideoGenerationRequest{Prompt: "cancelled"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled video start error = %v, want context.Canceled", err)
	}
	pollGen := fake.NewVideoGenerator([]byte("video"))
	pollOperation, err := pollGen.StartVideo(context.Background(), providers.VideoGenerationRequest{Prompt: "poll cancellation"})
	if err != nil {
		t.Fatalf("start poll-cancellation operation: %v", err)
	}
	pollGen.WithDelay(20 * time.Millisecond)
	pollCtx, pollCancel := context.WithCancel(context.Background())
	pollCancel()
	if _, err := pollGen.GetVideoOperation(pollCtx, pollOperation.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled video poll error = %v, want context.Canceled", err)
	}

	failedGen := fake.NewVideoGenerator([]byte("video")).WithOperationFailure(errors.New("terminal failure"))
	operation, err := failedGen.StartVideo(context.Background(), providers.VideoGenerationRequest{Prompt: "fail"})
	if err != nil {
		t.Fatalf("start failed operation: %v", err)
	}
	if _, err := failedGen.OpenVideoResult(context.Background(), operation.ID); !errors.Is(err, providers.ErrResultUnavailable) {
		t.Fatalf("queued result error = %v, want ErrResultUnavailable", err)
	}
	if _, err := failedGen.GetVideoOperation(context.Background(), operation.ID); err != nil {
		t.Fatalf("poll failed operation running: %v", err)
	}
	failed, err := failedGen.GetVideoOperation(context.Background(), operation.ID)
	if err != nil || failed.State != providers.VideoOperationFailed {
		t.Fatalf("failed operation = %#v, error = %v", failed, err)
	}
	if _, err := failedGen.OpenVideoResult(context.Background(), operation.ID); !errors.Is(err, providers.ErrVideoOperationFailed) {
		t.Fatalf("failed result error = %v, want ErrVideoOperationFailed", err)
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
