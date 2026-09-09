package renderexport

import (
	"context"
	"errors"
	"os"
	"testing"
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
