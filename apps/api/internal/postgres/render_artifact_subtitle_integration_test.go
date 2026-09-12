package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/renderexport"
)

func TestRenderArtifactRepositoryIntegrationPersistsWebVTTSidecarProvenance(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	ownerID := uuid.New()
	projectItem, err := NewProjectRepository(pool).Create(ctx, ownerID, validIntegrationCreateInput("Render subtitle provenance"))
	if err != nil {
		t.Fatal(err)
	}
	projectID := projectItem.ID
	digest := strings.Repeat("a", 64)
	jobRepo := NewJobRepository(pool)
	job, err := jobRepo.Enqueue(ctx, jobs.EnqueueInput{
		ID: uuid.New(), OwnerID: ownerID, ProjectID: &projectID, Kind: renderexport.JobKind, MaxAttempts: 2,
		Payload: json.RawMessage(`{"snapshot_digest":"` + digest + `","profile_id":"local_software_mp4_v1","subtitle_mode":"webvtt"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	claimed := claimRenderJob(t, jobRepo)
	now := time.Now().UTC()
	mediaRepo := NewMediaAssetRepository(pool)

	videoID := uuid.New()
	video := mediaasset.MediaAsset{
		ID: videoID, OwnerID: ownerID, ProjectID: projectID, Kind: mediaasset.KindVideo, Origin: mediaasset.OriginSystem,
		ObjectKey: "projects/" + projectID.String() + "/assets/" + videoID.String(), MimeType: "video/mp4",
		ByteSize: 2048, SHA256: strings.Repeat("b", 64), OriginalFilename: "render.mp4",
		Metadata: json.RawMessage(`{"source":"render_export_v1"}`), CreatedAt: now, UpdatedAt: now,
	}
	if _, err := mediaRepo.Create(ctx, video); err != nil {
		t.Fatal(err)
	}

	subtitleID := uuid.New()
	subtitleMetadata, err := json.Marshal(map[string]string{
		"source": renderexport.JobKind, "render_job_id": job.ID.String(), "snapshot_digest": digest,
		"profile_id": renderexport.LocalProfileID, "output_role": "subtitle",
	})
	if err != nil {
		t.Fatal(err)
	}
	subtitle := mediaasset.MediaAsset{
		ID: subtitleID, OwnerID: ownerID, ProjectID: projectID, Kind: mediaasset.KindDocument, Origin: mediaasset.OriginSystem,
		ObjectKey: "projects/" + projectID.String() + "/assets/" + subtitleID.String(), MimeType: "text/vtt",
		ByteSize: 64, SHA256: strings.Repeat("c", 64), OriginalFilename: "render.vtt", Metadata: subtitleMetadata,
		CreatedAt: now, UpdatedAt: now,
	}
	if _, err := mediaRepo.Create(ctx, subtitle); err != nil {
		t.Fatal(err)
	}

	artifact := renderexport.RenderArtifact{
		ID: uuid.New(), OwnerID: ownerID, ProjectID: projectID, JobID: job.ID,
		SnapshotDigest: digest, ProfileID: renderexport.LocalProfileID, MediaAssetID: video.ID,
		SubtitleMediaAssetID: &subtitle.ID, ByteSize: video.ByteSize, SHA256: video.SHA256, MimeType: video.MimeType,
		DurationMS: 1000, Width: 320, Height: 180, ToolchainVersion: "ffmpeg version integration-test", CreatedAt: now,
	}
	repo := NewRenderArtifactRepository(pool)
	created, err := repo.CreateForLease(ctx, *claimed.LeaseToken, artifact)
	if err != nil {
		t.Fatalf("create render artifact with subtitle: %v", err)
	}
	if created.SubtitleMediaAssetID == nil || *created.SubtitleMediaAssetID != subtitle.ID {
		t.Fatalf("created subtitle provenance = %v, want %s", created.SubtitleMediaAssetID, subtitle.ID)
	}
	fetched, err := repo.GetByJob(ctx, ownerID, projectID, job.ID)
	if err != nil || fetched.SubtitleMediaAssetID == nil || *fetched.SubtitleMediaAssetID != subtitle.ID {
		t.Fatalf("fetched subtitle provenance=%v err=%v", fetched.SubtitleMediaAssetID, err)
	}
}
