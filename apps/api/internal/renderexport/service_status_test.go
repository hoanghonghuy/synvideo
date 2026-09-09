package renderexport

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

type jobReaderStub struct {
	job       jobs.Job
	err       error
	ownerID   uuid.UUID
	projectID uuid.UUID
	jobID     uuid.UUID
}

func (r *jobReaderStub) GetByIDForProject(_ context.Context, ownerID, projectID, jobID uuid.UUID) (jobs.Job, error) {
	r.ownerID = ownerID
	r.projectID = projectID
	r.jobID = jobID
	return r.job, r.err
}

func (r *jobReaderStub) GetByDedupeKey(context.Context, uuid.UUID, string, string) (jobs.Job, error) {
	return jobs.Job{}, jobs.ErrJobNotFound
}

func (r *jobReaderStub) ListByProjectKind(context.Context, jobs.ListByProjectKindOptions) ([]jobs.Job, *jobs.ListCursor, error) {
	return nil, nil, nil
}

func (r *jobReaderStub) RequestCancel(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (jobs.Job, error) {
	return jobs.Job{}, jobs.ErrJobNotFound
}

type artifactReaderStub struct {
	artifact  RenderArtifact
	err       error
	ownerID   uuid.UUID
	projectID uuid.UUID
	jobID     uuid.UUID
}

func (r *artifactReaderStub) CreateForLease(context.Context, uuid.UUID, RenderArtifact) (RenderArtifact, error) {
	return RenderArtifact{}, errors.New("unexpected CreateForLease")
}

func (r *artifactReaderStub) GetByJob(_ context.Context, ownerID, projectID, jobID uuid.UUID) (RenderArtifact, error) {
	r.ownerID = ownerID
	r.projectID = projectID
	r.jobID = jobID
	return r.artifact, r.err
}

func renderJobForStatus(t *testing.T, ownerID, projectID, jobID uuid.UUID, state jobs.State, digest string) jobs.Job {
	t.Helper()
	payload, err := json.Marshal(RenderPayload{
		SnapshotDigest: digest,
		SnapshotSchema: sceneeditor.SnapshotSchemaVersion,
		ProfileID:      LocalProfileID,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	return jobs.Job{
		ID:          jobID,
		OwnerID:     ownerID,
		ProjectID:   &projectID,
		Kind:        JobKind,
		State:       state,
		Attempt:     1,
		MaxAttempts: DefaultMaxAttempts,
		Payload:     payload,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestGetScopesRenderStatusToOwnerProjectAndReturnsDurableArtifact(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	jobID := uuid.New()
	assetID := uuid.New()
	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	reader := &jobReaderStub{job: renderJobForStatus(t, ownerID, projectID, jobID, jobs.StateSucceeded, digest)}
	artifacts := &artifactReaderStub{artifact: RenderArtifact{
		ID:             uuid.New(),
		OwnerID:        ownerID,
		ProjectID:      projectID,
		JobID:          jobID,
		SnapshotDigest: digest,
		ProfileID:      LocalProfileID,
		MediaAssetID:   assetID,
	}}
	service := NewServiceWithRuntime(nil, nil, reader, artifacts, uuid.New)

	view, err := service.Get(context.Background(), ownerID, projectID, jobID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if reader.ownerID != ownerID || reader.projectID != projectID || reader.jobID != jobID {
		t.Fatalf("job lookup escaped owner/project scope: %#v", reader)
	}
	if artifacts.ownerID != ownerID || artifacts.projectID != projectID || artifacts.jobID != jobID {
		t.Fatalf("artifact lookup escaped owner/project scope: %#v", artifacts)
	}
	if view.Artifact == nil || view.Artifact.MediaAssetID != assetID {
		t.Fatalf("artifact = %#v, want media asset %s", view.Artifact, assetID)
	}
}

func TestGetRefusesFalseSucceededStateWithoutDurableArtifact(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	jobID := uuid.New()
	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	reader := &jobReaderStub{job: renderJobForStatus(t, ownerID, projectID, jobID, jobs.StateSucceeded, digest)}
	artifacts := &artifactReaderStub{err: ErrArtifactNotFound}
	service := NewServiceWithRuntime(nil, nil, reader, artifacts, uuid.New)

	_, err := service.Get(context.Background(), ownerID, projectID, jobID)
	if !errors.Is(err, ErrRenderArtifactUnavailable) {
		t.Fatalf("Get() error = %v, want ErrRenderArtifactUnavailable", err)
	}
}

func TestGetHidesNonRenderJobAsNotFound(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	jobID := uuid.New()
	job := renderJobForStatus(t, ownerID, projectID, jobID, jobs.StateRunning, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	job.Kind = "other_kind"
	service := NewServiceWithRuntime(nil, nil, &jobReaderStub{job: job}, &artifactReaderStub{}, uuid.New)

	_, err := service.Get(context.Background(), ownerID, projectID, jobID)
	if !errors.Is(err, ErrRenderNotFound) {
		t.Fatalf("Get() error = %v, want ErrRenderNotFound", err)
	}
}
