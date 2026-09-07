package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/renderexport"
)

type renderResolverStub struct{ principal project.Principal }

func (r renderResolverStub) Resolve(*http.Request) (project.Principal, error) {
	return r.principal, nil
}

type renderExportServiceStub struct {
	job        jobs.Job
	view       renderexport.JobView
	enqueueErr error
	getErr     error
	ownerID    uuid.UUID
	projectID  uuid.UUID
	digest     string
	jobID      uuid.UUID
}

func (s *renderExportServiceStub) Enqueue(_ context.Context, ownerID, projectID uuid.UUID, digest string) (jobs.Job, error) {
	s.ownerID, s.projectID, s.digest = ownerID, projectID, digest
	return s.job, s.enqueueErr
}
func (s *renderExportServiceStub) Get(_ context.Context, ownerID, projectID, jobID uuid.UUID) (renderexport.JobView, error) {
	s.ownerID, s.projectID, s.jobID = ownerID, projectID, jobID
	return s.view, s.getErr
}

func TestRenderExportCreateReturnsDurableStatusView(t *testing.T) {
	ownerID, projectID, jobID, assetID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	now := time.Now().UTC()
	service := &renderExportServiceStub{
		job: jobs.Job{ID: jobID},
		view: renderexport.JobView{
			ID: jobID, State: jobs.StateSucceeded, Attempt: 1, MaxAttempts: 2,
			SnapshotDigest: digest, ProfileID: renderexport.LocalProfileID, CreatedAt: now, UpdatedAt: now,
			Artifact: &renderexport.RenderArtifact{ID: uuid.New(), MediaAssetID: assetID, ByteSize: 10, SHA256: digest, MimeType: "video/mp4", DurationMS: 1000, Width: 1280, Height: 720, ToolchainVersion: "ffmpeg test", CreatedAt: now},
		},
	}
	handler := renderExportHandler{service: service, actorResolver: renderResolverStub{principal: project.Principal{OwnerID: ownerID}}}
	body, err := json.Marshal(createRenderExportRequest{SnapshotDigest: digest})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/render-exports", bytes.NewReader(body))
	req.SetPathValue("id", projectID.String())
	w := httptest.NewRecorder()

	handler.create(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if service.ownerID != ownerID || service.projectID != projectID || service.digest != digest || service.jobID != jobID {
		t.Fatalf("service scope mismatch: %#v", service)
	}
	var response renderExportResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Artifact == nil || response.Artifact.MediaAssetID != assetID.String() {
		t.Fatalf("artifact response = %#v", response.Artifact)
	}
}

func TestRenderExportGetRejectsInvalidJobIDBeforeService(t *testing.T) {
	ownerID, projectID := uuid.New(), uuid.New()
	service := &renderExportServiceStub{}
	handler := renderExportHandler{service: service, actorResolver: renderResolverStub{principal: project.Principal{OwnerID: ownerID}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/render-exports/not-a-uuid", nil)
	req.SetPathValue("id", projectID.String())
	req.SetPathValue("job_id", "not-a-uuid")
	w := httptest.NewRecorder()

	handler.get(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if service.jobID != uuid.Nil {
		t.Fatalf("service called with invalid job id: %s", service.jobID)
	}
}
