package s3storage_test

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset/s3storage"
)

func validConfig() s3storage.Config {
	return s3storage.Config{
		Endpoint:        "http://localhost:8333",
		Region:          "local",
		Bucket:          "synvideo-local",
		AccessKeyID:     "synvideo",
		SecretAccessKey: "synvideo_dev_password",
		UsePathStyle:    true,
		Timeout:         5 * time.Second,
	}
}

func TestConfigValidationDoesNotEchoSecretsOrEndpointInternals(t *testing.T) {
	cases := []s3storage.Config{
		{Endpoint: "http://user:secret@example.test", Bucket: "bucket", AccessKeyID: "key", SecretAccessKey: "secret"},
		{Endpoint: "http://example.test/path", Bucket: "bucket", AccessKeyID: "key", SecretAccessKey: "secret"},
		{Endpoint: "http://example.test", Bucket: "", AccessKeyID: "key", SecretAccessKey: "secret"},
		{Endpoint: "http://example.test", Bucket: "bucket", AccessKeyID: "", SecretAccessKey: "secret"},
	}
	for _, cfg := range cases {
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected invalid config")
		}
		if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "example.test") {
			t.Fatalf("config details leaked: %v", err)
		}
	}
}

func TestStorageRejectsUntrustedObjectKeys(t *testing.T) {
	storage, err := s3storage.New(validConfig())
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	_, err = storage.Stat(context.Background(), "../secret")
	if !errors.Is(err, mediaasset.ErrStorageFailed) {
		t.Fatalf("expected safe storage error, got %v", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("key leaked in error: %v", err)
	}
}

func TestStoragePropagatesContextCancellation(t *testing.T) {
	storage, err := s3storage.New(validConfig())
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	key := "projects/22222222-2222-4222-8222-222222222222/assets/33333333-3333-4333-8333-333333333333"
	_, err = storage.Stat(ctx, key)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestStorageLocalS3CompatibleRoundTrip(t *testing.T) {
	endpoint := storageEnv("SYNVIDEO_MEDIA_STORAGE_ENDPOINT", "SYNVIDEO_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("SYNVIDEO_MEDIA_STORAGE_ENDPOINT is not set; start local SeaweedFS/MinIO for this integration test")
	}
	cfg := validConfig()
	cfg.Endpoint = endpoint
	if value := storageEnv("SYNVIDEO_MEDIA_STORAGE_REGION", "SYNVIDEO_S3_REGION"); value != "" {
		cfg.Region = value
	}
	if value := storageEnv("SYNVIDEO_MEDIA_STORAGE_BUCKET", "SYNVIDEO_S3_BUCKET"); value != "" {
		cfg.Bucket = value
	}
	if value := storageEnv("SYNVIDEO_MEDIA_STORAGE_ACCESS_KEY_ID", "SYNVIDEO_S3_ACCESS_KEY_ID"); value != "" {
		cfg.AccessKeyID = value
	}
	if value := storageEnv("SYNVIDEO_MEDIA_STORAGE_SECRET_ACCESS_KEY", "SYNVIDEO_S3_SECRET_ACCESS_KEY"); value != "" {
		cfg.SecretAccessKey = value
	}
	storage, err := s3storage.New(cfg)
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := storage.EnsureBucket(ctx); err != nil {
		t.Fatalf("ensure bucket: %v", err)
	}
	key := "projects/22222222-2222-4222-8222-222222222222/assets/" + uuid.NewString()
	want := "local deterministic media object"
	if _, err := storage.Put(ctx, mediaasset.PutObjectInput{Key: key, Body: strings.NewReader(want), ContentType: "text/plain"}); err != nil {
		t.Fatalf("put: %v", err)
	}
	t.Cleanup(func() { _ = storage.Delete(context.Background(), key) })
	info, err := storage.Stat(ctx, key)
	if err != nil || info.Size != int64(len(want)) {
		t.Fatalf("stat: info=%+v err=%v", info, err)
	}
	reader, err := storage.Open(ctx, key)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	got, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil || string(got) != want {
		t.Fatalf("read: bytes=%q read_err=%v close_err=%v", got, readErr, closeErr)
	}
	rangeReader, err := storage.OpenRange(ctx, key, 6, 13)
	if err != nil {
		t.Fatalf("open range: %v", err)
	}
	rangeBytes, rangeReadErr := io.ReadAll(rangeReader)
	rangeCloseErr := rangeReader.Close()
	if rangeReadErr != nil || rangeCloseErr != nil || string(rangeBytes) != "deterministic" {
		t.Fatalf("range read: bytes=%q read_err=%v close_err=%v", rangeBytes, rangeReadErr, rangeCloseErr)
	}
	if err := storage.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := storage.Stat(ctx, key); !errors.Is(err, mediaasset.ErrObjectNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

func storageEnv(primary, legacy string) string {
	if value := os.Getenv(primary); value != "" {
		return value
	}
	return os.Getenv(legacy)
}
