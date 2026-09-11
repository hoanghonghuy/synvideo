package httpserver

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/publishing"
)

type publishingTestActorResolver struct {
	principal project.Principal
	err       error
}

func (r publishingTestActorResolver) Resolve(*http.Request) (project.Principal, error) {
	return r.principal, r.err
}

type fakePublishingHTTPService struct {
	connections []publishing.ChannelConnection
	listErr     error
	attempt     publishing.PublishAttempt
	getErr      error
	createErr   error

	listOwnerID   uuid.UUID
	getOwnerID    uuid.UUID
	getProjectID  uuid.UUID
	getAttemptID  uuid.UUID
	createOwnerID uuid.UUID
	createProject uuid.UUID
	connectionID  uuid.UUID
	artifactID    uuid.UUID
	requestID     uuid.UUID
	title         string
	description   string
}

func (s *fakePublishingHTTPService) ListConnections(_ context.Context, ownerID uuid.UUID) ([]publishing.ChannelConnection, error) {
	s.listOwnerID = ownerID
	return s.connections, s.listErr
}

func (s *fakePublishingHTTPService) GetAttempt(_ context.Context, ownerID, projectID, attemptID uuid.UUID) (publishing.PublishAttempt, error) {
	s.getOwnerID = ownerID
	s.getProjectID = projectID
	s.getAttemptID = attemptID
	return s.attempt, s.getErr
}

func (s *fakePublishingHTTPService) CreateAttempt(_ context.Context, ownerID, projectID, connectionID, renderArtifactID, requestID uuid.UUID, title, description string) (publishing.PublishAttempt, error) {
	s.createOwnerID = ownerID
	s.createProject = projectID
	s.connectionID = connectionID
	s.artifactID = renderArtifactID
	s.requestID = requestID
	s.title = title
	s.description = description
	return s.attempt, s.createErr
}

func newPublishingTestMux(ownerID uuid.UUID, service PublishingService) *http.ServeMux {
	mux := http.NewServeMux()
	RegisterPublishingRoutes(mux, service, publishingTestActorResolver{principal: project.Principal{OwnerID: ownerID}})
	return mux
}

func publishingTestAttempt(ownerID, projectID, connectionID, artifactID, requestID uuid.UUID) publishing.PublishAttempt {
	now := time.Date(2026, 9, 11, 15, 0, 0, 0, time.UTC)
	return publishing.PublishAttempt{
		ID:               uuid.MustParse("66666666-6666-4666-8666-666666666666"),
		OwnerID:          ownerID,
		ProjectID:        projectID,
		ConnectionID:     connectionID,
		RenderArtifactID: artifactID,
		RequestID:        requestID,
		Provider:         publishing.ProviderYouTube,
		State:            publishing.PublishQueued,
		Title:            "Launch video",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func performPublishingRequest(handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestPublishingRoutesRequirePrincipal(t *testing.T) {
	mux := http.NewServeMux()
	RegisterPublishingRoutes(mux, &fakePublishingHTTPService{}, publishingTestActorResolver{err: errors.New("missing principal")})

	response := performPublishingRequest(mux, http.MethodGet, "/api/v1/publishing/connections", "")

	if response.Code != http.StatusUnauthorized || !bytes.Contains(response.Body.Bytes(), []byte("principal_required")) {
		t.Fatalf("expected principal_required 401, got %d: %s", response.Code, response.Body.String())
	}
}

func TestPublishingRoutesValidateRequestBoundary(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	connectionID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	artifactID := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	requestID := uuid.MustParse("55555555-5555-4555-8555-555555555555")
	mux := newPublishingTestMux(ownerID, &fakePublishingHTTPService{})

	tests := []struct {
		name   string
		target string
		body   string
		field  string
	}{
		{name: "project uuid", target: "/api/v1/projects/not-a-uuid/publishing/attempts", body: `{}`, field: "project_id"},
		{name: "invalid json", target: "/api/v1/projects/" + projectID.String() + "/publishing/attempts", body: `{`, field: "body"},
		{name: "unknown field", target: "/api/v1/projects/" + projectID.String() + "/publishing/attempts", body: `{"connection_id":"` + connectionID.String() + `","render_artifact_id":"` + artifactID.String() + `","request_id":"` + requestID.String() + `","title":"Launch","owner_id":"` + ownerID.String() + `"}`, field: "body"},
		{name: "connection uuid", target: "/api/v1/projects/" + projectID.String() + "/publishing/attempts", body: `{"connection_id":"bad","render_artifact_id":"` + artifactID.String() + `","request_id":"` + requestID.String() + `","title":"Launch"}`, field: "connection_id"},
		{name: "artifact uuid", target: "/api/v1/projects/" + projectID.String() + "/publishing/attempts", body: `{"connection_id":"` + connectionID.String() + `","render_artifact_id":"bad","request_id":"` + requestID.String() + `","title":"Launch"}`, field: "render_artifact_id"},
		{name: "request uuid", target: "/api/v1/projects/" + projectID.String() + "/publishing/attempts", body: `{"connection_id":"` + connectionID.String() + `","render_artifact_id":"` + artifactID.String() + `","request_id":"bad","title":"Launch"}`, field: "request_id"},
		{name: "title", target: "/api/v1/projects/" + projectID.String() + "/publishing/attempts", body: `{"connection_id":"` + connectionID.String() + `","render_artifact_id":"` + artifactID.String() + `","request_id":"` + requestID.String() + `","title":"   "}`, field: "title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performPublishingRequest(mux, http.MethodPost, tt.target, tt.body)
			if response.Code != http.StatusBadRequest || !bytes.Contains(response.Body.Bytes(), []byte("validation_failed")) || !bytes.Contains(response.Body.Bytes(), []byte(tt.field)) {
				t.Fatalf("expected validation error for %s, got %d: %s", tt.field, response.Code, response.Body.String())
			}
		})
	}
}

func TestPublishingRoutesListCreateAndGetUsePrincipalAndProjectScope(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	connectionID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	artifactID := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	requestID := uuid.MustParse("55555555-5555-4555-8555-555555555555")
	attempt := publishingTestAttempt(ownerID, projectID, connectionID, artifactID, requestID)
	service := &fakePublishingHTTPService{
		connections: []publishing.ChannelConnection{{
			ID:              connectionID,
			OwnerID:         ownerID,
			Provider:        publishing.ProviderYouTube,
			RemoteChannelID: "channel-1",
			DisplayName:     "SynVideo",
			State:           publishing.ConnectionConnected,
			Capabilities:    publishing.Capabilities{CanUpload: true},
			CreatedAt:       attempt.CreatedAt,
			UpdatedAt:       attempt.UpdatedAt,
		}},
		attempt: attempt,
	}
	mux := newPublishingTestMux(ownerID, service)

	listResponse := performPublishingRequest(mux, http.MethodGet, "/api/v1/publishing/connections", "")
	if listResponse.Code != http.StatusOK || service.listOwnerID != ownerID || !bytes.Contains(listResponse.Body.Bytes(), []byte(connectionID.String())) {
		t.Fatalf("unexpected list response %d: %s", listResponse.Code, listResponse.Body.String())
	}
	if bytes.Contains(listResponse.Body.Bytes(), []byte(ownerID.String())) {
		t.Fatalf("owner id must not be exposed: %s", listResponse.Body.String())
	}

	body := `{"connection_id":"` + connectionID.String() + `","render_artifact_id":"` + artifactID.String() + `","request_id":"` + requestID.String() + `","title":"  Launch video  ","description":"Description"}`
	createResponse := performPublishingRequest(mux, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/publishing/attempts", body)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("unexpected create response %d: %s", createResponse.Code, createResponse.Body.String())
	}
	if service.createOwnerID != ownerID || service.createProject != projectID || service.connectionID != connectionID || service.artifactID != artifactID || service.requestID != requestID {
		t.Fatalf("create scope/ids were not forwarded correctly: %+v", service)
	}
	if service.title != "  Launch video  " || service.description != "Description" {
		t.Fatalf("metadata changed unexpectedly at HTTP boundary: title=%q description=%q", service.title, service.description)
	}
	if bytes.Contains(createResponse.Body.Bytes(), []byte(ownerID.String())) || bytes.Contains(createResponse.Body.Bytes(), []byte("resumable")) {
		t.Fatalf("secret/server-controlled publishing fields leaked: %s", createResponse.Body.String())
	}

	getResponse := performPublishingRequest(mux, http.MethodGet, "/api/v1/projects/"+projectID.String()+"/publishing/attempts/"+attempt.ID.String(), "")
	if getResponse.Code != http.StatusOK || service.getOwnerID != ownerID || service.getProjectID != projectID || service.getAttemptID != attempt.ID {
		t.Fatalf("unexpected get response %d: %s", getResponse.Code, getResponse.Body.String())
	}
}

func TestPublishingRoutesMapNotFoundAndConflictWithoutLeakingDetails(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	connectionID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	artifactID := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	requestID := uuid.MustParse("55555555-5555-4555-8555-555555555555")
	attemptID := uuid.MustParse("66666666-6666-4666-8666-666666666666")

	t.Run("attempt not found", func(t *testing.T) {
		mux := newPublishingTestMux(ownerID, &fakePublishingHTTPService{getErr: publishing.ErrAttemptNotFound})
		response := performPublishingRequest(mux, http.MethodGet, "/api/v1/projects/"+projectID.String()+"/publishing/attempts/"+attemptID.String(), "")
		if response.Code != http.StatusNotFound || !bytes.Contains(response.Body.Bytes(), []byte("publishing_attempt_not_found")) {
			t.Fatalf("expected attempt 404, got %d: %s", response.Code, response.Body.String())
		}
	})

	t.Run("connection not found", func(t *testing.T) {
		mux := newPublishingTestMux(ownerID, &fakePublishingHTTPService{createErr: publishing.ErrConnectionNotFound})
		body := `{"connection_id":"` + connectionID.String() + `","render_artifact_id":"` + artifactID.String() + `","request_id":"` + requestID.String() + `","title":"Launch"}`
		response := performPublishingRequest(mux, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/publishing/attempts", body)
		if response.Code != http.StatusNotFound || !bytes.Contains(response.Body.Bytes(), []byte("publishing_connection_not_found")) {
			t.Fatalf("expected connection 404, got %d: %s", response.Code, response.Body.String())
		}
	})

	t.Run("idempotency conflict", func(t *testing.T) {
		mux := newPublishingTestMux(ownerID, &fakePublishingHTTPService{createErr: publishing.ErrAttemptConflict})
		body := `{"connection_id":"` + connectionID.String() + `","render_artifact_id":"` + artifactID.String() + `","request_id":"` + requestID.String() + `","title":"Launch"}`
		response := performPublishingRequest(mux, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/publishing/attempts", body)
		if response.Code != http.StatusConflict || !bytes.Contains(response.Body.Bytes(), []byte("publishing_attempt_conflict")) {
			t.Fatalf("expected conflict, got %d: %s", response.Code, response.Body.String())
		}
	})
}
