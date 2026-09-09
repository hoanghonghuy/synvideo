package renderexport

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
)

func TestRenderHandlerCompensatesStoredOutputWhenCancelFencesFinalization(t *testing.T) {
	snapshot, visual, visualBytes := handlerFixture(t)
	assets := &handlerAssets{visual: visual, visualBytes: visualBytes}
	artifacts := &handlerArtifactRepo{cancelFence: true}
	handler := NewHandler(handlerSnapshotStore{snapshot: snapshot}, assets, artifacts, testLocalProfile())
	handler.render = func(_ context.Context, _ RenderProcessRunner, profile FFmpegProfile, input PreparedLocalRenderInput) (RenderMetadata, error) {
		body := []byte("playable-mp4-fixture")
		if err := os.WriteFile(input.OutputPath, body, 0o600); err != nil {
			return RenderMetadata{}, err
		}
		return RenderMetadata{ByteSize: int64(len(body)), DurationMS: input.DurationMS, Width: input.Width, Height: input.Height, ToolchainVersion: profile.VersionLine}, nil
	}
	job := handlerJob(t, snapshot)

	_, err := handler.Handle(context.Background(), job)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Handle() error = %v, want context.Canceled", err)
	}
	if artifacts.creates != 1 {
		t.Fatalf("expected one finalization attempt, got %d", artifacts.creates)
	}
	if assets.stores != 1 || assets.deletes != 1 {
		t.Fatalf("expected stored output compensation stores=%d deletes=%d", assets.stores, assets.deletes)
	}
	if artifacts.artifact != nil {
		t.Fatalf("cancel-fenced handler retained artifact %+v", artifacts.artifact)
	}
	if assets.final != nil {
		t.Fatalf("cancel-fenced handler retained final asset %+v", assets.final)
	}
}

func TestRenderHandlerCancelFenceDeleteFailureIsRetryableAndPreservesOrphanAsset(t *testing.T) {
	snapshot, visual, visualBytes := handlerFixture(t)
	assets := &handlerAssets{
		visual:      visual,
		visualBytes: visualBytes,
		deleteErr:   errors.New("compensation delete failed"),
	}
	artifacts := &handlerArtifactRepo{cancelFence: true}
	handler := NewHandler(handlerSnapshotStore{snapshot: snapshot}, assets, artifacts, testLocalProfile())
	handler.render = func(_ context.Context, _ RenderProcessRunner, profile FFmpegProfile, input PreparedLocalRenderInput) (RenderMetadata, error) {
		body := []byte("playable-mp4-fixture")
		if err := os.WriteFile(input.OutputPath, body, 0o600); err != nil {
			return RenderMetadata{}, err
		}
		return RenderMetadata{ByteSize: int64(len(body)), DurationMS: input.DurationMS, Width: input.Width, Height: input.Height, ToolchainVersion: profile.VersionLine}, nil
	}
	job := handlerJob(t, snapshot)

	_, err := handler.Handle(context.Background(), job)
	var retryErr *jobs.RetryableJobError
	if !errors.As(err, &retryErr) || retryErr.Code != ErrorStorageFailed {
		t.Fatalf("Handle() error = %v, want retryable %s", err, ErrorStorageFailed)
	}
	if errors.Is(err, context.Canceled) {
		t.Fatal("delete failure must not report clean cancellation")
	}
	if assets.deletes != 1 || assets.final == nil {
		t.Fatalf("expected orphan asset after failed compensation delete, deletes=%d final=%+v", assets.deletes, assets.final)
	}
	if artifacts.artifact != nil {
		t.Fatalf("cancel-fenced handler published artifact %+v", artifacts.artifact)
	}
}

func TestRenderHandlerCancelFenceDeleteFailureRetriesCompensationWithoutRerender(t *testing.T) {
	snapshot, visual, visualBytes := handlerFixture(t)
	assets := &handlerAssets{
		visual:      visual,
		visualBytes: visualBytes,
		deleteErr:   errors.New("compensation delete failed"),
	}
	artifacts := &handlerArtifactRepo{cancelFence: true}
	handler := NewHandler(handlerSnapshotStore{snapshot: snapshot}, assets, artifacts, testLocalProfile())
	renderCalls := 0
	handler.render = func(_ context.Context, _ RenderProcessRunner, profile FFmpegProfile, input PreparedLocalRenderInput) (RenderMetadata, error) {
		renderCalls++
		body := []byte("playable-mp4-fixture")
		if err := os.WriteFile(input.OutputPath, body, 0o600); err != nil {
			return RenderMetadata{}, err
		}
		return RenderMetadata{ByteSize: int64(len(body)), DurationMS: input.DurationMS, Width: input.Width, Height: input.Height, ToolchainVersion: profile.VersionLine}, nil
	}
	job := handlerJob(t, snapshot)

	_, err := handler.Handle(context.Background(), job)
	var retryErr *jobs.RetryableJobError
	if !errors.As(err, &retryErr) || retryErr.Code != ErrorStorageFailed {
		t.Fatalf("first Handle() error = %v, want retryable %s", err, ErrorStorageFailed)
	}
	if renderCalls != 1 || assets.stores != 1 {
		t.Fatalf("first attempt render=%d stores=%d", renderCalls, assets.stores)
	}

	assets.deleteErr = nil
	_, err = handler.Handle(context.Background(), job)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("second Handle() error = %v, want context.Canceled after compensation", err)
	}
	if renderCalls != 1 || assets.stores != 1 || assets.deletes != 2 {
		t.Fatalf("compensation retry rerendered or stored again: render=%d stores=%d deletes=%d", renderCalls, assets.stores, assets.deletes)
	}
	if assets.final != nil {
		t.Fatalf("compensated asset should be deleted, still have %+v", assets.final)
	}
}
