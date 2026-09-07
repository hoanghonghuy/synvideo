package renderexport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

type handlerSnapshotStore struct{ snapshot sceneeditor.Snapshot }

func (s handlerSnapshotStore) GetSnapshot(_ context.Context, ownerID, projectID uuid.UUID, digest string) (sceneeditor.Snapshot, error) {
	if projectID != s.snapshot.ProjectID || digest != s.snapshot.Digest || ownerID == uuid.Nil {
		return sceneeditor.Snapshot{}, sceneeditor.ErrNotFound
	}
	return s.snapshot, nil
}

type handlerAssets struct {
	visual      mediaasset.MediaAsset
	visualBytes []byte
	final       *mediaasset.MediaAsset
	stores      int
	deletes     int
}

func (s *handlerAssets) Get(_ context.Context, _ project.Principal, projectID, assetID uuid.UUID) (mediaasset.MediaAsset, error) {
	if s.visual.ProjectID != projectID || s.visual.ID != assetID {
		return mediaasset.MediaAsset{}, mediaasset.ErrNotFound
	}
	return s.visual, nil
}

func (s *handlerAssets) Open(_ context.Context, _ project.Principal, projectID, assetID uuid.UUID) (io.ReadCloser, error) {
	if s.visual.ProjectID != projectID || s.visual.ID != assetID {
		return nil, mediaasset.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(s.visualBytes)), nil
}

func (s *handlerAssets) FindFinalByJob(_ context.Context, _ project.Principal, projectID, jobID uuid.UUID) (mediaasset.MediaAsset, error) {
	if s.final == nil || s.final.ProjectID != projectID {
		return mediaasset.MediaAsset{}, mediaasset.ErrNotFound
	}
	var metadata renderOutputMetadata
	if json.Unmarshal(s.final.Metadata, &metadata) != nil || metadata.RenderJobID != jobID.String() {
		return mediaasset.MediaAsset{}, mediaasset.ErrNotFound
	}
	return *s.final, nil
}

func (s *handlerAssets) Store(_ context.Context, principal project.Principal, projectID uuid.UUID, input mediaasset.CreateInput) (mediaasset.MediaAsset, error) {
	s.stores++
	body, err := io.ReadAll(input.Reader)
	if err != nil {
		return mediaasset.MediaAsset{}, err
	}
	digest := sha256.Sum256(body)
	now := time.Now().UTC()
	asset := mediaasset.MediaAsset{
		ID:               uuid.New(),
		OwnerID:          principal.OwnerID,
		ProjectID:        projectID,
		Kind:             input.Kind,
		Origin:           input.Origin,
		ObjectKey:        "projects/" + projectID.String() + "/assets/" + uuid.NewString(),
		MimeType:         input.MimeType,
		ByteSize:         int64(len(body)),
		SHA256:           hex.EncodeToString(digest[:]),
		OriginalFilename: input.OriginalFilename,
		Metadata:         append([]byte(nil), input.Metadata...),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.final = &asset
	return asset, nil
}

func (s *handlerAssets) Delete(_ context.Context, _ project.Principal, _ uuid.UUID, assetID uuid.UUID) error {
	s.deletes++
	if s.final != nil && s.final.ID == assetID {
		s.final = nil
	}
	return nil
}

type handlerArtifactRepo struct {
	artifact  *RenderArtifact
	createErr error
	creates   int
}

func (r *handlerArtifactRepo) CreateForLease(_ context.Context, _ uuid.UUID, artifact RenderArtifact) (RenderArtifact, error) {
	r.creates++
	if r.createErr != nil {
		err := r.createErr
		r.createErr = nil
		return RenderArtifact{}, err
	}
	copy := artifact
	r.artifact = &copy
	return copy, nil
}

func (r *handlerArtifactRepo) GetByJob(_ context.Context, ownerID, projectID, jobID uuid.UUID) (RenderArtifact, error) {
	if r.artifact == nil || r.artifact.OwnerID != ownerID || r.artifact.ProjectID != projectID || r.artifact.JobID != jobID {
		return RenderArtifact{}, ErrArtifactNotFound
	}
	return *r.artifact, nil
}

func TestRenderHandlerFinalizesDurableArtifactAndRetryDoesNotRerender(t *testing.T) {
	snapshot, visual, visualBytes := handlerFixture(t)
	assets := &handlerAssets{visual: visual, visualBytes: visualBytes}
	artifacts := &handlerArtifactRepo{}
	handler := NewHandler(handlerSnapshotStore{snapshot: snapshot}, assets, artifacts, testLocalProfile())
	renderCalls := 0
	handler.render = func(_ context.Context, _ RenderProcessRunner, profile FFmpegProfile, input PreparedLocalRenderInput) (RenderMetadata, error) {
		renderCalls++
		if profile.ID != LocalProfileID || input.DurationMS != snapshot.Scenes[0].DurationMS || input.Width != LocalProfileWidth || input.Height != LocalProfileHeight {
			t.Fatalf("unexpected render request: profile=%+v input=%+v", profile, input)
		}
		body := []byte("playable-mp4-fixture")
		if err := os.WriteFile(input.OutputPath, body, 0o600); err != nil {
			return RenderMetadata{}, err
		}
		return RenderMetadata{ByteSize: int64(len(body)), DurationMS: input.DurationMS, Width: input.Width, Height: input.Height, ToolchainVersion: profile.VersionLine}, nil
	}
	job := handlerJob(t, snapshot)

	result, err := handler.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	var decoded HandlerResult
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ArtifactID == uuid.Nil || decoded.MediaAssetID == uuid.Nil || decoded.SnapshotDigest != snapshot.Digest || decoded.ProfileID != LocalProfileID {
		t.Fatalf("unexpected result: %+v", decoded)
	}
	if renderCalls != 1 || assets.stores != 1 || artifacts.creates != 1 {
		t.Fatalf("first execution counts render=%d stores=%d creates=%d", renderCalls, assets.stores, artifacts.creates)
	}

	if _, err := handler.Handle(context.Background(), job); err != nil {
		t.Fatalf("retry Handle() error = %v", err)
	}
	if renderCalls != 1 || assets.stores != 1 || artifacts.creates != 1 {
		t.Fatalf("retry duplicated work render=%d stores=%d creates=%d", renderCalls, assets.stores, artifacts.creates)
	}
}

func TestRenderHandlerRecoversStoredOutputAfterLeaseLossWithoutRerender(t *testing.T) {
	snapshot, visual, visualBytes := handlerFixture(t)
	assets := &handlerAssets{visual: visual, visualBytes: visualBytes}
	artifacts := &handlerArtifactRepo{createErr: ErrStaleRenderLease}
	handler := NewHandler(handlerSnapshotStore{snapshot: snapshot}, assets, artifacts, testLocalProfile())
	renderCalls := 0
	handler.render = func(_ context.Context, _ RenderProcessRunner, profile FFmpegProfile, input PreparedLocalRenderInput) (RenderMetadata, error) {
		renderCalls++
		body := []byte("render-output")
		if err := os.WriteFile(input.OutputPath, body, 0o600); err != nil {
			return RenderMetadata{}, err
		}
		return RenderMetadata{ByteSize: int64(len(body)), DurationMS: input.DurationMS, Width: input.Width, Height: input.Height, ToolchainVersion: profile.VersionLine}, nil
	}
	job := handlerJob(t, snapshot)

	_, err := handler.Handle(context.Background(), job)
	var retryErr *jobs.RetryableJobError
	if !errors.As(err, &retryErr) || retryErr.Code != ErrorStaleLease {
		t.Fatalf("first Handle() error = %v, want stale-lease retry", err)
	}
	if assets.final == nil || renderCalls != 1 || assets.stores != 1 {
		t.Fatalf("expected recoverable stored output, final=%+v render=%d stores=%d", assets.final, renderCalls, assets.stores)
	}

	newLease := uuid.New()
	job.LeaseToken = &newLease
	if _, err := handler.Handle(context.Background(), job); err != nil {
		t.Fatalf("reclaimed Handle() error = %v", err)
	}
	if renderCalls != 1 || assets.stores != 1 || artifacts.artifact == nil {
		t.Fatalf("reclaimed execution repeated expensive work render=%d stores=%d artifact=%+v", renderCalls, assets.stores, artifacts.artifact)
	}
}

func TestRenderHandlerFailsClosedOnUnsupportedSnapshotSemantics(t *testing.T) {
	snapshot, visual, visualBytes := handlerFixture(t)
	doc := sceneeditor.Document{
		ID:               snapshot.CompositionID,
		OwnerID:          visual.OwnerID,
		ProjectID:        snapshot.ProjectID,
		Revision:         snapshot.Revision,
		ScenePlanVersion: snapshot.ScenePlanVersion,
		Scenes:           append([]sceneeditor.Scene(nil), snapshot.Scenes...),
		AudioMix:         snapshot.AudioMix,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	doc.Scenes[0].Narration = &sceneeditor.NarrationRef{AssetID: uuid.New(), BindingID: uuid.New(), LineageID: uuid.New(), DurationMS: 500}
	var err error
	snapshot, err = sceneeditor.NewSnapshot(doc, sceneeditor.StateCurrent)
	if err != nil {
		t.Fatalf("NewSnapshot() with unsupported-but-valid narration error = %v", err)
	}
	assets := &handlerAssets{visual: visual, visualBytes: visualBytes}
	handler := NewHandler(handlerSnapshotStore{snapshot: snapshot}, assets, &handlerArtifactRepo{}, testLocalProfile())
	rendered := false
	handler.render = func(context.Context, RenderProcessRunner, FFmpegProfile, PreparedLocalRenderInput) (RenderMetadata, error) {
		rendered = true
		return RenderMetadata{}, nil
	}

	_, err = handler.Handle(context.Background(), handlerJob(t, snapshot))
	var terminal *jobs.TerminalJobError
	if !errors.As(err, &terminal) || terminal.Code != ErrorUnsupportedSnapshot {
		t.Fatalf("Handle() error = %v, want unsupported terminal error", err)
	}
	if rendered || assets.stores != 0 {
		t.Fatalf("unsupported snapshot performed render/store")
	}
}

func TestRenderHandlerRejectsCorruptVisualBytesBeforeFFmpeg(t *testing.T) {
	snapshot, visual, _ := handlerFixture(t)
	assets := &handlerAssets{visual: visual, visualBytes: []byte("corrupt")}
	handler := NewHandler(handlerSnapshotStore{snapshot: snapshot}, assets, &handlerArtifactRepo{}, testLocalProfile())
	rendered := false
	handler.render = func(context.Context, RenderProcessRunner, FFmpegProfile, PreparedLocalRenderInput) (RenderMetadata, error) {
		rendered = true
		return RenderMetadata{}, nil
	}

	_, err := handler.Handle(context.Background(), handlerJob(t, snapshot))
	var terminal *jobs.TerminalJobError
	if !errors.As(err, &terminal) || terminal.Code != ErrorInputUnavailable {
		t.Fatalf("Handle() error = %v, want input-integrity terminal error", err)
	}
	if rendered {
		t.Fatal("corrupt input reached FFmpeg")
	}
}

func TestRenderHandlerRejectsSnapshotWhoseCanonicalDigestNoLongerMatches(t *testing.T) {
	snapshot, visual, visualBytes := handlerFixture(t)
	acceptedDigest := snapshot.Digest
	snapshot.Scenes[0].DurationMS++
	snapshot.Digest = acceptedDigest
	assets := &handlerAssets{visual: visual, visualBytes: visualBytes}
	handler := NewHandler(handlerSnapshotStore{snapshot: snapshot}, assets, &handlerArtifactRepo{}, testLocalProfile())
	rendered := false
	handler.render = func(context.Context, RenderProcessRunner, FFmpegProfile, PreparedLocalRenderInput) (RenderMetadata, error) {
		rendered = true
		return RenderMetadata{}, nil
	}

	_, err := handler.Handle(context.Background(), handlerJob(t, snapshot))
	var terminal *jobs.TerminalJobError
	if !errors.As(err, &terminal) || terminal.Code != ErrorSnapshotInvalid {
		t.Fatalf("Handle() error = %v, want snapshot-integrity terminal error", err)
	}
	if rendered {
		t.Fatal("tampered snapshot reached FFmpeg")
	}
}

func handlerFixture(t *testing.T) (sceneeditor.Snapshot, mediaasset.MediaAsset, []byte) {
	t.Helper()
	projectID := uuid.New()
	assetID := uuid.New()
	visualBytes := []byte("valid-image-fixture")
	sum := sha256.Sum256(visualBytes)
	now := time.Now().UTC()
	doc := sceneeditor.Document{
		ID:               uuid.New(),
		OwnerID:          uuid.New(),
		ProjectID:        projectID,
		Revision:         1,
		ScenePlanVersion: 1,
		Scenes: []sceneeditor.Scene{{
			ID:         uuid.New(),
			SceneKey:   "scene-1",
			Visual:     &sceneeditor.VisualRef{AssetID: assetID, BindingID: uuid.New()},
			DurationMS: 1000,
			VisualTreatment: sceneeditor.VisualTreatment{
				Fit:   sceneeditor.FitContain,
				Scale: 1,
			},
			TransitionOut: sceneeditor.Transition{Kind: sceneeditor.TransitionCut},
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
	snapshot, err := sceneeditor.NewSnapshot(doc, sceneeditor.StateCurrent)
	if err != nil {
		t.Fatalf("NewSnapshot() error = %v", err)
	}
	visual := mediaasset.MediaAsset{
		ID:        assetID,
		OwnerID:   doc.OwnerID,
		ProjectID: projectID,
		Kind:      mediaasset.KindImage,
		Origin:    mediaasset.OriginUpload,
		MimeType:  "image/png",
		ByteSize:  int64(len(visualBytes)),
		SHA256:    hex.EncodeToString(sum[:]),
	}
	return snapshot, visual, visualBytes
}

func handlerJob(t *testing.T, snapshot sceneeditor.Snapshot) jobs.Job {
	t.Helper()
	projectID := snapshot.ProjectID
	lease := uuid.New()
	payload, err := json.Marshal(RenderPayload{SnapshotDigest: snapshot.Digest, SnapshotSchema: snapshot.SchemaVersion, ProfileID: LocalProfileID})
	if err != nil {
		t.Fatal(err)
	}
	return jobs.Job{
		ID:          uuid.New(),
		OwnerID:     uuid.New(),
		ProjectID:   &projectID,
		Kind:        JobKind,
		State:       jobs.StateRunning,
		Attempt:     1,
		MaxAttempts: 2,
		LeaseToken:  &lease,
		Payload:     payload,
	}
}

func TestRenderHandlerRealFFmpegIntegrationProducesPlayableMP4(t *testing.T) {
	if os.Getenv("SYNVIDEO_TEST_FFMPEG") != "1" {
		t.Skip("set SYNVIDEO_TEST_FFMPEG=1 to run real FFmpeg render worker integration")
	}
	profile, err := ProbeLocalFFmpegProfile(context.Background(), nil)
	if err != nil {
		t.Fatalf("ProbeLocalFFmpegProfile() error = %v", err)
	}
	snapshot, visual, _ := handlerFixture(t)
	ppm := []byte("P6\n2 2\n255\n" + "\xff\x00\x00\x00\xff\x00\x00\x00\xff\xff\xff\xff")
	sum := sha256.Sum256(ppm)
	visual.ByteSize = int64(len(ppm))
	visual.SHA256 = hex.EncodeToString(sum[:])
	visual.MimeType = "image/x-portable-pixmap"
	assets := &handlerAssets{visual: visual, visualBytes: ppm}
	artifacts := &handlerArtifactRepo{}
	handler := NewHandler(handlerSnapshotStore{snapshot: snapshot}, assets, artifacts, profile)
	job := handlerJob(t, snapshot)

	result, err := handler.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle(real FFmpeg) error = %v", err)
	}
	var decoded HandlerResult
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ArtifactID == uuid.Nil || decoded.MediaAssetID == uuid.Nil || assets.final == nil || assets.final.ByteSize <= 0 {
		t.Fatalf("real FFmpeg render did not finalize durable output: result=%+v final=%+v", decoded, assets.final)
	}
	if artifacts.artifact == nil || artifacts.artifact.DurationMS <= 0 || artifacts.artifact.Width != LocalProfileWidth || artifacts.artifact.Height != LocalProfileHeight {
		t.Fatalf("real FFmpeg artifact metadata invalid: %+v", artifacts.artifact)
	}
}
