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

type SubtitleMode string

const (
	JobKind            = "render_export_v1"
	LocalProfileID     = "local_software_mp4_v1"
	DefaultMaxAttempts = 2

	SubtitleModeOff    SubtitleMode = "off"
	SubtitleModeWebVTT SubtitleMode = "webvtt"
)

var (
	ErrUnauthenticated           = errors.New("render export principal is required")
	ErrInvalidRequest            = errors.New("render export request is invalid")
	ErrSnapshotMismatch          = errors.New("render export snapshot identity mismatch")
	ErrRenderNotFound            = errors.New("render export job not found")
	ErrRenderArtifactUnavailable = errors.New("render export succeeded without a durable artifact")
	ErrRenderNotCancellable      = errors.New("render export job is not cancellable")
	ErrRenderNotRetryable        = errors.New("render export job is not retryable")
	ErrRetryRequestConflict      = errors.New("render retry request identity conflict")
	ErrInvalidHistoryCursor      = errors.New("render history cursor is invalid")
)

type SnapshotStore interface {
	GetSnapshot(ctx context.Context, ownerID, projectID uuid.UUID, digest string) (sceneeditor.Snapshot, error)
}

type JobQueue interface {
	Enqueue(ctx context.Context, input jobs.EnqueueInput) (jobs.Job, error)
}

type JobReader interface {
	GetByIDForProject(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, id uuid.UUID) (jobs.Job, error)
	GetByDedupeKey(ctx context.Context, ownerID uuid.UUID, kind string, dedupeKey string) (jobs.Job, error)
	ListByProjectKind(ctx context.Context, options jobs.ListByProjectKindOptions) ([]jobs.Job, *jobs.ListCursor, error)
	RequestCancel(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, id uuid.UUID) (jobs.Job, error)
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
	SnapshotDigest     string       `json:"snapshot_digest"`
	SnapshotSchema     int          `json:"snapshot_schema"`
	ProfileID          string       `json:"profile_id"`
	SubtitleMode       SubtitleMode `json:"subtitle_mode"`
	RetryOfRenderJobID *string      `json:"retry_of_render_job_id,omitempty"`
}

type JobView struct {
	ID                  uuid.UUID       `json:"id"`
	State               jobs.State      `json:"state"`
	Attempt             int             `json:"attempt"`
	MaxAttempts         int             `json:"max_attempts"`
	ErrorCode           *string         `json:"error_code,omitempty"`
	SnapshotDigest      string          `json:"snapshot_digest"`
	ProfileID           string          `json:"profile_id"`
	SubtitleMode        SubtitleMode    `json:"subtitle_mode"`
	RetryOfRenderJobID  *uuid.UUID      `json:"retry_of_render_job_id,omitempty"`
	CancellationPending bool            `json:"cancellation_pending"`
	Artifact            *RenderArtifact `json:"artifact,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

type HistoryResult struct {
	Items      []JobView `json:"items"`
	NextCursor string    `json:"next_cursor,omitempty"`
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

func (s *Service) Enqueue(ctx context.Context, ownerID, projectID uuid.UUID, snapshotDigest string, requestedMode ...SubtitleMode) (jobs.Job, error) {
	if ownerID == uuid.Nil {
		return jobs.Job{}, ErrUnauthenticated
	}
	if projectID == uuid.Nil || !validDigest(snapshotDigest) {
		return jobs.Job{}, ErrInvalidRequest
	}
	subtitleMode, err := normalizeSubtitleMode(requestedMode)
	if err != nil {
		return jobs.Job{}, err
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
		SubtitleMode:   subtitleMode,
	})
	if err != nil {
		return jobs.Job{}, fmt.Errorf("marshal render payload: %w", err)
	}

	dedupeParts := []string{"render", projectID.String(), snapshot.Digest, LocalProfileID}
	if subtitleMode != SubtitleModeOff {
		dedupeParts = append(dedupeParts, string(subtitleMode))
	}
	dedupe := strings.Join(dedupeParts, ":")
	job, err := s.jobs.Enqueue(ctx, jobs.EnqueueInput{
		ID:          s.newID(),
		OwnerID:     ownerID,
		ProjectID:   &projectID,
		Kind:        JobKind,
		DedupeKey:   &dedupe,
		MaxAttempts: DefaultMaxAttempts,
		Payload:     payload,
	})
	if err != nil {
		if errors.Is(err, jobs.ErrDuplicateJob) && s.reader != nil {
			existing, lookupErr := s.reader.GetByDedupeKey(ctx, ownerID, JobKind, dedupe)
			if lookupErr != nil {
				return jobs.Job{}, lookupErr
			}
			return existing, nil
		}
		return jobs.Job{}, err
	}
	return job, nil
}

func (s *Service) Cancel(ctx context.Context, ownerID, projectID, jobID uuid.UUID) (JobView, error) {
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
	if job.Kind != JobKind {
		return JobView{}, ErrRenderNotFound
	}
	if jobs.IsTerminalState(job.State) {
		if job.State == jobs.StateCancelled {
			return s.jobToView(ctx, ownerID, projectID, job)
		}
		return JobView{}, ErrRenderNotCancellable
	}
	if !jobs.IsCancellableState(job.State) {
		return JobView{}, ErrRenderNotCancellable
	}
	cancelled, err := s.reader.RequestCancel(ctx, ownerID, projectID, jobID)
	if err != nil {
		if errors.Is(err, jobs.ErrJobTerminal) {
			if cancelled.State == jobs.StateCancelled {
				return s.jobToView(ctx, ownerID, projectID, cancelled)
			}
			return JobView{}, ErrRenderNotCancellable
		}
		if errors.Is(err, jobs.ErrJobNotFound) {
			return JobView{}, ErrRenderNotFound
		}
		return JobView{}, err
	}
	return s.jobToView(ctx, ownerID, projectID, cancelled)
}

func (s *Service) Retry(ctx context.Context, ownerID, projectID, sourceJobID, requestID uuid.UUID) (JobView, error) {
	if ownerID == uuid.Nil {
		return JobView{}, ErrUnauthenticated
	}
	if projectID == uuid.Nil || sourceJobID == uuid.Nil || requestID == uuid.Nil || s.reader == nil {
		return JobView{}, ErrInvalidRequest
	}
	if existing, err := s.reader.GetByIDForProject(ctx, ownerID, projectID, requestID); err == nil {
		if existing.Kind != JobKind {
			return JobView{}, ErrRetryRequestConflict
		}
		payload, decodeErr := decodeRenderPayload(existing.Payload)
		if decodeErr != nil || payload.RetryOfRenderJobID == nil || *payload.RetryOfRenderJobID != sourceJobID.String() {
			return JobView{}, ErrRetryRequestConflict
		}
		return s.jobToView(ctx, ownerID, projectID, existing)
	} else if !errors.Is(err, jobs.ErrJobNotFound) {
		return JobView{}, err
	}

	source, err := s.reader.GetByIDForProject(ctx, ownerID, projectID, sourceJobID)
	if err != nil {
		if errors.Is(err, jobs.ErrJobNotFound) {
			return JobView{}, ErrRenderNotFound
		}
		return JobView{}, err
	}
	if source.Kind != JobKind {
		return JobView{}, ErrRenderNotFound
	}
	if !jobs.IsTerminalState(source.State) {
		return JobView{}, ErrRenderNotRetryable
	}
	sourcePayload, err := decodeRenderPayload(source.Payload)
	if err != nil {
		return JobView{}, err
	}
	retryPayload, err := json.Marshal(RenderPayload{
		SnapshotDigest:     sourcePayload.SnapshotDigest,
		SnapshotSchema:     sourcePayload.SnapshotSchema,
		ProfileID:          sourcePayload.ProfileID,
		SubtitleMode:       sourcePayload.SubtitleMode,
		RetryOfRenderJobID: stringPtr(sourceJobID.String()),
	})
	if err != nil {
		return JobView{}, fmt.Errorf("marshal retry payload: %w", err)
	}
	dedupe := strings.Join([]string{"render-retry", sourceJobID.String(), requestID.String()}, ":")
	job, err := s.jobs.Enqueue(ctx, jobs.EnqueueInput{
		ID:          requestID,
		OwnerID:     ownerID,
		ProjectID:   &projectID,
		Kind:        JobKind,
		DedupeKey:   &dedupe,
		MaxAttempts: DefaultMaxAttempts,
		Payload:     retryPayload,
	})
	if err != nil {
		if errors.Is(err, jobs.ErrDuplicateJob) {
			existing, lookupErr := s.reader.GetByDedupeKey(ctx, ownerID, JobKind, dedupe)
			if lookupErr != nil {
				return JobView{}, lookupErr
			}
			return s.jobToView(ctx, ownerID, projectID, existing)
		}
		return JobView{}, err
	}
	return s.jobToView(ctx, ownerID, projectID, job)
}

func (s *Service) ListHistory(ctx context.Context, ownerID, projectID uuid.UUID, limit int, cursorValue string) (HistoryResult, error) {
	if ownerID == uuid.Nil {
		return HistoryResult{}, ErrUnauthenticated
	}
	if projectID == uuid.Nil || s.reader == nil {
		return HistoryResult{}, ErrInvalidRequest
	}
	var cursor *jobs.ListCursor
	if cursorValue != "" {
		decoded, err := DecodeHistoryCursor(cursorValue)
		if err != nil {
			return HistoryResult{}, ErrInvalidHistoryCursor
		}
		cursor = &decoded
	}
	items, next, err := s.reader.ListByProjectKind(ctx, jobs.ListByProjectKindOptions{
		OwnerID:   ownerID,
		ProjectID: projectID,
		Kind:      JobKind,
		Limit:     limit,
		Cursor:    cursor,
	})
	if err != nil {
		return HistoryResult{}, err
	}
	views := make([]JobView, 0, len(items))
	for _, job := range items {
		view, err := s.jobToView(ctx, ownerID, projectID, job)
		if err != nil {
			return HistoryResult{}, err
		}
		views = append(views, view)
	}
	result := HistoryResult{Items: views}
	if next != nil {
		encoded, err := EncodeHistoryCursor(*next)
		if err != nil {
			return HistoryResult{}, err
		}
		result.NextCursor = encoded
	}
	return result, nil
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
	return s.jobToView(ctx, ownerID, projectID, job)
}

func (s *Service) jobToView(ctx context.Context, ownerID, projectID uuid.UUID, job jobs.Job) (JobView, error) {
	payload, err := decodeRenderPayload(job.Payload)
	if err != nil {
		return JobView{}, err
	}
	view := JobView{
		ID:                  job.ID,
		State:               job.State,
		Attempt:             job.Attempt,
		MaxAttempts:         job.MaxAttempts,
		ErrorCode:           job.ErrorCode,
		SnapshotDigest:      payload.SnapshotDigest,
		ProfileID:           payload.ProfileID,
		SubtitleMode:        payload.SubtitleMode,
		CancellationPending: job.State == jobs.StateRunning && job.CancelRequestedAt != nil,
		CreatedAt:           job.CreatedAt,
		UpdatedAt:           job.UpdatedAt,
	}
	if payload.RetryOfRenderJobID != nil {
		retryID, parseErr := uuid.Parse(*payload.RetryOfRenderJobID)
		if parseErr != nil {
			return JobView{}, ErrInvalidRequest
		}
		view.RetryOfRenderJobID = &retryID
	}
	if job.State != jobs.StateSucceeded {
		return view, nil
	}
	if s.artifacts == nil {
		return JobView{}, ErrRenderArtifactUnavailable
	}
	artifact, err := s.artifacts.GetByJob(ctx, ownerID, projectID, job.ID)
	if err != nil {
		if errors.Is(err, ErrArtifactNotFound) {
			return JobView{}, ErrRenderArtifactUnavailable
		}
		return JobView{}, err
	}
	if artifact.JobID != job.ID || artifact.ProjectID != projectID || artifact.SnapshotDigest != payload.SnapshotDigest || artifact.ProfileID != payload.ProfileID {
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
	mode, err := normalizeSubtitleMode([]SubtitleMode{payload.SubtitleMode})
	if err != nil {
		return RenderPayload{}, err
	}
	payload.SubtitleMode = mode
	if payload.RetryOfRenderJobID != nil {
		retryID, err := uuid.Parse(*payload.RetryOfRenderJobID)
		if err != nil || retryID == uuid.Nil {
			return RenderPayload{}, ErrInvalidRequest
		}
	}
	return payload, nil
}

func normalizeSubtitleMode(requested []SubtitleMode) (SubtitleMode, error) {
	if len(requested) > 1 {
		return "", ErrInvalidRequest
	}
	if len(requested) == 0 || strings.TrimSpace(string(requested[0])) == "" {
		return SubtitleModeOff, nil
	}
	mode := SubtitleMode(strings.ToLower(strings.TrimSpace(string(requested[0]))))
	switch mode {
	case SubtitleModeOff, SubtitleModeWebVTT:
		return mode, nil
	default:
		return "", ErrInvalidRequest
	}
}

func stringPtr(value string) *string {
	return &value
}

func validDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}
