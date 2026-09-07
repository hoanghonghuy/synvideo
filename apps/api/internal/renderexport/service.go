package renderexport

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

const (
	JobKind          = "render_export_v1"
	LocalProfileID   = "local_software_mp4_v1"
	DefaultMaxAttempts = 2
)

var (
	ErrUnauthenticated = errors.New("render export principal is required")
	ErrInvalidRequest  = errors.New("render export request is invalid")
	ErrSnapshotMismatch = errors.New("render export snapshot identity mismatch")
)

type SnapshotStore interface {
	GetSnapshot(ctx context.Context, ownerID, projectID uuid.UUID, digest string) (sceneeditor.Snapshot, error)
}

type JobQueue interface {
	Enqueue(ctx context.Context, input jobs.EnqueueInput) (jobs.Job, error)
}

type IDGenerator func() uuid.UUID

type Service struct {
	snapshots SnapshotStore
	jobs      JobQueue
	newID     IDGenerator
}

type RenderPayload struct {
	SnapshotDigest string `json:"snapshot_digest"`
	SnapshotSchema int    `json:"snapshot_schema"`
	ProfileID      string `json:"profile_id"`
}

func NewService(snapshots SnapshotStore, queue JobQueue, newID IDGenerator) *Service {
	if newID == nil {
		newID = uuid.New
	}
	return &Service{snapshots: snapshots, jobs: queue, newID: newID}
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

func validDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}
