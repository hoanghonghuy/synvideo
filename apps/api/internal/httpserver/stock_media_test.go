package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/stockmedia"
)

type fakeStockMediaService struct {
	searchRequest stockmedia.SearchRequest
	providerKey   string
	acquireInput  stockmedia.AcquireInput
	searchPage    stockmedia.SearchPage
	acquisition   stockmedia.Acquisition
	searchErr     error
	acquireErr    error
}

func (f *fakeStockMediaService) Search(_ context.Context, _ project.Principal, _ uuid.UUID, providerKey string, request stockmedia.SearchRequest) (stockmedia.SearchPage, error) {
	f.providerKey = providerKey
	f.searchRequest = request
	return f.searchPage, f.searchErr
}

func (f *fakeStockMediaService) Acquire(_ context.Context, _ project.Principal, _ uuid.UUID, input stockmedia.AcquireInput) (stockmedia.Acquisition, error) {
	f.acquireInput = input
	return f.acquisition, f.acquireErr
}

func TestStockMediaSearchForwardsBoundedExplicitInputs(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	service := &fakeStockMediaService{searchPage: stockmedia.SearchPage{Page: 2, PerPage: 10}}
	handler := stockMediaHandler{service: service, actorResolver: fakeMediaActorResolver{principal: project.Principal{OwnerID: ownerID}}}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/stock-media/search?provider=pexels&q=rainy+city&kind=image&orientation=landscape&page=2&per_page=10", nil)
	req.SetPathValue("id", projectID.String())
	rec := httptest.NewRecorder()
	handler.search(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if service.providerKey != "pexels" || service.searchRequest.Query != "rainy city" || service.searchRequest.Kind != stockmedia.MediaKindImage || service.searchRequest.Orientation != stockmedia.OrientationLandscape || service.searchRequest.Page != 2 || service.searchRequest.PerPage != 10 {
		t.Fatalf("unexpected forwarded search: provider=%q request=%#v", service.providerKey, service.searchRequest)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["per_page"]; !ok {
		t.Fatalf("search response must use stable snake_case JSON: %s", rec.Body.String())
	}
}

func TestStockMediaAcquireRejectsTrailingJSONBeforeService(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	service := &fakeStockMediaService{}
	handler := stockMediaHandler{service: service, actorResolver: fakeMediaActorResolver{principal: project.Principal{OwnerID: ownerID}}}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/stock-media/acquisitions", strings.NewReader(`{"provider_key":"pexels","provider_result_id":"123","kind":"image"} {}`))
	req.SetPathValue("id", projectID.String())
	rec := httptest.NewRecorder()
	handler.acquire(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if service.acquireInput.ProviderResultID != "" {
		t.Fatalf("service must not receive malformed request: %#v", service.acquireInput)
	}
}

func TestStockMediaAcquireReturnsDurableAssetAndReuseState(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	asset := mediaasset.MediaAsset{ID: uuid.New(), OwnerID: ownerID, ProjectID: projectID, Kind: mediaasset.KindImage, Origin: mediaasset.OriginStock}
	service := &fakeStockMediaService{acquisition: stockmedia.Acquisition{Asset: asset, Reused: true}}
	handler := stockMediaHandler{service: service, actorResolver: fakeMediaActorResolver{principal: project.Principal{OwnerID: ownerID}}}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/stock-media/acquisitions", strings.NewReader(`{"provider_key":"pexels","provider_result_id":"123","kind":"image"}`))
	req.SetPathValue("id", projectID.String())
	rec := httptest.NewRecorder()
	handler.acquire(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Asset  mediaasset.MediaAsset `json:"asset"`
		Reused bool                  `json:"reused"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Asset.ID != asset.ID || !body.Reused {
		t.Fatalf("response = %#v", body)
	}
}

func TestStockMediaProviderRateLimitIsExplicit(t *testing.T) {
	service := &fakeStockMediaService{searchErr: stockmedia.ProviderError{Kind: stockmedia.ProviderErrorRateLimited, Provider: "pexels", RetryAfter: "30"}}
	ownerID := uuid.New()
	projectID := uuid.New()
	handler := stockMediaHandler{service: service, actorResolver: fakeMediaActorResolver{principal: project.Principal{OwnerID: ownerID}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/stock-media/search?provider=pexels&q=city&kind=image", nil)
	req.SetPathValue("id", projectID.String())
	rec := httptest.NewRecorder()
	handler.search(rec, req)

	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "30" {
		t.Fatalf("status=%d retry-after=%q body=%s", rec.Code, rec.Header().Get("Retry-After"), rec.Body.String())
	}
}
