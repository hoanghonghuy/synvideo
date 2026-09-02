package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativebrief"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

func TestGetCreativeBriefEndpoint(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	server := newCreativeBriefTestServer(ownerID, &fakeCreativeBriefService{
		getResult: validBrief(projectID, 2, "Existing intent"),
	})

	response := performRequest(server, http.MethodGet, "/api/v1/projects/"+projectID.String()+"/creative-brief", "")

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["project_id"] != projectID.String() || body["source_text"] != "Existing intent" {
		t.Fatalf("unexpected response: %#v", body)
	}
	if _, exists := body["owner_id"]; exists {
		t.Fatal("owner_id must not be exposed in creative brief response")
	}
}

func TestPutCreativeBriefEndpointCreatesFirstRevision(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	service := &fakeCreativeBriefService{
		putResult:  validBrief(projectID, 1, "Creator intent"),
		putCreated: true,
	}
	server := newCreativeBriefTestServer(ownerID, service)

	response := performRequest(server, http.MethodPut, "/api/v1/projects/"+projectID.String()+"/creative-brief", `{
		"source_text": "Creator intent",
		"target_audience": "Creators",
		"objective": "Explain",
		"desired_style": "Documentary",
		"tone": "Confident",
		"distribution_targets": ["youtube"],
		"call_to_action": "Start trial",
		"must_include": ["Demo"],
		"must_avoid": ["Unsupported claims"]
	}`)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}
	if service.lastInput.Revision != nil {
		t.Fatalf("create request must not set revision, got %v", *service.lastInput.Revision)
	}
	if service.lastProjectID != projectID {
		t.Fatalf("expected project id %s, got %s", projectID, service.lastProjectID)
	}
}

func TestPutCreativeBriefEndpointUpdatesExistingRevision(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	service := &fakeCreativeBriefService{
		putResult: validBrief(projectID, 2, "Updated intent"),
	}
	server := newCreativeBriefTestServer(ownerID, service)

	response := performRequest(server, http.MethodPut, "/api/v1/projects/"+projectID.String()+"/creative-brief", `{
		"revision": 1,
		"source_text": "Updated intent"
	}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
	if service.lastInput.Revision == nil || *service.lastInput.Revision != 1 {
		t.Fatalf("expected revision 1 input, got %#v", service.lastInput.Revision)
	}
}

func TestPutCreativeBriefEndpointValidation(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	server := newCreativeBriefTestServer(ownerID, &fakeCreativeBriefService{
		putErr: creativebrief.ValidationError{Fields: map[string]string{"source_text": "required"}},
	})

	response := performRequest(server, http.MethodPut, "/api/v1/projects/"+projectID.String()+"/creative-brief", `{
		"source_text": ""
	}`)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("validation_failed")) {
		t.Fatalf("expected validation error envelope, got %s", response.Body.String())
	}
}

func TestCreativeBriefEndpointMapsNotFound(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	server := newCreativeBriefTestServer(ownerID, &fakeCreativeBriefService{
		getErr: creativebrief.ErrNotFound,
	})

	response := performRequest(server, http.MethodGet, "/api/v1/projects/"+projectID.String()+"/creative-brief", "")

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("creative_brief_not_found")) {
		t.Fatalf("expected creative brief not found code, got %s", response.Body.String())
	}
}

func TestPutCreativeBriefEndpointMapsStaleRevision(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	server := newCreativeBriefTestServer(ownerID, &fakeCreativeBriefService{
		putErr: creativebrief.ErrStaleRevision,
	})

	response := performRequest(server, http.MethodPut, "/api/v1/projects/"+projectID.String()+"/creative-brief", `{
		"revision": 1,
		"source_text": "Stale intent"
	}`)

	if response.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("STALE_REVISION")) {
		t.Fatalf("expected STALE_REVISION code, got %s", response.Body.String())
	}
}

func TestPutCreativeBriefEndpointMapsRevisionRequired(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	server := newCreativeBriefTestServer(ownerID, &fakeCreativeBriefService{
		putErr: creativebrief.ErrRevisionRequired,
	})

	response := performRequest(server, http.MethodPut, "/api/v1/projects/"+projectID.String()+"/creative-brief", `{
		"source_text": "Missing revision"
	}`)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"revision":"required"`)) {
		t.Fatalf("expected revision required field error, got %s", response.Body.String())
	}
}

func TestPutCreativeBriefEndpointMapsRevisionUnexpected(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	server := newCreativeBriefTestServer(ownerID, &fakeCreativeBriefService{
		putErr: creativebrief.ErrRevisionUnexpected,
	})

	response := performRequest(server, http.MethodPut, "/api/v1/projects/"+projectID.String()+"/creative-brief", `{
		"revision": 1,
		"source_text": "Revision on create"
	}`)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"revision":"not_allowed"`)) {
		t.Fatalf("expected revision not_allowed field error, got %s", response.Body.String())
	}
}

func TestPutCreativeBriefEndpointRejectsServerControlledFields(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	server := newCreativeBriefTestServer(ownerID, &fakeCreativeBriefService{})

	response := performRequest(server, http.MethodPut, "/api/v1/projects/"+projectID.String()+"/creative-brief", `{
		"project_id": "33333333-3333-4333-8333-333333333333",
		"source_text": "Creator intent"
	}`)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("validation_failed")) {
		t.Fatalf("expected validation error envelope, got %s", response.Body.String())
	}
}

func newCreativeBriefTestServer(ownerID uuid.UUID, service CreativeBriefService) *http.Server {
	cfg := config.Config{Addr: ":0", Environment: config.EnvironmentTest}
	return New(cfg, slog.Default(), nil, service, nil, nil, nil, nil, nil, nil, nil, fixedResolver{ownerID: ownerID})
}

type fakeCreativeBriefService struct {
	getResult     creativebrief.CreativeBrief
	getErr        error
	putResult     creativebrief.CreativeBrief
	putCreated    bool
	putErr        error
	lastProjectID uuid.UUID
	lastInput     creativebrief.PutInput
}

func (s *fakeCreativeBriefService) Get(_ context.Context, _ project.Principal, projectID uuid.UUID) (creativebrief.CreativeBrief, error) {
	s.lastProjectID = projectID
	return s.getResult, s.getErr
}

func (s *fakeCreativeBriefService) Put(_ context.Context, _ project.Principal, projectID uuid.UUID, input creativebrief.PutInput) (creativebrief.CreativeBrief, bool, error) {
	s.lastProjectID = projectID
	s.lastInput = input
	return s.putResult, s.putCreated, s.putErr
}

func validBrief(projectID uuid.UUID, revision int, sourceText string) creativebrief.CreativeBrief {
	now := time.Date(2026, 8, 31, 10, 30, 0, 0, time.UTC)
	return creativebrief.CreativeBrief{
		ProjectID:           projectID,
		Revision:            revision,
		SourceText:          sourceText,
		TargetAudience:      "Creators",
		Objective:           "Explain",
		DesiredStyle:        "Documentary",
		Tone:                "Confident",
		DistributionTargets: []creativebrief.DistributionTarget{creativebrief.DistributionTargetYouTube},
		CallToAction:        "Start trial",
		MustInclude:         []string{"Demo"},
		MustAvoid:           []string{"Unsupported claims"},
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}
