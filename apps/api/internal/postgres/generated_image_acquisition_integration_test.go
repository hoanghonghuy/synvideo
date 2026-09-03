package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/generatedimagejob"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset/s3storage"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/fake"
)

func TestGeneratedImageAcquisitionIntegrationPersistsAndRecovers(t *testing.T) {
	pool := integrationPool(t)
	storage := integrationStorage(t)
	projectRepository := NewProjectRepository(pool)
	assetRepository := NewMediaAssetRepository(pool)
	assetService := mediaasset.NewService(projectRepository, assetRepository, storage)
	assetStore := generatedimagejob.NewAssetStore(assetService, assetRepository)

	ownerA := uuid.New()
	ownerB := uuid.New()
	projectA, err := projectRepository.Create(context.Background(), ownerA, validIntegrationCreateInput("Generated image recovery"))
	if err != nil {
		t.Fatalf("create owner A project: %v", err)
	}
	projectB, err := projectRepository.Create(context.Background(), ownerB, validIntegrationCreateInput("Foreign project"))
	if err != nil {
		t.Fatalf("create owner B project: %v", err)
	}

	generator := fake.NewImageGenerator([]byte("durable generated image"))
	runtime := integrationImageRuntime{generator: generator}
	handler := generatedimagejob.NewHandler(runtime, assetStore, nil)
	jobID := uuid.New()
	payload, err := json.Marshal(generatedimagejob.Payload{
		SchemaVersion:       generatedimagejob.SchemaVersion,
		ProviderID:          "openai",
		ModelID:             "image-1",
		ScenePlanVersion:    1,
		SceneKey:            "intro",
		Prompt:              "a lighthouse at sunrise",
		AssignPrimaryVisual: false,
	})
	if err != nil {
		t.Fatalf("marshal job payload: %v", err)
	}
	job := jobs.Job{ID: jobID, OwnerID: ownerA, ProjectID: &projectA.ID, Kind: generatedimagejob.JobKind, Payload: payload}

	resultRaw, err := handler.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("first delivery: %v", err)
	}
	var result generatedimagejob.Result
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		t.Fatalf("decode first result: %v", err)
	}
	if result.MediaAssetID == uuid.Nil {
		t.Fatal("first delivery did not return a durable asset")
	}
	if len(generator.Requests()) != 1 {
		t.Fatalf("expected one provider call, got %d", len(generator.Requests()))
	}

	asset, err := assetService.Get(context.Background(), project.Principal{OwnerID: ownerA}, projectA.ID, result.MediaAssetID)
	if err != nil {
		t.Fatalf("get persisted asset: %v", err)
	}
	if asset.Origin != mediaasset.OriginGeneratedImage || asset.ByteSize != int64(len("durable generated image")) {
		t.Fatalf("unexpected persisted asset: %+v", asset)
	}
	reader, err := assetService.Open(context.Background(), project.Principal{OwnerID: ownerA}, projectA.ID, asset.ID)
	if err != nil {
		t.Fatalf("open persisted object: %v", err)
	}
	content, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil || string(content) != "durable generated image" {
		t.Fatalf("unexpected persisted object: content=%q read_err=%v close_err=%v", content, readErr, closeErr)
	}
	t.Cleanup(func() { _ = storage.Delete(context.Background(), asset.ObjectKey) })

	var metadata map[string]any
	if err := json.Unmarshal(asset.Metadata, &metadata); err != nil {
		t.Fatalf("decode persisted provenance: %v", err)
	}
	if metadata["job_id"] != jobID.String() || metadata["provider_id"] != "openai" || metadata["model_id"] != "image-1" {
		t.Fatalf("unexpected persisted provenance: %v", metadata)
	}
	for _, forbidden := range []string{"provider_url", "base_url", "credential", "ciphertext", "external_model_id", "raw_response"} {
		if _, ok := metadata[forbidden]; ok {
			t.Fatalf("forbidden provenance key %q persisted", forbidden)
		}
	}

	// A retry after the generation -> ingestion boundary must recover the asset
	// from the real repository without paying for generation again.
	retryRaw, err := handler.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("post-ingest retry: %v", err)
	}
	var retryResult generatedimagejob.Result
	if err := json.Unmarshal(retryRaw, &retryResult); err != nil {
		t.Fatalf("decode retry result: %v", err)
	}
	if retryResult.MediaAssetID != result.MediaAssetID || len(generator.Requests()) != 1 {
		t.Fatalf("retry regenerated or changed asset: first=%s retry=%s provider_calls=%d", result.MediaAssetID, retryResult.MediaAssetID, len(generator.Requests()))
	}
	assets, err := assetRepository.List(context.Background(), ownerA, projectA.ID, mediaasset.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("list persisted assets: %v", err)
	}
	if len(assets.Assets) != 1 {
		t.Fatalf("retry created duplicate media assets: %+v", assets.Assets)
	}

	if _, err := assetStore.FindGeneratedByJob(context.Background(), project.Principal{OwnerID: ownerB}, projectA.ID, jobID); !errors.Is(err, mediaasset.ErrNotFound) {
		t.Fatalf("cross-owner lookup recovered generated asset: %v", err)
	}
	if _, err := assetStore.FindGeneratedByJob(context.Background(), project.Principal{OwnerID: ownerA}, projectB.ID, jobID); !errors.Is(err, mediaasset.ErrNotFound) {
		t.Fatalf("cross-project lookup recovered generated asset: %v", err)
	}
}

type integrationImageRuntime struct {
	generator providers.ImageGenerator
}

func (r integrationImageRuntime) ResolveImageGenerator(context.Context, uuid.UUID, providers.ProviderID, providers.ModelID) (providers.ImageGenerator, error) {
	return r.generator, nil
}

func integrationStorage(t *testing.T) *s3storage.Storage {
	t.Helper()
	endpoint := integrationEnv("SYNVIDEO_MEDIA_STORAGE_ENDPOINT", "SYNVIDEO_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("SYNVIDEO_MEDIA_STORAGE_ENDPOINT is not set; start local SeaweedFS/MinIO for this integration test")
	}
	cfg := s3storage.Config{
		Endpoint:        endpoint,
		Region:          integrationEnvOr("SYNVIDEO_MEDIA_STORAGE_REGION", "SYNVIDEO_S3_REGION", "local"),
		Bucket:          integrationEnvOr("SYNVIDEO_MEDIA_STORAGE_BUCKET", "SYNVIDEO_S3_BUCKET", "synvideo-local"),
		AccessKeyID:     integrationEnvOr("SYNVIDEO_MEDIA_STORAGE_ACCESS_KEY_ID", "SYNVIDEO_S3_ACCESS_KEY_ID", "synvideo"),
		SecretAccessKey: integrationEnvOr("SYNVIDEO_MEDIA_STORAGE_SECRET_ACCESS_KEY", "SYNVIDEO_S3_SECRET_ACCESS_KEY", "synvideo_dev_password"),
		UsePathStyle:    true,
		Timeout:         30 * time.Second,
	}
	if raw := os.Getenv("SYNVIDEO_MEDIA_STORAGE_PATH_STYLE"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			t.Fatalf("parse storage path style: %v", err)
		}
		cfg.UsePathStyle = value
	}
	storage, err := s3storage.New(cfg)
	if err != nil {
		t.Fatalf("create integration storage: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := storage.EnsureBucket(ctx); err != nil {
		t.Fatalf("ensure integration bucket: %v", err)
	}
	return storage
}

func integrationEnv(primary, legacy string) string {
	if value := os.Getenv(primary); value != "" {
		return value
	}
	return os.Getenv(legacy)
}

func integrationEnvOr(primary, legacy, fallback string) string {
	if value := integrationEnv(primary, legacy); value != "" {
		return value
	}
	return fallback
}
