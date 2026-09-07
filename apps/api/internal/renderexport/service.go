package renderexport

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

const (
	JobKind            = "render_export_v1"
	LocalProfileID     = "local_software_mp4_v1"
	DefaultMaxAttempts = 2
)

var (
	ErrUnauthenticated           = errors.New("render export principal is required")
	ErrInvalidRequest            = errors.New("render export request is invalid")
	ErrSnapshotMismatch          = errors.New("render export snapshot identity mismatch")
	ErrRenderNotFound            = errors.New("render export job not found")
	ErrRenderArtifactUnavailable = errors.New("render export succeeded without a durable artifact")
)

type SnapshotStore interface {
	GetSnapshot(ctx context.Context, ownerID, projectID uuid.UUID, digest string) (sceneeditor.Snapshot, error)
}

type JobQueue interface {
	Enqueue(ctx context.Context, input jobs.EnqueueInput) (jobs.Job, error)
}

type JobReader interface {
	GetByIDForProject(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, id uuid.UUID) (jobs.Job, error)
}

type IDGenerator func() uuid.UUID

type Service struct {
	snapshots SnapshotStore
	jobs      JobQueue
	reader    JobReader
	artifacts ArtifactRepository
	newID     IDGenerator
}

type RenderPayload struct {
	SnapshotDigest string `json:"snapshot_digest"`
	SnapshotSchema int    `json:"snapshot_schema"`
	ProfileID      string `json:"profile_id"`
}

type JobView struct {
	ID             uuid.UUID       `json:"id"`
	State          jobs.State      `json:"state"`
	Attempt        int             `json:"attempt"`
	MaxAttempts    int             `json:"max_attempts"`
	ErrorCode      *string         `json:"error_code,omitempty"`
	SnapshotDigest string          `json:"snapshot_digest"`
	ProfileID      string          `json:"profile_id"`
	Artifact       *RenderArtifact `json:"artifact,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

func NewService(snapshots SnapshotStore, queue JobQueue, newID IDGenerator) *Service {
	return NewServiceWithRuntime(snapshots, queue, nil, nil, newID)
}

func NewServiceWithRuntime(snapshots SnapshotStore, queue JobQueue, reader JobReader, artifacts ArtifactRepository, newID IDGenerator) *Service {
	if newID == nil {
		newID = uuid.New
	}
	return &Service{snapshots: snapshots, jobs: queue, reader: reader, artifacts: artifacts, newID: newID}
}

func (s *Service) Enqueue(ctx context.Context, ownerID, projectID uuid.UUID, snapshotDigest string) (jobs.Job, error) {
	if ownerID == uuid.Nil {
		return jobs.Job{}, ErrUnauthenticated
	}
	if projectID == uuid.Nil || !validDigest(snapshotDigest) {
		return jobs.Job{}, ErrInvalidRequest
	}

	snapshot, err := s.snapshots.GetSnapshot(ctx, ownerID, projectID, snapshotDigest)
	if err != nil {
		return jobs.Job{}, err
	}
	if snapshot.ProjectID != projectID || snapshot.Digest != snapshotDigest || snapshot.SchemaVersion != sceneeditor.SnapshotSchemaVersion {
		return jobs.Job{}, ErrSnapshotMismatch
	}

	payload, err := json.Marshal(RenderPayload{
		SnapshotDigest: snapshot.Digest,
		SnapshotSchema: snapshot.SchemaVersion,
		ProfileID:      LocalProfileID,
	})
	if err != nil {
		return jobs.Job{}, fmt.Errorf("marshal render payload: %w", err)
	}

	dedupe := strings.Join([]string{"render", projectID.String(), snapshot.Digest, LocalProfileID}, ":")
	return s.jobs.Enqueue(ctx, jobs.EnqueueInput{
		ID:          s.newID(),
		OwnerID:     ownerID,
		ProjectID:   &projectID,
		Kind:        JobKind,
		DedupeKey:   &dedupe,
		MaxAttempts: DefaultMaxAttempts,
		Payload:     payload,
	})
}

func (s *Service) Get(ctx context.Context, ownerID, projectID, jobID uuid.UUID) (JobView, error) {
	if ownerID == uuid.Nil {
		return JobView{}, ErrUnauthenticated
	}
	if projectID == uuid.Nil || jobID == uuid.Nil || s.reader == nil {
		return JobView{}, ErrInvalidRequest
	}
	job, err := s.reader.GetByIDForProject(ctx, ownerID, projectID, jobID)
	if err != nil {
		if errors.Is(err, jobs.ErrJobNotFound) {
			return JobView{}, ErrRenderNotFound
		}
		return JobView{}, err
	}
	if job.Kind != JobKind || job.ProjectID == nil || *job.ProjectID != projectID {
		return JobView{}, ErrRenderNotFound
	}
	payload, err := decodeRenderPayload(job.Payload)
	if err != nil {
		return JobView{}, err
	}
	view := JobView{
		ID:             job.ID,
		State:          job.State,
		Attempt:        job.Attempt,
		MaxAttempts:    job.MaxAttempts,
		ErrorCode:      job.ErrorCode,
		SnapshotDigest: payload.SnapshotDigest,
		ProfileID:      payload.ProfileID,
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
	}
	if job.State != jobs.StateSucceeded {
		return view, nil
	}
	if s.artifacts == nil {
		return JobView{}, ErrRenderArtifactUnavailable
	}
	artifact, err := s.artifacts.GetByJob(ctx, ownerID, projectID, jobID)
	if err != nil {
		if errors.Is(err, ErrArtifactNotFound) {
			return JobView{}, ErrRenderArtifactUnavailable
		}
		return JobView{}, err
	}
	if artifact.JobID != jobID || artifact.ProjectID != projectID || artifact.SnapshotDigest != payload.SnapshotDigest || artifact.ProfileID != payload.ProfileID {
		return JobView{}, ErrRenderArtifactUnavailable
	}
	view.Artifact = &artifact
	return view, nil
}

func decodeRenderPayload(raw json.RawMessage) (RenderPayload, error) {
	var payload RenderPayload
	if err := json.Unmarshal(raw, &payload); err != nil || !validDigest(payload.SnapshotDigest) || payload.SnapshotSchema != sceneeditor.SnapshotSchemaVersion || payload.ProfileID != LocalProfileID {
		return RenderPayload{}, ErrInvalidRequest
	}
	return payload, nil
}

func validDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}
