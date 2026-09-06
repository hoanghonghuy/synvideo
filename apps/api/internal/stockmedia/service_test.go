package stockmedia

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

func TestAcquireStoresAuthoritativeProvenanceAndReusesDurableIdentity(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	principal := project.Principal{OwnerID: ownerID}
	provider := &fakeProvider{source: AcquisitionSource{
		Result: SearchResult{
			ProviderKey:      "pexels",
			ProviderResultID: "123",
			Kind:             MediaKindImage,
			PreviewURL:       "https://preview.example/123.jpg",
			SourcePageURL:    "https://www.pexels.com/photo/123/",
			CreatorName:      "Ada",
			CreatorURL:       "https://www.pexels.com/@ada",
			LicenseSummary:   "Pexels License",
			LicenseReference: "https://www.pexels.com/license/",
			AttributionText:  "Content by Ada on Pexels",
			Acquirable:       true,
		},
		Filename: "pexels-123.jpg",
		Remote:   staticRemote{contentType: "image/jpeg", payload: []byte("image")},
	}}
	assets := &fakeAssetStore{}
	service, err := NewService(fakeProjectAccess{ownerID: ownerID, projectID: projectID}, assets, map[string]Provider{"pexels": provider}, 1024)
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.Acquire(context.Background(), principal, projectID, AcquireInput{ProviderKey: "pexels", ProviderResultID: "123", Kind: MediaKindImage})
	if err != nil {
		t.Fatal(err)
	}
	if first.Reused || assets.storeCalls != 1 || provider.resolveCalls != 1 {
		t.Fatalf("first acquisition = %#v storeCalls=%d resolveCalls=%d", first, assets.storeCalls, provider.resolveCalls)
	}
	var metadata map[string]any
	if err := json.Unmarshal(assets.lastInput.Metadata, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["stock_provider"] != "pexels" || metadata["stock_result_id"] != "123" || metadata["stock_creator_name"] != "Ada" {
		t.Fatalf("metadata = %#v", metadata)
	}

	second, err := service.Acquire(context.Background(), principal, projectID, AcquireInput{ProviderKey: "pexels", ProviderResultID: "123", Kind: MediaKindImage})
	if err != nil {
		t.Fatal(err)
	}
	if !second.Reused || second.Asset.ID != first.Asset.ID {
		t.Fatalf("second acquisition = %#v, first = %#v", second, first)
	}
	if assets.storeCalls != 1 || provider.resolveCalls != 1 {
		t.Fatalf("retry performed duplicate work: storeCalls=%d resolveCalls=%d", assets.storeCalls, provider.resolveCalls)
	}
}

func TestAcquireRejectsProviderIdentityMismatch(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	provider := &fakeProvider{source: AcquisitionSource{
		Result: SearchResult{ProviderKey: "other", ProviderResultID: "123", Kind: MediaKindImage, PreviewURL: "https://preview.example/x", LicenseSummary: "license", Acquirable: true},
		Remote: staticRemote{contentType: "image/jpeg", payload: []byte("image")},
	}}
	assets := &fakeAssetStore{}
	service, err := NewService(fakeProjectAccess{ownerID: ownerID, projectID: projectID}, assets, map[string]Provider{"pexels": provider}, 1024)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Acquire(context.Background(), project.Principal{OwnerID: ownerID}, projectID, AcquireInput{ProviderKey: "pexels", ProviderResultID: "123", Kind: MediaKindImage})
	providerErr, ok := err.(ProviderError)
	if !ok || providerErr.Kind != ProviderErrorTransient {
		t.Fatalf("error = %#v", err)
	}
	if assets.storeCalls != 0 {
		t.Fatal("identity mismatch must not persist an asset")
	}
}

type fakeProjectAccess struct {
	ownerID   uuid.UUID
	projectID uuid.UUID
}

func (f fakeProjectAccess) Get(_ context.Context, ownerID, projectID uuid.UUID) (project.Project, error) {
	if ownerID != f.ownerID || projectID != f.projectID {
		return project.Project{}, project.ErrNotFound
	}
	return project.Project{ID: projectID, OwnerID: ownerID}, nil
}

type fakeProvider struct {
	source       AcquisitionSource
	resolveCalls int
}

func (f *fakeProvider) Search(_ context.Context, request SearchRequest) (SearchPage, error) {
	return SearchPage{Page: request.Page, PerPage: request.PerPage}, nil
}

func (f *fakeProvider) ResolveForAcquisition(_ context.Context, _ string, _ MediaKind) (AcquisitionSource, error) {
	f.resolveCalls++
	return f.source, nil
}

type staticRemote struct {
	contentType string
	payload     []byte
}

func (r staticRemote) ContentType() string { return r.contentType }
func (r staticRemote) Open(context.Context) (ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(r.payload)), nil
}

type fakeAssetStore struct {
	asset      *mediaasset.MediaAsset
	storeCalls int
	lastInput  mediaasset.CreateInput
}

func (f *fakeAssetStore) FindStockOrigin(_ context.Context, _ project.Principal, projectID uuid.UUID, providerKey, resultID string, kind mediaasset.Kind) (mediaasset.MediaAsset, error) {
	if f.asset == nil {
		return mediaasset.MediaAsset{}, mediaasset.ErrNotFound
	}
	return *f.asset, nil
}

func (f *fakeAssetStore) Store(_ context.Context, principal project.Principal, projectID uuid.UUID, input mediaasset.CreateInput) (mediaasset.MediaAsset, error) {
	f.storeCalls++
	f.lastInput = input
	asset := mediaasset.MediaAsset{ID: uuid.New(), OwnerID: principal.OwnerID, ProjectID: projectID, Kind: input.Kind, Origin: input.Origin, Metadata: append([]byte(nil), input.Metadata...)}
	f.asset = &asset
	return asset, nil
}
