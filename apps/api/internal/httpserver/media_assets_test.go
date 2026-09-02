package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenemedia"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

type fakeMediaAssetService struct {
	asset      mediaasset.MediaAsset
	assets     []mediaasset.MediaAsset
	storeInput mediaasset.CreateInput
	storeBody  []byte
	openRange  [2]int64
	deleteErr  error
}

func (f *fakeMediaAssetService) Store(_ context.Context, _ project.Principal, _ uuid.UUID, input mediaasset.CreateInput) (mediaasset.MediaAsset, error) {
	f.storeInput = input
	f.storeBody, _ = io.ReadAll(input.Reader)
	return f.asset, nil
}

func (f *fakeMediaAssetService) Get(_ context.Context, _ project.Principal, _, _ uuid.UUID) (mediaasset.MediaAsset, error) {
	return f.asset, nil
}

func (f *fakeMediaAssetService) List(context.Context, project.Principal, uuid.UUID, int) (mediaasset.ListResult, error) {
	return mediaasset.ListResult{Assets: f.assets}, nil
}

func (f *fakeMediaAssetService) Open(context.Context, project.Principal, uuid.UUID, uuid.UUID) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("full content")), nil
}

func (f *fakeMediaAssetService) OpenRange(_ context.Context, _ project.Principal, _, _ uuid.UUID, offset, length int64) (io.ReadCloser, error) {
	f.openRange = [2]int64{offset, length}
	return io.NopCloser(strings.NewReader("range content")), nil
}

func (f *fakeMediaAssetService) Delete(context.Context, project.Principal, uuid.UUID, uuid.UUID) error {
	return f.deleteErr
}

type fakeSceneMediaService struct {
	current  []scenemedia.CurrentSceneBinding
	history  []scenemedia.Binding
	assigned scenemedia.Binding
}

func (f *fakeSceneMediaService) ListCurrent(context.Context, project.Principal, uuid.UUID, int) ([]scenemedia.CurrentSceneBinding, error) {
	return f.current, nil
}

func (f *fakeSceneMediaService) AssignPrimaryVisual(context.Context, project.Principal, uuid.UUID, int, string, uuid.UUID) (scenemedia.Binding, error) {
	return f.assigned, nil
}

func (f *fakeSceneMediaService) ListHistory(context.Context, project.Principal, uuid.UUID, int, string) ([]scenemedia.Binding, error) {
	return f.history, nil
}

type fakeMediaActorResolver struct{ principal project.Principal }

func (r fakeMediaActorResolver) Resolve(*http.Request) (project.Principal, error) {
	return r.principal, nil
}

func newTestMediaServer(assets MediaAssetService, bindings SceneMediaService, resolver actorResolver) *http.Server {
	return New(
		config.Config{Addr: ":0", Environment: "test", MediaStorage: config.MediaStorageConfig{MaxUploadBytes: 1024}},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, resolver,
		MediaServices{Assets: assets, Bindings: bindings},
	)
}

type actorResolver interface {
	Resolve(*http.Request) (project.Principal, error)
}

func TestMediaAssetUploadDerivesKindAndOriginAndStreamsBody(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	asset := mediaasset.MediaAsset{ID: uuid.New(), OwnerID: ownerID, ProjectID: projectID, Kind: mediaasset.KindImage, Origin: mediaasset.OriginUpload}
	assets := &fakeMediaAssetService{asset: asset}
	server := newTestMediaServer(assets, nil, fakeMediaActorResolver{principal: project.Principal{OwnerID: ownerID}})

	var body bytes.Buffer
	writer := multipartWriter(t, &body)
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="file"; filename="../cover.png"`},
		"Content-Type":        {"image/png"},
	})
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	_, _ = part.Write([]byte("png bytes"))
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/media-assets", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if assets.storeInput.Kind != mediaasset.KindImage || assets.storeInput.Origin != mediaasset.OriginUpload {
		t.Fatalf("handler did not derive upload type: %+v", assets.storeInput)
	}
	if assets.storeInput.OriginalFilename != "cover.png" {
		t.Fatalf("expected display filename to be basename, got %q", assets.storeInput.OriginalFilename)
	}
	if string(assets.storeBody) != "png bytes" {
		t.Fatalf("handler did not pass streaming file body: %q", assets.storeBody)
	}
}

func TestMediaAssetContentSupportsSingleByteRange(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	assetID := uuid.New()
	assets := &fakeMediaAssetService{asset: mediaasset.MediaAsset{
		ID: assetID, OwnerID: ownerID, ProjectID: projectID, Kind: mediaasset.KindVideo,
		MimeType: "video/mp4", ByteSize: 20, OriginalFilename: "clip.mp4",
	}}
	server := newTestMediaServer(assets, nil, fakeMediaActorResolver{principal: project.Principal{OwnerID: ownerID}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/media-assets/"+assetID.String()+"/content", nil)
	req.Header.Set("Range", "bytes=5-9")
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent || rec.Header().Get("Content-Range") != "bytes 5-9/20" {
		t.Fatalf("unexpected range response: status=%d headers=%v body=%s", rec.Code, rec.Header(), rec.Body.String())
	}
	if rec.Header().Get("Content-Length") != "5" || rec.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("missing range headers: %v", rec.Header())
	}
	if assets.openRange != [2]int64{5, 5} {
		t.Fatalf("unexpected storage range: %v", assets.openRange)
	}
}

func TestMediaAssetContentRejectsMalformedOrUnsatisfiableRanges(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	assetID := uuid.New()
	assets := &fakeMediaAssetService{asset: mediaasset.MediaAsset{ID: assetID, OwnerID: ownerID, ProjectID: projectID, MimeType: "video/mp4", ByteSize: 20}}
	server := newTestMediaServer(assets, nil, fakeMediaActorResolver{principal: project.Principal{OwnerID: ownerID}})
	for _, raw := range []string{"bytes=1-2,4-5", "bytes=20-21", "bytes=bad"} {
		t.Run(raw, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/media-assets/"+assetID.String()+"/content", nil)
			req.Header.Set("Range", raw)
			rec := httptest.NewRecorder()
			server.Handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusRequestedRangeNotSatisfiable || rec.Header().Get("Content-Range") != "bytes */20" {
				t.Fatalf("unexpected invalid range response: status=%d headers=%v", rec.Code, rec.Header())
			}
		})
	}
}

func TestSceneMediaBindingEndpointsIncludeSafeAssetMetadataAndUnboundScenes(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	assetID := uuid.New()
	now := time.Now().UTC()
	asset := mediaasset.MediaAsset{ID: assetID, OwnerID: ownerID, ProjectID: projectID, Kind: mediaasset.KindImage, Origin: mediaasset.OriginUpload, MimeType: "image/png", ByteSize: 3, SHA256: strings.Repeat("a", 64), CreatedAt: now, UpdatedAt: now, Metadata: json.RawMessage(`{"width":1}`)}
	binding := scenemedia.Binding{ID: uuid.New(), OwnerID: ownerID, ProjectID: projectID, ScenePlanVersion: 2, SceneKey: "intro", Role: scenemedia.RolePrimaryVisual, BindingVersion: 1, AssetID: assetID, Status: scenemedia.StatusActive, CreatedAt: now}
	assets := &fakeMediaAssetService{asset: asset}
	bindings := &fakeSceneMediaService{current: []scenemedia.CurrentSceneBinding{
		{Scene: sceneplan.Scene{Key: "intro"}, Binding: &binding},
		{Scene: sceneplan.Scene{Key: "outro"}},
	}}
	server := newTestMediaServer(assets, bindings, fakeMediaActorResolver{principal: project.Principal{OwnerID: ownerID}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/scene-plans/2/media-bindings", nil)
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response []sceneMediaBindingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response) != 2 || response[0].SceneKey != "intro" || response[1].SceneKey != "outro" || response[1].Asset != nil {
		t.Fatalf("unexpected binding response: %+v", response)
	}
	if response[0].Asset == nil || response[0].Asset.ID != assetID || response[0].Asset.OwnerID != uuid.Nil {
		t.Fatalf("unsafe or missing asset metadata: %+v", response[0].Asset)
	}
}

func multipartWriter(t *testing.T, body *bytes.Buffer) *multipart.Writer {
	t.Helper()
	return multipart.NewWriter(body)
}
