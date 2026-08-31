package providers_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/fake"
)

func TestRegistryRegistersAndResolvesTextGenerator(t *testing.T) {
	registry := providers.NewRegistry()
	textGen := fake.NewTextGenerator("hello from fake")

	err := registry.Register(providers.Registration{
		Provider: providers.ProviderMetadata{
			ID:          "synvideo-lab",
			DisplayName: "SynVideo Lab",
		},
		Models: []providers.ModelRegistration{
			{
				Metadata: providers.ModelMetadata{
					ProviderID:            "synvideo-lab",
					ID:                    "lab-text-v1",
					DisplayName:           "Lab Text V1",
					SupportedCapabilities: []providers.Capability{providers.CapabilityTextGeneration},
				},
				TextGenerator: textGen,
			},
		},
	})
	if err != nil {
		t.Fatalf("register provider: %v", err)
	}

	resolved, metadata, err := registry.ResolveTextGenerator("synvideo-lab", "lab-text-v1")
	if err != nil {
		t.Fatalf("resolve text generator: %v", err)
	}
	if resolved == nil {
		t.Fatal("expected text generator")
	}
	if metadata.ID != "lab-text-v1" {
		t.Fatalf("metadata id = %q, want lab-text-v1", metadata.ID)
	}

	resp, err := resolved.GenerateText(context.Background(), providers.TextGenerationRequest{
		ProviderID: "synvideo-lab",
		ModelID:    "lab-text-v1",
		Messages: []providers.TextMessage{
			{Role: "user", Content: "draft proposal"},
		},
	})
	if err != nil {
		t.Fatalf("generate text: %v", err)
	}
	if resp.Text != "hello from fake" {
		t.Fatalf("text = %q, want hello from fake", resp.Text)
	}
	if resp.Usage.InputTokens == nil || *resp.Usage.InputTokens != 2 {
		t.Fatalf("usage input tokens = %#v, want 2", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens == nil || *resp.Usage.OutputTokens != 3 {
		t.Fatalf("usage output tokens = %#v, want 3", resp.Usage.OutputTokens)
	}
}

func TestRegistryRejectsDuplicateConflictingRegistration(t *testing.T) {
	registry := providers.NewRegistry()
	registration := providers.Registration{
		Provider: providers.ProviderMetadata{
			ID:          "synvideo-lab",
			DisplayName: "SynVideo Lab",
		},
		Models: []providers.ModelRegistration{
			{
				Metadata: providers.ModelMetadata{
					ProviderID:            "synvideo-lab",
					ID:                    "lab-text-v1",
					DisplayName:           "Lab Text V1",
					SupportedCapabilities: []providers.Capability{providers.CapabilityTextGeneration},
				},
				TextGenerator: fake.NewTextGenerator("first"),
			},
		},
	}
	if err := registry.Register(registration); err != nil {
		t.Fatalf("first register: %v", err)
	}

	registration.Models[0].TextGenerator = fake.NewTextGenerator("second")
	if err := registry.Register(registration); !errors.Is(err, providers.ErrDuplicateRegistration) {
		t.Fatalf("duplicate register error = %v, want ErrDuplicateRegistration", err)
	}
}

func TestRegistryReturnsUnknownProviderError(t *testing.T) {
	registry := providers.NewRegistry()
	_, _, err := registry.ResolveTextGenerator("missing-provider", "any-model")
	if !errors.Is(err, providers.ErrUnknownProvider) {
		t.Fatalf("error = %v, want ErrUnknownProvider", err)
	}
}

func TestRegistryReturnsUnknownModelDistinctFromUnsupportedCapability(t *testing.T) {
	registry := providers.NewRegistry()
	if err := registry.Register(providers.Registration{
		Provider: providers.ProviderMetadata{
			ID:          "synvideo-lab",
			DisplayName: "SynVideo Lab",
		},
		Models: []providers.ModelRegistration{
			{
				Metadata: providers.ModelMetadata{
					ProviderID:            "synvideo-lab",
					ID:                    "image-only-v1",
					DisplayName:           "Image Only V1",
					SupportedCapabilities: []providers.Capability{providers.CapabilityImageGeneration},
				},
			},
		},
	}); err != nil {
		t.Fatalf("register provider: %v", err)
	}

	_, _, err := registry.ResolveTextGenerator("synvideo-lab", "missing-model")
	if !errors.Is(err, providers.ErrUnknownModel) {
		t.Fatalf("unknown model error = %v, want ErrUnknownModel", err)
	}

	_, _, err = registry.ResolveTextGenerator("synvideo-lab", "image-only-v1")
	if !errors.Is(err, providers.ErrUnsupportedCapability) {
		t.Fatalf("unsupported capability error = %v, want ErrUnsupportedCapability", err)
	}
}

func TestFakeTextGeneratorRecordsProviderNeutralRequest(t *testing.T) {
	textGen := fake.NewTextGenerator("deterministic output")
	req := providers.TextGenerationRequest{
		ProviderID: "synvideo-lab",
		ModelID:    "lab-text-v1",
		Messages: []providers.TextMessage{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Write a hook."},
		},
	}

	resp, err := textGen.GenerateText(context.Background(), req)
	if err != nil {
		t.Fatalf("generate text: %v", err)
	}
	if resp.Text != "deterministic output" {
		t.Fatalf("text = %q", resp.Text)
	}
	if len(textGen.Requests()) != 1 {
		t.Fatalf("recorded requests = %d, want 1", len(textGen.Requests()))
	}
	if textGen.Requests()[0].Messages[1].Content != "Write a hook." {
		t.Fatalf("recorded request = %#v", textGen.Requests()[0])
	}
}

func TestTextGenerationPropagatesContextCancellation(t *testing.T) {
	textGen := fake.NewTextGenerator("ignored").WithDelay(200 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := textGen.GenerateText(ctx, providers.TextGenerationRequest{
		ProviderID: "synvideo-lab",
		ModelID:    "lab-text-v1",
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestProviderExecutionErrorDoesNotLeakSecretInPresentationMessage(t *testing.T) {
	rawErr := errors.New("provider failed: api_key=super-secret-value")
	providerErr := providers.NewExecutionError("Text generation failed.", rawErr)

	boundaryErr, ok := providerErr.(*providers.BoundaryError)
	if !ok {
		t.Fatalf("expected BoundaryError, got %T", providerErr)
	}
	if strings.Contains(boundaryErr.PresentationMessage(), "super-secret-value") {
		t.Fatalf("presentation message leaked secret: %q", boundaryErr.PresentationMessage())
	}
	if !errors.Is(providerErr, providers.ErrProviderExecution) {
		t.Fatalf("error = %v, want ErrProviderExecution", providerErr)
	}
	if !errors.Is(providerErr, rawErr) {
		t.Fatal("expected wrapped raw cause for internal diagnostics")
	}
}

func TestRegistryConcurrentRegistrationIsSafe(t *testing.T) {
	registry := providers.NewRegistry()
	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	register := func(suffix string) {
		defer wg.Done()
		err := registry.Register(providers.Registration{
			Provider: providers.ProviderMetadata{
				ID:          providers.ProviderID("provider-" + suffix),
				DisplayName: "Provider " + suffix,
			},
			Models: []providers.ModelRegistration{
				{
					Metadata: providers.ModelMetadata{
						ProviderID:            providers.ProviderID("provider-" + suffix),
						ID:                    providers.ModelID("model-" + suffix),
						DisplayName:           "Model " + suffix,
						SupportedCapabilities: []providers.Capability{providers.CapabilityTextGeneration},
					},
					TextGenerator: fake.NewTextGenerator("ok-" + suffix),
				},
			},
		})
		errCh <- err
	}

	wg.Add(2)
	go register("a")
	go register("b")
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent register failed: %v", err)
		}
	}
}

func TestRegistryListsProvidersAndModelsDeterministically(t *testing.T) {
	registry := providers.NewRegistry()
	for _, id := range []providers.ProviderID{"provider-b", "provider-a"} {
		if err := registry.Register(providers.Registration{
			Provider: providers.ProviderMetadata{
				ID:          id,
				DisplayName: id.String(),
			},
			Models: []providers.ModelRegistration{
				{
					Metadata: providers.ModelMetadata{
						ProviderID:            id,
						ID:                    "model-b",
						DisplayName:           "Model B",
						SupportedCapabilities: []providers.Capability{providers.CapabilityTextGeneration},
					},
					TextGenerator: fake.NewTextGenerator("b"),
				},
				{
					Metadata: providers.ModelMetadata{
						ProviderID:            id,
						ID:                    "model-a",
						DisplayName:           "Model A",
						SupportedCapabilities: []providers.Capability{providers.CapabilityTextGeneration},
					},
					TextGenerator: fake.NewTextGenerator("a"),
				},
			},
		}); err != nil {
			t.Fatalf("register %s: %v", id, err)
		}
	}

	providersList := registry.ListProviders()
	if got := []providers.ProviderID{providersList[0].ID, providersList[1].ID}; got[0] != "provider-a" || got[1] != "provider-b" {
		t.Fatalf("providers not sorted by id: %#v", got)
	}

	models, err := registry.ListModels("provider-a")
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	if got := []providers.ModelID{models[0].ID, models[1].ID}; got[0] != "model-a" || got[1] != "model-b" {
		t.Fatalf("models not sorted by id: %#v", got)
	}
}
