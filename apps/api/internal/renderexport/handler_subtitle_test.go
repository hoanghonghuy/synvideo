package renderexport

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/captions"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

func TestRenderHandlerWebVTTFinalizesPinnedSidecarAndRetryIsIdempotent(t *testing.T) {
	snapshot, visual, visualBytes, captionDoc := handlerCaptionFixture(t)
	assets := &handlerAssets{visual: visual, visualBytes: visualBytes}
	artifacts := &handlerArtifactRepo{}
	reader := &snapshotCaptionReaderStub{doc: captionDoc}
	handler := NewHandler(handlerSnapshotStore{snapshot: snapshot}, assets, artifacts, testLocalProfile(), reader)
	handler.render = subtitleTestRenderer
	job := handlerJobWithSubtitleMode(t, snapshot, captionDoc.OwnerID, SubtitleModeWebVTT)

	result, err := handler.Handle(context.Background(), job)
	if err != nil {
		t.Fatal(err)
	}
	var decoded HandlerResult
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SubtitleMediaAssetID == nil || assets.subtitle == nil || artifacts.artifact == nil {
		t.Fatalf("sidecar was not durably finalized: result=%+v subtitle=%+v artifact=%+v", decoded, assets.subtitle, artifacts.artifact)
	}
	if !strings.Contains(string(assets.subtitleBytes), "Pinned &lt;caption&gt;") || len(reader.reads) != 1 {
		t.Fatalf("unexpected pinned WebVTT payload=%q reads=%d", assets.subtitleBytes, len(reader.reads))
	}
	stores := assets.stores
	if _, err := handler.Handle(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if assets.stores != stores || artifacts.creates != 1 {
		t.Fatalf("retry duplicated finalized outputs: stores=%d want=%d artifactCreates=%d", assets.stores, stores, artifacts.creates)
	}
}

func TestRenderHandlerWebVTTWithoutPinnedCaptionsCreatesNoSidecar(t *testing.T) {
	snapshot, visual, visualBytes := handlerFixture(t)
	assets := &handlerAssets{visual: visual, visualBytes: visualBytes}
	artifacts := &handlerArtifactRepo{}
	handler := NewHandler(handlerSnapshotStore{snapshot: snapshot}, assets, artifacts, testLocalProfile(), &snapshotCaptionReaderStub{})
	handler.render = subtitleTestRenderer
	job := handlerJobWithSubtitleMode(t, snapshot, uuid.New(), SubtitleModeWebVTT)

	result, err := handler.Handle(context.Background(), job)
	if err != nil {
		t.Fatal(err)
	}
	var decoded HandlerResult
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SubtitleMediaAssetID != nil || assets.subtitle != nil || assets.stores != 1 {
		t.Fatalf("no-caption render created sidecar: result=%+v subtitle=%+v stores=%d", decoded, assets.subtitle, assets.stores)
	}
}

func TestRenderHandlerSubtitleStorageFailureCannotFinalizeSuccess(t *testing.T) {
	snapshot, visual, visualBytes, captionDoc := handlerCaptionFixture(t)
	assets := &handlerAssets{visual: visual, visualBytes: visualBytes, subtitleStoreErr: errors.New("subtitle storage unavailable")}
	artifacts := &handlerArtifactRepo{}
	handler := NewHandler(handlerSnapshotStore{snapshot: snapshot}, assets, artifacts, testLocalProfile(), &snapshotCaptionReaderStub{doc: captionDoc})
	handler.render = subtitleTestRenderer
	job := handlerJobWithSubtitleMode(t, snapshot, captionDoc.OwnerID, SubtitleModeWebVTT)

	_, err := handler.Handle(context.Background(), job)
	var retryable *jobs.RetryableJobError
	if !errors.As(err, &retryable) || retryable.Code != ErrorStorageFailed {
		t.Fatalf("Handle() error=%v, want retryable subtitle storage failure", err)
	}
	if artifacts.creates != 0 || artifacts.artifact != nil {
		t.Fatalf("partial subtitle failure falsely finalized artifact: creates=%d artifact=%+v", artifacts.creates, artifacts.artifact)
	}
}

func handlerCaptionFixture(t *testing.T) (sceneeditor.Snapshot, mediaasset.MediaAsset, []byte, captions.Document) {
	t.Helper()
	base, visual, visualBytes := handlerFixture(t)
	ownerID := uuid.New()
	documentID := uuid.New()
	scenes := append([]sceneeditor.Scene(nil), base.Scenes...)
	scenes[0].Caption = &sceneeditor.CaptionRef{DocumentID: documentID, Revision: 2, LineageID: uuid.New(), LastEndMS: 800}
	now := time.Now().UTC()
	doc := sceneeditor.Document{ID: base.CompositionID, OwnerID: ownerID, ProjectID: base.ProjectID, Revision: base.Revision, ScenePlanVersion: base.ScenePlanVersion, Scenes: scenes, AudioMix: base.AudioMix, CreatedAt: now, UpdatedAt: now}
	snapshot, err := sceneeditor.NewSnapshot(doc, sceneeditor.StateCurrent)
	if err != nil {
		t.Fatal(err)
	}
	captionDoc := captions.Document{ID: documentID, OwnerID: ownerID, ProjectID: base.ProjectID, ScenePlanVersion: base.ScenePlanVersion, SceneKey: scenes[0].SceneKey, Revision: 2, Segments: []captions.Segment{{ID: uuid.New(), StartMS: 100, EndMS: 800, Text: "Pinned <caption>"}}}
	return snapshot, visual, visualBytes, captionDoc
}

func handlerJobWithSubtitleMode(t *testing.T, snapshot sceneeditor.Snapshot, ownerID uuid.UUID, mode SubtitleMode) jobs.Job {
	t.Helper()
	job := handlerJob(t, snapshot)
	job.OwnerID = ownerID
	payload := RenderPayload{SnapshotDigest: snapshot.Digest, SnapshotSchema: snapshot.SchemaVersion, ProfileID: LocalProfileID, SubtitleMode: mode}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	job.Payload = encoded
	return job
}

func subtitleTestRenderer(_ context.Context, _ RenderProcessRunner, profile FFmpegProfile, input PreparedLocalRenderInput) (RenderMetadata, error) {
	body := []byte("render-output")
	if err := os.WriteFile(input.OutputPath, body, 0o600); err != nil {
		return RenderMetadata{}, err
	}
	return RenderMetadata{ByteSize: int64(len(body)), DurationMS: input.DurationMS, Width: input.Width, Height: input.Height, ToolchainVersion: profile.VersionLine}, nil
}

func TestRenderHandlerOffModeBurnsPinnedCaptionsWithoutCreatingSidecar(t *testing.T) {
	snapshot, visual, visualBytes, captionDoc := handlerCaptionFixture(t)
	assets := &handlerAssets{visual: visual, visualBytes: visualBytes}
	artifacts := &handlerArtifactRepo{}
	reader := &snapshotCaptionReaderStub{doc: captionDoc}
	handler := NewHandler(handlerSnapshotStore{snapshot: snapshot}, assets, artifacts, testLocalProfile(), reader)
	var renderedCaption []byte
	var renderedProfile string
	handler.render = func(ctx context.Context, runner RenderProcessRunner, profile FFmpegProfile, input PreparedLocalRenderInput) (RenderMetadata, error) {
		if input.CaptionVTTPath == "" {
			t.Fatal("captioned snapshot reached renderer without burned-caption VTT")
		}
		payload, err := os.ReadFile(input.CaptionVTTPath)
		if err != nil {
			t.Fatal(err)
		}
		renderedCaption = payload
		renderedProfile = input.CaptionProfileID
		return subtitleTestRenderer(ctx, runner, profile, input)
	}
	job := handlerJobWithSubtitleMode(t, snapshot, captionDoc.OwnerID, SubtitleModeOff)

	result, err := handler.Handle(context.Background(), job)
	if err != nil {
		t.Fatal(err)
	}
	var decoded HandlerResult
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SubtitleMediaAssetID != nil || assets.subtitle != nil || artifacts.artifact == nil || artifacts.artifact.SubtitleMediaAssetID != nil {
		t.Fatalf("off mode created sidecar work: result=%+v subtitle=%+v artifact=%+v", decoded, assets.subtitle, artifacts.artifact)
	}
	if assets.stores != 1 {
		t.Fatalf("off mode stores=%d, want only MP4", assets.stores)
	}
	if renderedProfile != BurnedCaptionProfileV1 || !strings.Contains(string(renderedCaption), "Pinned &lt;caption&gt;") || len(reader.reads) != 1 {
		t.Fatalf("burned-caption input profile=%q payload=%q reads=%d", renderedProfile, renderedCaption, len(reader.reads))
	}
}

func TestRenderHandlerSubtitleStoreRetryReusesDurableMP4(t *testing.T) {
	snapshot, visual, visualBytes, captionDoc := handlerCaptionFixture(t)
	assets := &handlerAssets{visual: visual, visualBytes: visualBytes, subtitleStoreErr: errors.New("temporary subtitle storage failure")}
	artifacts := &handlerArtifactRepo{}
	handler := NewHandler(handlerSnapshotStore{snapshot: snapshot}, assets, artifacts, testLocalProfile(), &snapshotCaptionReaderStub{doc: captionDoc})
	renderCalls := 0
	handler.render = func(ctx context.Context, runner RenderProcessRunner, profile FFmpegProfile, input PreparedLocalRenderInput) (RenderMetadata, error) {
		renderCalls++
		return subtitleTestRenderer(ctx, runner, profile, input)
	}
	job := handlerJobWithSubtitleMode(t, snapshot, captionDoc.OwnerID, SubtitleModeWebVTT)

	_, err := handler.Handle(context.Background(), job)
	var retryable *jobs.RetryableJobError
	if !errors.As(err, &retryable) || retryable.Code != ErrorStorageFailed {
		t.Fatalf("first Handle() error=%v, want retryable subtitle storage failure", err)
	}
	if assets.final == nil || assets.subtitle != nil || artifacts.artifact != nil || renderCalls != 1 {
		t.Fatalf("first attempt state final=%+v subtitle=%+v artifact=%+v renders=%d", assets.final, assets.subtitle, artifacts.artifact, renderCalls)
	}

	result, err := handler.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("retry Handle() error=%v", err)
	}
	var decoded HandlerResult
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatal(err)
	}
	if renderCalls != 1 || decoded.SubtitleMediaAssetID == nil || assets.subtitle == nil || artifacts.artifact == nil {
		t.Fatalf("retry failed to reuse MP4/finalize sidecar renders=%d result=%+v subtitle=%+v artifact=%+v", renderCalls, decoded, assets.subtitle, artifacts.artifact)
	}
}
