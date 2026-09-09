package renderexport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

const (
	LocalProfileWidth     = 1280
	LocalProfileHeight    = 720
	LocalProfileFrameRate = 30
	MaxRenderInputBytes   = 512 << 20
	MaxFinalMP4Bytes      = 512 << 20

	ErrorInvalidPayload      = "ERR_RENDER_INVALID_PAYLOAD"
	ErrorSnapshotInvalid     = "ERR_RENDER_SNAPSHOT_INVALID"
	ErrorInputUnavailable    = "ERR_RENDER_INPUT_UNAVAILABLE"
	ErrorUnsupportedSnapshot = "ERR_RENDER_UNSUPPORTED_SNAPSHOT"
	ErrorRendererFailed      = "ERR_RENDER_PROCESS_FAILED"
	ErrorStorageFailed       = "ERR_RENDER_STORAGE_FAILED"
	ErrorFinalizeFailed      = "ERR_RENDER_FINALIZE_FAILED"
	ErrorStaleLease          = "ERR_RENDER_STALE_LEASE"
)

type RenderAssets interface {
	Get(ctx context.Context, principal project.Principal, projectID, assetID uuid.UUID) (mediaasset.MediaAsset, error)
	Open(ctx context.Context, principal project.Principal, projectID, assetID uuid.UUID) (io.ReadCloser, error)
	FindFinalByJob(ctx context.Context, principal project.Principal, projectID, jobID uuid.UUID) (mediaasset.MediaAsset, error)
	Store(ctx context.Context, principal project.Principal, projectID uuid.UUID, input mediaasset.CreateInput) (mediaasset.MediaAsset, error)
	Delete(ctx context.Context, principal project.Principal, projectID, assetID uuid.UUID) error
}

type LocalRenderFunc func(context.Context, RenderProcessRunner, FFmpegProfile, PreparedLocalRenderInput) (RenderMetadata, error)

type Handler struct {
	snapshots SnapshotStore
	assets    RenderAssets
	artifacts ArtifactRepository
	profile   FFmpegProfile
	render    LocalRenderFunc
	newID     IDGenerator
	now       func() time.Time
}

func NewHandler(snapshots SnapshotStore, assets RenderAssets, artifacts ArtifactRepository, profile FFmpegProfile) *Handler {
	return &Handler{
		snapshots: snapshots,
		assets:    assets,
		artifacts: artifacts,
		profile:   profile,
		render:    RenderSingleVisualMP4,
		newID:     uuid.New,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

type HandlerResult struct {
	ArtifactID     uuid.UUID `json:"artifact_id"`
	MediaAssetID   uuid.UUID `json:"media_asset_id"`
	SnapshotDigest string    `json:"snapshot_digest"`
	ProfileID      string    `json:"profile_id"`
}

type renderOutputMetadata struct {
	Source           string `json:"source"`
	RenderJobID      string `json:"render_job_id"`
	SnapshotDigest   string `json:"snapshot_digest"`
	ProfileID        string `json:"profile_id"`
	DurationMS       int64  `json:"duration_ms"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	ToolchainVersion string `json:"toolchain_version"`
}

func (h *Handler) Handle(ctx context.Context, job jobs.Job) (json.RawMessage, error) {
	projectID, payload, err := validateRenderJob(job)
	if err != nil {
		return nil, jobs.NewTerminalError(ErrorInvalidPayload, err)
	}
	if h.snapshots == nil || h.assets == nil || h.artifacts == nil || h.render == nil {
		return nil, jobs.NewTerminalError(ErrorRendererFailed, errors.New("render runtime unavailable"))
	}
	if h.profile.ID != LocalProfileID || h.profile.VideoEncoder != "libx264" || h.profile.AudioEncoder != "aac" || h.profile.PixelFormat != "yuv420p" || h.profile.Container != "mp4" || h.profile.VersionLine == "" {
		return nil, jobs.NewTerminalError(ErrorRendererFailed, ErrUnsupportedFFmpeg)
	}
	principal := project.Principal{OwnerID: job.OwnerID}

	if artifact, getErr := h.artifacts.GetByJob(ctx, job.OwnerID, projectID, job.ID); getErr == nil {
		return h.resultForArtifact(artifact, payload)
	} else if !errors.Is(getErr, ErrArtifactNotFound) {
		return nil, jobs.NewRetryableError(ErrorFinalizeFailed, getErr, nil)
	}

	snapshot, err := h.snapshots.GetSnapshot(ctx, job.OwnerID, projectID, payload.SnapshotDigest)
	if err != nil {
		return nil, classifySnapshotError(err)
	}
	if snapshot.ProjectID != projectID || snapshot.Digest != payload.SnapshotDigest || snapshot.SchemaVersion != payload.SnapshotSchema {
		return nil, jobs.NewTerminalError(ErrorSnapshotInvalid, ErrSnapshotMismatch)
	}
	if err := sceneeditor.VerifySnapshotIntegrity(snapshot); err != nil {
		return nil, jobs.NewTerminalError(ErrorSnapshotInvalid, err)
	}
	scene, err := supportedSingleVisualScene(snapshot)
	if err != nil {
		return nil, jobs.NewTerminalError(ErrorUnsupportedSnapshot, err)
	}

	if existing, findErr := h.assets.FindFinalByJob(ctx, principal, projectID, job.ID); findErr == nil {
		artifact, finalizeErr := h.finalizeAsset(ctx, job, payload, existing)
		if finalizeErr != nil {
			return nil, finalizeErr
		}
		return h.resultForArtifact(artifact, payload)
	} else if !errors.Is(findErr, mediaasset.ErrNotFound) {
		return nil, jobs.NewRetryableError(ErrorStorageFailed, findErr, nil)
	}

	visualAsset, err := h.assets.Get(ctx, principal, projectID, scene.Visual.AssetID)
	if err != nil {
		return nil, classifyAssetReadError(err)
	}
	if visualAsset.Kind != mediaasset.KindImage || visualAsset.ByteSize <= 0 || visualAsset.ByteSize > MaxRenderInputBytes || !validDigest(visualAsset.SHA256) {
		return nil, jobs.NewTerminalError(ErrorInputUnavailable, errors.New("visual asset is not an accepted bounded image input"))
	}

	workDir, err := os.MkdirTemp("", "synvideo-render-*")
	if err != nil {
		return nil, jobs.NewRetryableError(ErrorRendererFailed, err, nil)
	}
	defer os.RemoveAll(workDir)
	visualPath := filepath.Join(workDir, "visual.input")
	outputPath := filepath.Join(workDir, "render.mp4")
	if err := h.materializeVerifiedAsset(ctx, principal, projectID, visualAsset, visualPath); err != nil {
		return nil, err
	}

	metadata, err := h.render(ctx, nil, h.profile, PreparedLocalRenderInput{
		VisualPath: visualPath,
		OutputPath: outputPath,
		Width:      LocalProfileWidth,
		Height:     LocalProfileHeight,
		FrameRate:  LocalProfileFrameRate,
		DurationMS: scene.DurationMS,
		Fit:        scene.VisualTreatment.Fit,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if errors.Is(err, ErrUnsupportedRenderSemantics) || errors.Is(err, ErrInvalidRenderInput) || errors.Is(err, ErrUnsupportedFFmpeg) {
			return nil, jobs.NewTerminalError(ErrorUnsupportedSnapshot, err)
		}
		return nil, jobs.NewRetryableError(ErrorRendererFailed, err, nil)
	}
	if metadata.ByteSize <= 0 || metadata.ByteSize > MaxFinalMP4Bytes {
		return nil, jobs.NewTerminalError(ErrorRendererFailed, ErrInvalidRenderOutput)
	}

	output, err := os.Open(outputPath)
	if err != nil {
		return nil, jobs.NewRetryableError(ErrorRendererFailed, err, nil)
	}
	defer output.Close()
	assetMetadata, err := json.Marshal(renderOutputMetadata{
		Source:           JobKind,
		RenderJobID:      job.ID.String(),
		SnapshotDigest:   payload.SnapshotDigest,
		ProfileID:        payload.ProfileID,
		DurationMS:       metadata.DurationMS,
		Width:            metadata.Width,
		Height:           metadata.Height,
		ToolchainVersion: metadata.ToolchainVersion,
	})
	if err != nil {
		return nil, jobs.NewTerminalError(ErrorInvalidPayload, err)
	}
	finalAsset, err := h.assets.Store(ctx, principal, projectID, mediaasset.CreateInput{
		Kind:             mediaasset.KindVideo,
		Origin:           mediaasset.OriginSystem,
		MimeType:         "video/mp4",
		OriginalFilename: "render.mp4",
		Metadata:         assetMetadata,
		Reader:           output,
		MaxBytes:         MaxFinalMP4Bytes,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if existing, findErr := h.assets.FindFinalByJob(ctx, principal, projectID, job.ID); findErr == nil {
			artifact, finalizeErr := h.finalizeAsset(ctx, job, payload, existing)
			if finalizeErr != nil {
				return nil, finalizeErr
			}
			return h.resultForArtifact(artifact, payload)
		}
		return nil, jobs.NewRetryableError(ErrorStorageFailed, err, nil)
	}
	if finalAsset.ByteSize != metadata.ByteSize || finalAsset.MimeType != "video/mp4" || !validDigest(finalAsset.SHA256) {
		_ = h.assets.Delete(context.Background(), principal, projectID, finalAsset.ID)
		return nil, jobs.NewTerminalError(ErrorStorageFailed, errors.New("stored render output metadata mismatch"))
	}

	artifact, err := h.finalizeAsset(ctx, job, payload, finalAsset)
	if err != nil {
		return nil, err
	}
	return h.resultForArtifact(artifact, payload)
}

func validateRenderJob(job jobs.Job) (uuid.UUID, RenderPayload, error) {
	if job.Kind != JobKind || job.OwnerID == uuid.Nil || job.ProjectID == nil || *job.ProjectID == uuid.Nil || job.LeaseToken == nil || *job.LeaseToken == uuid.Nil {
		return uuid.Nil, RenderPayload{}, ErrInvalidRequest
	}
	var payload RenderPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil || !validDigest(payload.SnapshotDigest) || payload.SnapshotSchema != sceneeditor.SnapshotSchemaVersion || payload.ProfileID != LocalProfileID {
		return uuid.Nil, RenderPayload{}, ErrInvalidRequest
	}
	return *job.ProjectID, payload, nil
}

func supportedSingleVisualScene(snapshot sceneeditor.Snapshot) (sceneeditor.Scene, error) {
	if len(snapshot.Scenes) != 1 || snapshot.AudioMix != nil {
		return sceneeditor.Scene{}, ErrUnsupportedRenderSemantics
	}
	scene := snapshot.Scenes[0]
	if scene.Visual == nil || scene.Narration != nil || scene.Caption != nil || scene.DurationMS <= 0 || scene.DurationMS > MaxLocalRenderDuration.Milliseconds() {
		return sceneeditor.Scene{}, ErrUnsupportedRenderSemantics
	}
	if scene.VisualTreatment.Fit != sceneeditor.FitContain || scene.VisualTreatment.Crop != nil || scene.VisualTreatment.PositionX != 0 || scene.VisualTreatment.PositionY != 0 || scene.VisualTreatment.Scale != 1 {
		return sceneeditor.Scene{}, ErrUnsupportedRenderSemantics
	}
	if scene.TransitionOut.Kind != sceneeditor.TransitionCut || scene.TransitionOut.DurationMS != 0 {
		return sceneeditor.Scene{}, ErrUnsupportedRenderSemantics
	}
	return scene, nil
}

func (h *Handler) materializeVerifiedAsset(ctx context.Context, principal project.Principal, projectID uuid.UUID, asset mediaasset.MediaAsset, path string) error {
	reader, err := h.assets.Open(ctx, principal, projectID, asset.ID)
	if err != nil {
		return classifyAssetReadError(err)
	}
	defer reader.Close()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return jobs.NewRetryableError(ErrorRendererFailed, err, nil)
	}
	hash := sha256.New()
	written, copyErr := copyContextBounded(ctx, io.MultiWriter(file, hash), reader, asset.ByteSize)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		if errors.Is(copyErr, context.Canceled) || errors.Is(copyErr, context.DeadlineExceeded) {
			return copyErr
		}
		return jobs.NewTerminalError(ErrorInputUnavailable, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return jobs.NewRetryableError(ErrorRendererFailed, closeErr, nil)
	}
	if written != asset.ByteSize || hex.EncodeToString(hash.Sum(nil)) != asset.SHA256 {
		_ = os.Remove(path)
		return jobs.NewTerminalError(ErrorInputUnavailable, errors.New("visual asset integrity mismatch"))
	}
	return nil
}

func copyContextBounded(ctx context.Context, dst io.Writer, src io.Reader, expected int64) (int64, error) {
	if expected <= 0 || expected > MaxRenderInputBytes {
		return 0, errors.New("render input size outside allowed bound")
	}
	buf := make([]byte, 64*1024)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, err := src.Read(buf)
		if n > 0 {
			total += int64(n)
			if total > expected {
				return total, errors.New("render input exceeds recorded byte size")
			}
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return total, writeErr
			}
		}
		if errors.Is(err, io.EOF) {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}

func (h *Handler) finalizeAsset(ctx context.Context, job jobs.Job, payload RenderPayload, asset mediaasset.MediaAsset) (RenderArtifact, error) {
	metadata, err := parseRenderOutputMetadata(asset, job.ID, payload)
	if err != nil {
		return RenderArtifact{}, jobs.NewTerminalError(ErrorStorageFailed, err)
	}
	artifact := RenderArtifact{
		ID:               h.newID(),
		OwnerID:          job.OwnerID,
		ProjectID:        *job.ProjectID,
		JobID:            job.ID,
		SnapshotDigest:   payload.SnapshotDigest,
		ProfileID:        payload.ProfileID,
		MediaAssetID:     asset.ID,
		ByteSize:         asset.ByteSize,
		SHA256:           asset.SHA256,
		MimeType:         asset.MimeType,
		DurationMS:       metadata.DurationMS,
		Width:            metadata.Width,
		Height:           metadata.Height,
		ToolchainVersion: metadata.ToolchainVersion,
		CreatedAt:        h.now(),
	}
	created, createErr := h.artifacts.CreateForLease(ctx, *job.LeaseToken, artifact)
	if createErr == nil {
		return created, nil
	}
	if errors.Is(createErr, ErrRenderCancelFenced) {
		principal := project.Principal{OwnerID: job.OwnerID}
		if deleteErr := h.assets.Delete(context.Background(), principal, *job.ProjectID, asset.ID); deleteErr != nil {
			return RenderArtifact{}, jobs.NewRetryableError(ErrorStorageFailed, deleteErr, nil)
		}
		return RenderArtifact{}, context.Canceled
	}
	if existing, getErr := h.artifacts.GetByJob(ctx, job.OwnerID, *job.ProjectID, job.ID); getErr == nil {
		if existing.SnapshotDigest != payload.SnapshotDigest || existing.ProfileID != payload.ProfileID {
			return RenderArtifact{}, jobs.NewTerminalError(ErrorFinalizeFailed, ErrSnapshotMismatch)
		}
		if existing.MediaAssetID != asset.ID {
			principal := project.Principal{OwnerID: job.OwnerID}
			_ = h.assets.Delete(context.Background(), principal, *job.ProjectID, asset.ID)
		}
		return existing, nil
	}
	if errors.Is(createErr, ErrStaleRenderLease) {
		return RenderArtifact{}, jobs.NewRetryableError(ErrorStaleLease, createErr, nil)
	}
	if errors.Is(createErr, ErrArtifactConflict) {
		return RenderArtifact{}, jobs.NewRetryableError(ErrorFinalizeFailed, createErr, nil)
	}
	return RenderArtifact{}, jobs.NewRetryableError(ErrorFinalizeFailed, createErr, nil)
}

func parseRenderOutputMetadata(asset mediaasset.MediaAsset, jobID uuid.UUID, payload RenderPayload) (renderOutputMetadata, error) {
	if asset.Kind != mediaasset.KindVideo || asset.Origin != mediaasset.OriginSystem || asset.MimeType != "video/mp4" || asset.ByteSize <= 0 || !validDigest(asset.SHA256) {
		return renderOutputMetadata{}, errors.New("render output asset provenance is invalid")
	}
	var metadata renderOutputMetadata
	if err := json.Unmarshal(asset.Metadata, &metadata); err != nil {
		return renderOutputMetadata{}, errors.New("render output metadata is invalid")
	}
	if metadata.Source != JobKind || metadata.RenderJobID != jobID.String() || metadata.SnapshotDigest != payload.SnapshotDigest || metadata.ProfileID != payload.ProfileID || metadata.DurationMS <= 0 || metadata.Width <= 0 || metadata.Height <= 0 || metadata.ToolchainVersion == "" {
		return renderOutputMetadata{}, errors.New("render output provenance does not match job")
	}
	return metadata, nil
}

func (h *Handler) resultForArtifact(artifact RenderArtifact, payload RenderPayload) (json.RawMessage, error) {
	if artifact.SnapshotDigest != payload.SnapshotDigest || artifact.ProfileID != payload.ProfileID || artifact.ID == uuid.Nil || artifact.MediaAssetID == uuid.Nil {
		return nil, jobs.NewTerminalError(ErrorFinalizeFailed, ErrSnapshotMismatch)
	}
	result, err := json.Marshal(HandlerResult{
		ArtifactID:     artifact.ID,
		MediaAssetID:   artifact.MediaAssetID,
		SnapshotDigest: artifact.SnapshotDigest,
		ProfileID:      artifact.ProfileID,
	})
	if err != nil {
		return nil, jobs.NewTerminalError(ErrorFinalizeFailed, err)
	}
	return result, nil
}

func classifySnapshotError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, sceneeditor.ErrNotFound) || errors.Is(err, sceneeditor.ErrInvalidInput) {
		return jobs.NewTerminalError(ErrorSnapshotInvalid, err)
	}
	return jobs.NewRetryableError(ErrorSnapshotInvalid, err, nil)
}

func classifyAssetReadError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, mediaasset.ErrNotFound) || errors.Is(err, mediaasset.ErrObjectNotFound) {
		return jobs.NewTerminalError(ErrorInputUnavailable, err)
	}
	return jobs.NewRetryableError(ErrorInputUnavailable, err, nil)
}

func (h *Handler) String() string {
	return fmt.Sprintf("renderexport.Handler[%s]", h.profile.ID)
}
