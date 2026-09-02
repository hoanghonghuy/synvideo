package generatedimagejob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenemedia"
)

func TestHandlerReusesDurablyIngestedAssetWithoutRegeneration(t *testing.T) {
	ownerID, projectID, jobID, assetID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	payload, _ := json.Marshal(Payload{SchemaVersion: SchemaVersion, ProviderID: "openai", ModelID: "image-1", ScenePlanVersion: 1, SceneKey: "scene-1", Prompt: "lighthouse", AssignPrimaryVisual: true})
	asset := mediaasset.MediaAsset{ID: assetID, OwnerID: ownerID, ProjectID: projectID, Kind: mediaasset.KindImage, Origin: mediaasset.OriginGeneratedImage}
	store := &fakeGeneratedAssetStore{existing: &asset}
	runtime := &countingRuntime{generator: &countingImageGenerator{}}
	binder := &fakeBinder{currentErr: scenemedia.ErrNotFound}
	h := NewHandler(runtime, store, binder)

	resultRaw, err := h.Handle(context.Background(), jobs.Job{ID: jobID, OwnerID: ownerID, ProjectID: &projectID, Kind: JobKind, Payload: payload})
	if err != nil { t.Fatalf("handle: %v", err) }
	if runtime.resolveCalls != 0 { t.Fatalf("recovery regenerated image") }
	if store.storeCalls != 0 { t.Fatalf("recovery duplicated media asset") }
	if binder.assignCalls != 1 { t.Fatalf("expected one assignment, got %d", binder.assignCalls) }
	var result Result
	if err := json.Unmarshal(resultRaw, &result); err != nil { t.Fatal(err) }
	if result.MediaAssetID != assetID || !result.AssignedPrimaryVisual { t.Fatalf("unexpected result: %+v", result) }
}

func TestHandlerDuplicateDeliveryDoesNotDuplicateCurrentAssignment(t *testing.T) {
	ownerID, projectID, jobID, assetID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	payload, _ := json.Marshal(Payload{SchemaVersion: SchemaVersion, ProviderID: "openai", ModelID: "image-1", ScenePlanVersion: 1, SceneKey: "scene-1", Prompt: "lighthouse", AssignPrimaryVisual: true})
	asset := mediaasset.MediaAsset{ID: assetID, OwnerID: ownerID, ProjectID: projectID, Kind: mediaasset.KindImage, Origin: mediaasset.OriginGeneratedImage}
	binder := &fakeBinder{current: scenemedia.Binding{AssetID: assetID}}
	h := NewHandler(&countingRuntime{}, &fakeGeneratedAssetStore{existing: &asset}, binder)
	_, err := h.Handle(context.Background(), jobs.Job{ID: jobID, OwnerID: ownerID, ProjectID: &projectID, Kind: JobKind, Payload: payload})
	if err != nil { t.Fatalf("handle: %v", err) }
	if binder.assignCalls != 0 { t.Fatalf("duplicate delivery wrote assignment history") }
}

func TestHandlerGeneratesAndStoresSafeProvenance(t *testing.T) {
	ownerID, projectID, jobID := uuid.New(), uuid.New(), uuid.New()
	binary, err := providers.NewGeneratedBinary("image/png", []byte("png-data"))
	if err != nil { t.Fatal(err) }
	gen := &countingImageGenerator{response: providers.ImageGenerationResponse{Outputs: []providers.GeneratedImage{{Binary: binary}}}}
	runtime := &countingRuntime{generator: gen}
	store := &fakeGeneratedAssetStore{}
	payload, _ := json.Marshal(Payload{SchemaVersion: SchemaVersion, ProviderID: "openai", ModelID: "image-1", ScenePlanVersion: 1, SceneKey: "scene-1", Prompt: "lighthouse", AspectRatio: "16:9"})
	_, err = NewHandler(runtime, store, nil).Handle(context.Background(), jobs.Job{ID: jobID, OwnerID: ownerID, ProjectID: &projectID, Kind: JobKind, Payload: payload})
	if err != nil { t.Fatalf("handle: %v", err) }
	if gen.calls != 1 || store.storeCalls != 1 { t.Fatalf("generation/store calls = %d/%d", gen.calls, store.storeCalls) }
	var metadata map[string]any
	if err := json.Unmarshal(store.lastInput.Metadata, &metadata); err != nil { t.Fatal(err) }
	if metadata["job_id"] != jobID.String() || metadata["provider_id"] != "openai" || metadata["model_id"] != "image-1" { t.Fatalf("unsafe/missing provenance: %v", metadata) }
	for _, forbidden := range []string{"provider_url", "base_url", "credential", "external_model_id", "raw_response"} {
		if _, ok := metadata[forbidden]; ok { t.Fatalf("forbidden provenance key %q", forbidden) }
	}
}

type fakeGeneratedAssetStore struct {
	existing *mediaasset.MediaAsset
	storeCalls int
	lastInput mediaasset.CreateInput
}
func (f *fakeGeneratedAssetStore) FindGeneratedByJob(context.Context, project.Principal, uuid.UUID, uuid.UUID) (mediaasset.MediaAsset, error) {
	if f.existing == nil { return mediaasset.MediaAsset{}, mediaasset.ErrNotFound }
	return *f.existing, nil
}
func (f *fakeGeneratedAssetStore) Store(_ context.Context, principal project.Principal, projectID uuid.UUID, input mediaasset.CreateInput) (mediaasset.MediaAsset, error) {
	f.storeCalls++; f.lastInput = input
	data, err := io.ReadAll(input.Reader); if err != nil { return mediaasset.MediaAsset{}, err }
	if len(data) == 0 { return mediaasset.MediaAsset{}, errors.New("empty") }
	asset := mediaasset.MediaAsset{ID: uuid.New(), OwnerID: principal.OwnerID, ProjectID: projectID, Kind: input.Kind, Origin: input.Origin, MimeType: input.MimeType, Metadata: append(json.RawMessage(nil), input.Metadata...)}
	f.existing = &asset
	return asset, nil
}

type countingRuntime struct { generator providers.ImageGenerator; resolveCalls int }
func (f *countingRuntime) ResolveImageGenerator(context.Context, uuid.UUID, providers.ProviderID, providers.ModelID) (providers.ImageGenerator, error) {
	f.resolveCalls++
	if f.generator == nil { return nil, providers.ErrProviderUnavailable }
	return f.generator, nil
}

type countingImageGenerator struct { calls int; response providers.ImageGenerationResponse; err error }
func (f *countingImageGenerator) GenerateImage(context.Context, providers.ImageGenerationRequest) (providers.ImageGenerationResponse, error) { f.calls++; return f.response, f.err }

type fakeBinder struct { current scenemedia.Binding; currentErr error; assignCalls int }
func (f *fakeBinder) GetCurrent(context.Context, project.Principal, uuid.UUID, int, string) (scenemedia.Binding, error) { return f.current, f.currentErr }
func (f *fakeBinder) AssignPrimaryVisual(_ context.Context, _ project.Principal, _ uuid.UUID, _ int, _ string, assetID uuid.UUID) (scenemedia.Binding, error) { f.assignCalls++; f.current = scenemedia.Binding{AssetID: assetID}; f.currentErr = nil; return f.current, nil }

var _ = bytes.NewReader
