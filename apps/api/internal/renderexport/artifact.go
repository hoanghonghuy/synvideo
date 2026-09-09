package renderexport

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrArtifactNotFound   = errors.New("render export artifact not found")
	ErrArtifactConflict   = errors.New("render export artifact already exists")
	ErrStaleRenderLease   = errors.New("render export worker lease is stale")
	ErrRenderCancelFenced = errors.New("render export finalization lost to durable cancellation")
)

var artifactSHA256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type RenderArtifact struct {
	ID               uuid.UUID `json:"id"`
	OwnerID          uuid.UUID `json:"-"`
	ProjectID        uuid.UUID `json:"project_id"`
	JobID            uuid.UUID `json:"job_id"`
	SnapshotDigest   string    `json:"snapshot_digest"`
	ProfileID        string    `json:"profile_id"`
	MediaAssetID     uuid.UUID `json:"media_asset_id"`
	ByteSize         int64     `json:"byte_size"`
	SHA256           string    `json:"sha256"`
	MimeType         string    `json:"mime_type"`
	DurationMS       int64     `json:"duration_ms"`
	Width            int       `json:"width"`
	Height           int       `json:"height"`
	ToolchainVersion string    `json:"toolchain_version"`
	CreatedAt        time.Time `json:"created_at"`
}

func (a RenderArtifact) Validate() error {
	if a.ID == uuid.Nil || a.OwnerID == uuid.Nil || a.ProjectID == uuid.Nil || a.JobID == uuid.Nil || a.MediaAssetID == uuid.Nil {
		return ErrInvalidRequest
	}
	if !validDigest(a.SnapshotDigest) || a.ProfileID != LocalProfileID {
		return ErrInvalidRequest
	}
	if a.ByteSize <= 0 || !artifactSHA256Pattern.MatchString(a.SHA256) || a.MimeType != "video/mp4" {
		return ErrInvalidRequest
	}
	if a.DurationMS <= 0 || a.Width <= 0 || a.Height <= 0 || strings.TrimSpace(a.ToolchainVersion) == "" || a.CreatedAt.IsZero() {
		return ErrInvalidRequest
	}
	return nil
}

type ArtifactRepository interface {
	CreateForLease(ctx context.Context, leaseToken uuid.UUID, artifact RenderArtifact) (RenderArtifact, error)
	GetByJob(ctx context.Context, ownerID, projectID, jobID uuid.UUID) (RenderArtifact, error)
}
