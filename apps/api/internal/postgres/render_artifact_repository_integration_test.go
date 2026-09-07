package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/renderexport"
)

func TestRenderArtifactRepositoryIntegrationScopesDedupesAndFencesAuthoritativeOutputs(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	ownerID := uuid.New()
	projectRepository := NewProjectRepository(pool)
	projectItem, err := projectRepository.Create(ctx, ownerID, validIntegrationCreateInput("Render artifact"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	jobRepository := NewJobRepository(pool)
	projectID := projectItem.ID
	job, err := jobRepository.Enqueue(ctx, jobs.EnqueueInput{
		ID:          uuid.New(),
		OwnerID:     ownerID,
		ProjectID:   &projectID,
		Kind:        renderexport.JobKind,
		MaxAttempts: 2,
		Payload:     canonicalRenderPayload(),
	})
	if err != nil {
		t.Fatalf("enqueue render job: %v", err)
	}
	claimed := claimRenderJob(t, jobRepository)
	if claimed.ID != job.ID {
		t.Fatalf("claimed job = %s, want %s", claimed.ID, job.ID)
	}

	assetID := uuid.New()
	now := time.Now().UTC()
	asset := mediaasset.MediaAsset{
		ID:               assetID,
		OwnerID:          ownerID,
		ProjectID:        projectID,
		Kind:             mediaasset.KindVideo,
		Origin:           mediaasset.OriginSystem,
		ObjectKey:        "projects/" + projectID.String() + "/assets/" + assetID.String(),
		MimeType:         "video/mp4",
		ByteSize:         2048,
		SHA256:           strings.Repeat("b", 64),
		OriginalFilename: "render.mp4",
		Metadata:         json.RawMessage(`{"source":"render_export_v1"}`),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if _, err := NewMediaAssetRepository(pool).Create(ctx, asset); err != nil {
		t.Fatalf("create render media asset: %v", err)
	}

	repository := NewRenderArtifactRepository(pool)
	artifact := renderexport.RenderArtifact{
		ID:               uuid.New(),
		OwnerID:          ownerID,
		ProjectID:        projectID,
		JobID:            job.ID,
		SnapshotDigest:   strings.Repeat("a", 64),
		ProfileID:        renderexport.LocalProfileID,
		MediaAssetID:     asset.ID,
		ByteSize:         asset.ByteSize,
		SHA256:           asset.SHA256,
		MimeType:         asset.MimeType,
		DurationMS:       1000,
		Width:            320,
		Height:           180,
		ToolchainVersion: "ffmpeg version integration-test",
		CreatedAt:        now,
	}
	created, err := repository.CreateForLease(ctx, *claimed.LeaseToken, artifact)
	if err != nil {
		t.Fatalf("create render artifact: %v", err)
	}
	if created.ID != artifact.ID || created.MediaAssetID != asset.ID || created.JobID != job.ID {
		t.Fatalf("created artifact mismatch: got=%+v want=%+v", created, artifact)
	}

	fetched, err := repository.GetByJob(ctx, ownerID, projectID, job.ID)
	if err != nil || fetched.ID != artifact.ID {
		t.Fatalf("get artifact: artifact=%+v err=%v", fetched, err)
	}
	if _, err := repository.GetByJob(ctx, uuid.New(), projectID, job.ID); !errors.Is(err, renderexport.ErrArtifactNotFound) {
		t.Fatalf("cross-owner artifact leaked: %v", err)
	}

	duplicate := artifact
	duplicate.ID = uuid.New()
	if _, err := repository.CreateForLease(ctx, *claimed.LeaseToken, duplicate); !errors.Is(err, renderexport.ErrArtifactConflict) {
		t.Fatalf("expected one authoritative artifact per job, got %v", err)
	}

	staleToken := uuid.New()
	staleCandidate := artifact
	staleCandidate.ID = uuid.New()
	staleCandidate.MediaAssetID = uuid.New()
	staleAsset := asset
	staleAsset.ID = staleCandidate.MediaAssetID
	staleAsset.ObjectKey = "projects/" + projectID.String() + "/assets/" + staleAsset.ID.String()
	if _, err := NewMediaAssetRepository(pool).Create(ctx, staleAsset); err != nil {
		t.Fatalf("create stale candidate media asset: %v", err)
	}
	if _, err := repository.CreateForLease(ctx, staleToken, staleCandidate); !errors.Is(err, renderexport.ErrStaleRenderLease) {
		t.Fatalf("expected stale worker rejection, got %v", err)
	}
}

func TestRenderArtifactRepositoryIntegrationRejectsProvenanceMismatch(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	ownerID := uuid.New()
	projectRepository := NewProjectRepository(pool)
	projectItem, err := projectRepository.Create(ctx, ownerID, validIntegrationCreateInput("Render provenance"))
	if err != nil {
		t.Fatal(err)
	}
	projectID := projectItem.ID
	jobRepository := NewJobRepository(pool)
	job, err := jobRepository.Enqueue(ctx, jobs.EnqueueInput{
		ID:          uuid.New(),
		OwnerID:     ownerID,
		ProjectID:   &projectID,
		Kind:        renderexport.JobKind,
		MaxAttempts: 2,
		Payload:     canonicalRenderPayload(),
	})
	if err != nil {
		t.Fatal(err)
	}
	claimed := claimRenderJob(t, jobRepository)

	assetID := uuid.New()
	now := time.Now().UTC()
	asset := mediaasset.MediaAsset{
		ID:               assetID,
		OwnerID:          ownerID,
		ProjectID:        projectID,
		Kind:             mediaasset.KindVideo,
		Origin:           mediaasset.OriginSystem,
		ObjectKey:        "projects/" + projectID.String() + "/assets/" + assetID.String(),
		MimeType:         "video/mp4",
		ByteSize:         2048,
		SHA256:           strings.Repeat("b", 64),
		OriginalFilename: "render.mp4",
		Metadata:         json.RawMessage(`{}`),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if _, err := NewMediaAssetRepository(pool).Create(ctx, asset); err != nil {
		t.Fatal(err)
	}

	candidate := renderexport.RenderArtifact{
		ID:               uuid.New(),
		OwnerID:          ownerID,
		ProjectID:        projectID,
		JobID:            job.ID,
		SnapshotDigest:   strings.Repeat("c", 64),
		ProfileID:        renderexport.LocalProfileID,
		MediaAssetID:     asset.ID,
		ByteSize:         asset.ByteSize,
		SHA256:           asset.SHA256,
		MimeType:         asset.MimeType,
		DurationMS:       500,
		Width:            320,
		Height:           180,
		ToolchainVersion: "ffmpeg version integration-test",
		CreatedAt:        now,
	}
	if _, err := NewRenderArtifactRepository(pool).CreateForLease(ctx, *claimed.LeaseToken, candidate); !errors.Is(err, renderexport.ErrArtifactNotFound) {
		t.Fatalf("expected snapshot provenance mismatch rejection, got %v", err)
	}
}

func TestRenderArtifactRepositoryIntegrationRejectsCrossProjectAsset(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	ownerID := uuid.New()
	projectRepository := NewProjectRepository(pool)
	projectA, err := projectRepository.Create(ctx, ownerID, validIntegrationCreateInput("Render A"))
	if err != nil {
		t.Fatal(err)
	}
	projectB, err := projectRepository.Create(ctx, ownerID, validIntegrationCreateInput("Render B"))
	if err != nil {
		t.Fatal(err)
	}

	projectAID := projectA.ID
	jobRepository := NewJobRepository(pool)
	job, err := jobRepository.Enqueue(ctx, jobs.EnqueueInput{
		ID:          uuid.New(),
		OwnerID:     ownerID,
		ProjectID:   &projectAID,
		Kind:        renderexport.JobKind,
		MaxAttempts: 2,
		Payload:     canonicalRenderPayload(),
	})
	if err != nil {
		t.Fatal(err)
	}
	claimed := claimRenderJob(t, jobRepository)

	assetID := uuid.New()
	now := time.Now().UTC()
	asset := mediaasset.MediaAsset{
		ID:               assetID,
		OwnerID:          ownerID,
		ProjectID:        projectB.ID,
		Kind:             mediaasset.KindVideo,
		Origin:           mediaasset.OriginSystem,
		ObjectKey:        "projects/" + projectB.ID.String() + "/assets/" + assetID.String(),
		MimeType:         "video/mp4",
		ByteSize:         1,
		SHA256:           strings.Repeat("c", 64),
		OriginalFilename: "render.mp4",
		Metadata:         json.RawMessage(`{}`),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if _, err := NewMediaAssetRepository(pool).Create(ctx, asset); err != nil {
		t.Fatal(err)
	}

	_, err = NewRenderArtifactRepository(pool).CreateForLease(ctx, *claimed.LeaseToken, renderexport.RenderArtifact{
		ID:               uuid.New(),
		OwnerID:          ownerID,
		ProjectID:        projectA.ID,
		JobID:            job.ID,
		SnapshotDigest:   strings.Repeat("a", 64),
		ProfileID:        renderexport.LocalProfileID,
		MediaAssetID:     asset.ID,
		ByteSize:         1,
		SHA256:           strings.Repeat("c", 64),
		MimeType:         "video/mp4",
		DurationMS:       500,
		Width:            320,
		Height:           180,
		ToolchainVersion: "ffmpeg version integration-test",
		CreatedAt:        now,
	})
	if !errors.Is(err, renderexport.ErrArtifactNotFound) {
		t.Fatalf("expected cross-project asset rejection, got %v", err)
	}
}

func claimRenderJob(t *testing.T, repository *JobRepository) jobs.Job {
	t.Helper()
	claimed, err := repository.ClaimNext(context.Background(), jobs.ClaimOptions{
		Kinds:         []string{renderexport.JobKind},
		LeaseDuration: time.Minute,
	})
	if err != nil {
		t.Fatalf("claim render job: %v", err)
	}
	if claimed.LeaseToken == nil || claimed.LeaseUntil == nil || claimed.State != jobs.StateRunning {
		t.Fatalf("claimed job missing active lease: %+v", claimed)
	}
	return claimed
}

func canonicalRenderPayload() json.RawMessage {
	return json.RawMessage(`{"snapshot_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","profile_id":"local_software_mp4_v1"}`)
}
