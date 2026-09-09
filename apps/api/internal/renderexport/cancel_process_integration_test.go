package renderexport

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

func TestRenderHandlerRealFFmpegCancellationTerminatesProcess(t *testing.T) {
	if os.Getenv("SYNVIDEO_TEST_FFMPEG") != "1" {
		t.Skip("set SYNVIDEO_TEST_FFMPEG=1 to run real FFmpeg cancellation integration")
	}
	profile, err := ProbeLocalFFmpegProfile(context.Background(), nil)
	if err != nil {
		t.Fatalf("ProbeLocalFFmpegProfile() error = %v", err)
	}
	snapshot, visual, visualBytes := handlerFixture(t)
	now := time.Now().UTC()
	doc := sceneeditor.Document{
		ID:               snapshot.CompositionID,
		OwnerID:          visual.OwnerID,
		ProjectID:        snapshot.ProjectID,
		Revision:         snapshot.Revision,
		ScenePlanVersion: snapshot.ScenePlanVersion,
		Scenes:           append([]sceneeditor.Scene(nil), snapshot.Scenes...),
		AudioMix:         snapshot.AudioMix,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	doc.Scenes[0].DurationMS = 30_000
	snapshot, err = sceneeditor.NewSnapshot(doc, sceneeditor.StateCurrent)
	if err != nil {
		t.Fatalf("NewSnapshot() with long duration error = %v", err)
	}
	assets := &handlerAssets{visual: visual, visualBytes: visualBytes}
	artifacts := &handlerArtifactRepo{}
	handler := NewHandler(handlerSnapshotStore{snapshot: snapshot}, assets, artifacts, profile)
	job := handlerJob(t, snapshot)

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	var renderCalls atomic.Int32
	handler.render = func(renderCtx context.Context, runner RenderProcessRunner, profile FFmpegProfile, input PreparedLocalRenderInput) (RenderMetadata, error) {
		renderCalls.Add(1)
		close(started)
		return RenderSingleVisualMP4(renderCtx, runner, profile, input)
	}

	done := make(chan error, 1)
	go func() {
		_, handleErr := handler.Handle(ctx, job)
		done <- handleErr
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("real FFmpeg render did not start")
	}
	cancel()

	select {
	case handleErr := <-done:
		if handleErr == nil {
			t.Fatal("expected cancellation error, got success")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("real FFmpeg render did not stop after cancellation")
	}
	if artifacts.artifact != nil || assets.final != nil {
		t.Fatalf("cancelled render published artifact=%+v final=%+v", artifacts.artifact, assets.final)
	}
	if renderCalls.Load() != 1 {
		t.Fatalf("expected one render attempt, got %d", renderCalls.Load())
	}
}

func TestRenderHandlerCancellationUsesTerminalErrorClassification(t *testing.T) {
	snapshot, visual, visualBytes := handlerFixture(t)
	assets := &handlerAssets{visual: visual, visualBytes: visualBytes}
	handler := NewHandler(handlerSnapshotStore{snapshot: snapshot}, assets, &handlerArtifactRepo{}, testLocalProfile())
	handler.render = func(ctx context.Context, _ RenderProcessRunner, profile FFmpegProfile, input PreparedLocalRenderInput) (RenderMetadata, error) {
		<-ctx.Done()
		return RenderMetadata{}, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := handler.Handle(ctx, handlerJob(t, snapshot))
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if _, ok := err.(*jobs.RetryableJobError); ok {
		t.Fatalf("cancellation should not be classified retryable: %v", err)
	}
}
