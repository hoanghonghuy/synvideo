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

	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type fakeCreativeProposalService struct {
	listFn        func(ctx context.Context, principal project.Principal, projectID uuid.UUID) ([]creativeproposal.CreativeProposal, error)
	getFn         func(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int) (creativeproposal.CreativeProposal, error)
	createDraftFn func(ctx context.Context, principal project.Principal, projectID uuid.UUID, input creativeproposal.CreateDraftInput) (creativeproposal.CreativeProposal, error)
	updateDraftFn func(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, input creativeproposal.PutInput) (creativeproposal.CreativeProposal, error)
	approveFn     func(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, revision int) (creativeproposal.CreativeProposal, error)
}

func (f fakeCreativeProposalService) List(ctx context.Context, principal project.Principal, projectID uuid.UUID) ([]creativeproposal.CreativeProposal, error) {
	if f.listFn != nil {
		return f.listFn(ctx, principal, projectID)
	}
	return nil, nil
}

func (f fakeCreativeProposalService) Get(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int) (creativeproposal.CreativeProposal, error) {
	if f.getFn != nil {
		return f.getFn(ctx, principal, projectID, version)
	}
	return creativeproposal.CreativeProposal{}, nil
}

func (f fakeCreativeProposalService) CreateDraft(ctx context.Context, principal project.Principal, projectID uuid.UUID, input creativeproposal.CreateDraftInput) (creativeproposal.CreativeProposal, error) {
	if f.createDraftFn != nil {
		return f.createDraftFn(ctx, principal, projectID, input)
	}
	return creativeproposal.CreativeProposal{}, nil
}

func (f fakeCreativeProposalService) UpdateDraft(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, input creativeproposal.PutInput) (creativeproposal.CreativeProposal, error) {
	if f.updateDraftFn != nil {
		return f.updateDraftFn(ctx, principal, projectID, version, input)
	}
	return creativeproposal.CreativeProposal{}, nil
}

func (f fakeCreativeProposalService) Approve(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, revision int) (creativeproposal.CreativeProposal, error) {
	if f.approveFn != nil {
		return f.approveFn(ctx, principal, projectID, version, revision)
	}
	return creativeproposal.CreativeProposal{}, nil
}

func sampleProposal(projectID uuid.UUID, version int, revision int, status creativeproposal.Status) creativeproposal.CreativeProposal {
	duration := 120
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	var approvedAt *time.Time
	if status == creativeproposal.StatusApproved {
		approvedAt = &now
	}
	return creativeproposal.CreativeProposal{
		ProjectID:                projectID,
		Version:                  version,
		Revision:                 revision,
		Status:                   status,
		SourceBriefRevision:      1,
		TitleOptions:             []string{"Title 1", "Title 2"},
		HookOptions:              []string{"Hook 1"},
		AudienceSummary:          "Tech founders",
		ObjectiveSummary:         "Explain product",
		NarrativeAngle:           "Problem solving",
		EstimatedDurationSeconds: &duration,
		FormatRationale:          "Direct message",
		Structure: []creativeproposal.StructureItem{
			{Key: "intro", Title: "Introduction", Purpose: "Hook viewer"},
		},
		VisualDirection:  "Clean and modern",
		VoiceDirection:   "Warm tone",
		MusicDirection:   "Upbeat",
		CaptionDirection: "Dynamic",
		CallToAction:     "Try free",
		ResearchGaps:     []string{"Gap 1"},
		Warnings:         []string{"Warning 1"},
		CreatedAt:        now,
		UpdatedAt:        now,
		ApprovedAt:       approvedAt,
	}
}

func TestListCreativeProposalsEndpoint(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	service := fakeCreativeProposalService{
		listFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID) ([]creativeproposal.CreativeProposal, error) {
			if principal.OwnerID != ownerID || pID != projectID {
				t.Fatalf("unexpected call params: principal=%v, projectID=%v", principal, pID)
			}
			return []creativeproposal.CreativeProposal{
				sampleProposal(projectID, 2, 1, creativeproposal.StatusDraft),
				sampleProposal(projectID, 1, 3, creativeproposal.StatusSuperseded),
			}, nil
		},
	}

	server := New(config.Config{Environment: "test"}, nil, nil, nil, service, nil, nil, nil, fixedResolver{ownerID: ownerID})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/creative-proposals", nil)
	rec := httptest.NewRecorder()

	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response []creativeProposalSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(response) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(response))
	}
	if response[0].Version != 2 || response[0].Status != "draft" {
		t.Fatalf("unexpected first summary: %#v", response[0])
	}
	if response[1].Version != 1 || response[1].Status != "superseded" {
		t.Fatalf("unexpected second summary: %#v", response[1])
	}
}

func TestGetCreativeProposalVersionEndpoint(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	service := fakeCreativeProposalService{
		getFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, version int) (creativeproposal.CreativeProposal, error) {
			if version == 1 {
				return sampleProposal(projectID, 1, 1, creativeproposal.StatusDraft), nil
			}
			return creativeproposal.CreativeProposal{}, creativeproposal.ErrNotFound
		},
	}

	server := New(config.Config{Environment: "test"}, nil, nil, nil, service, nil, nil, nil, fixedResolver{ownerID: ownerID})

	// Success case
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/creative-proposals/1", nil)
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response creativeProposalResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Version != 1 || response.Status != "draft" || len(response.TitleOptions) != 2 {
		t.Fatalf("unexpected response: %#v", response)
	}

	// 404 case
	req404 := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/creative-proposals/99", nil)
	rec404 := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec404, req404)

	if rec404.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec404.Code)
	}
	var errResp errorEnvelope
	_ = json.Unmarshal(rec404.Body.Bytes(), &errResp)
	if errResp.Error.Code != "proposal_not_found" {
		t.Fatalf("expected code proposal_not_found, got %q", errResp.Error.Code)
	}

	// Invalid version format
	reqInvalid := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/creative-proposals/invalid", nil)
	recInvalid := httptest.NewRecorder()
	server.Handler.ServeHTTP(recInvalid, reqInvalid)

	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recInvalid.Code)
	}
}

func TestCreativeProposalEndpointDoesNotExposeSourceGenerationJobID(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	jobID := uuid.MustParse("33333333-3333-4333-8333-333333333333")

	proposal := sampleProposal(projectID, 1, 1, creativeproposal.StatusDraft)
	proposal.SourceGenerationJobID = &jobID

	service := fakeCreativeProposalService{
		getFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, version int) (creativeproposal.CreativeProposal, error) {
			return proposal, nil
		},
	}

	server := New(config.Config{Environment: "test"}, nil, nil, nil, service, nil, nil, nil, fixedResolver{ownerID: ownerID})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/creative-proposals/1", nil)
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var rawMap map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rawMap); err != nil {
		t.Fatalf("unmarshal json map: %v", err)
	}
	if _, exists := rawMap["source_generation_job_id"]; exists {
		t.Fatalf("source_generation_job_id must not be exposed in public API response: %s", rec.Body.String())
	}
}

func TestPutCreativeProposalEndpointUpdatesDraft(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	service := fakeCreativeProposalService{
		updateDraftFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, version int, input creativeproposal.PutInput) (creativeproposal.CreativeProposal, error) {
			if version != 1 || *input.Revision != 2 {
				t.Fatalf("unexpected update params: version=%d, revision=%v", version, input.Revision)
			}
			proposal := sampleProposal(projectID, 1, 3, creativeproposal.StatusDraft)
			proposal.AudienceSummary = input.AudienceSummary
			return proposal, nil
		},
	}

	server := New(config.Config{Environment: "test"}, nil, nil, nil, service, nil, nil, nil, fixedResolver{ownerID: ownerID})

	body := map[string]any{
		"revision":                   2,
		"title_options":              []string{"Updated Title 1", "Updated Title 2"},
		"hook_options":               []string{"Updated Hook 1"},
		"audience_summary":           "Updated Audience",
		"objective_summary":          "Updated Objective",
		"narrative_angle":            "Updated Angle",
		"estimated_duration_seconds": 60,
		"format_rationale":           "Updated Rationale",
		"structure": []map[string]string{
			{"key": "hook", "title": "Hook", "purpose": "Grab attention"},
		},
		"visual_direction":  "Updated visuals",
		"voice_direction":   "Updated voice",
		"music_direction":   "Updated music",
		"caption_direction": "Updated captions",
		"call_to_action":    "Updated CTA",
		"research_gaps":     []string{"Updated gap"},
		"warnings":          []string{"Updated warning"},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectID.String()+"/creative-proposals/1", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response creativeProposalResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Revision != 3 || response.AudienceSummary != "Updated Audience" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestPutCreativeProposalEndpointMapsStaleAndImmutable(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	service := fakeCreativeProposalService{
		updateDraftFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, version int, input creativeproposal.PutInput) (creativeproposal.CreativeProposal, error) {
			if version == 1 {
				return creativeproposal.CreativeProposal{}, creativeproposal.ErrStaleRevision
			}
			if version == 2 {
				return creativeproposal.CreativeProposal{}, creativeproposal.ErrProposalImmutable
			}
			return creativeproposal.CreativeProposal{}, creativeproposal.ErrNotFound
		},
	}

	server := New(config.Config{Environment: "test"}, nil, nil, nil, service, nil, nil, nil, fixedResolver{ownerID: ownerID})

	body := map[string]any{
		"revision":          1,
		"title_options":     []string{"Title 1"},
		"hook_options":      []string{"Hook 1"},
		"audience_summary":  "Audience",
		"objective_summary": "Objective",
		"narrative_angle":   "Angle",
		"structure": []map[string]string{
			{"key": "k", "title": "T", "purpose": "P"},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	// Stale revision -> 409 STALE_REVISION
	req1 := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectID.String()+"/creative-proposals/1", bytes.NewReader(bodyBytes))
	rec1 := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", rec1.Code, rec1.Body.String())
	}
	var errResp1 errorEnvelope
	_ = json.Unmarshal(rec1.Body.Bytes(), &errResp1)
	if errResp1.Error.Code != "STALE_REVISION" {
		t.Fatalf("expected STALE_REVISION, got %q", errResp1.Error.Code)
	}

	// Immutable -> 409 PROPOSAL_IMMUTABLE
	req2 := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectID.String()+"/creative-proposals/2", bytes.NewReader(bodyBytes))
	rec2 := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", rec2.Code, rec2.Body.String())
	}
	var errResp2 errorEnvelope
	_ = json.Unmarshal(rec2.Body.Bytes(), &errResp2)
	if errResp2.Error.Code != "PROPOSAL_IMMUTABLE" {
		t.Fatalf("expected PROPOSAL_IMMUTABLE, got %q", errResp2.Error.Code)
	}
}

func TestPutCreativeProposalEndpointRejectsServerControlledFields(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	server := New(config.Config{Environment: "test"}, nil, nil, nil, fakeCreativeProposalService{}, nil, nil, nil, fixedResolver{ownerID: ownerID})

	for _, field := range []string{"project_id", "version", "status", "source_brief_revision", "created_at", "updated_at", "approved_at"} {
		body := map[string]any{
			"revision":          1,
			"title_options":     []string{"Title 1"},
			"hook_options":      []string{"Hook 1"},
			"audience_summary":  "Audience",
			"objective_summary": "Objective",
			"narrative_angle":   "Angle",
			"structure": []map[string]string{
				{"key": "k", "title": "T", "purpose": "P"},
			},
			field: "disallowed",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectID.String()+"/creative-proposals/1", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 for server-controlled field %q, got %d: %s", field, rec.Code, rec.Body.String())
		}
	}
}

func TestApproveCreativeProposalEndpoint(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	service := fakeCreativeProposalService{
		approveFn: func(ctx context.Context, principal project.Principal, pID uuid.UUID, version int, revision int) (creativeproposal.CreativeProposal, error) {
			if version == 1 && revision == 3 {
				return sampleProposal(projectID, 1, 3, creativeproposal.StatusApproved), nil
			}
			if version == 1 && revision != 3 {
				return creativeproposal.CreativeProposal{}, creativeproposal.ErrStaleRevision
			}
			if version == 2 {
				return creativeproposal.CreativeProposal{}, creativeproposal.ErrProposalImmutable
			}
			return creativeproposal.CreativeProposal{}, creativeproposal.ErrNotFound
		},
	}

	server := New(config.Config{Environment: "test"}, nil, nil, nil, service, nil, nil, nil, fixedResolver{ownerID: ownerID})

	// Success
	body := map[string]any{"revision": 3}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/creative-proposals/1/approve", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response creativeProposalResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Status != "approved" || response.ApprovedAt == nil {
		t.Fatalf("expected approved proposal with approved_at, got %#v", response)
	}

	// Stale revision
	bodyStale := map[string]any{"revision": 2}
	bodyStaleBytes, _ := json.Marshal(bodyStale)
	reqStale := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/creative-proposals/1/approve", bytes.NewReader(bodyStaleBytes))
	recStale := httptest.NewRecorder()
	server.Handler.ServeHTTP(recStale, reqStale)

	if recStale.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", recStale.Code)
	}
	var errResp errorEnvelope
	_ = json.Unmarshal(recStale.Body.Bytes(), &errResp)
	if errResp.Error.Code != "STALE_REVISION" {
		t.Fatalf("expected STALE_REVISION, got %q", errResp.Error.Code)
	}

	// Non-draft / immutable
	reqImmutable := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/creative-proposals/2/approve", bytes.NewReader(bodyBytes))
	recImmutable := httptest.NewRecorder()
	server.Handler.ServeHTTP(recImmutable, reqImmutable)

	if recImmutable.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", recImmutable.Code)
	}
	var errRespImm errorEnvelope
	_ = json.Unmarshal(recImmutable.Body.Bytes(), &errRespImm)
	if errRespImm.Error.Code != "PROPOSAL_IMMUTABLE" {
		t.Fatalf("expected PROPOSAL_IMMUTABLE, got %q", errRespImm.Error.Code)
	}
}
