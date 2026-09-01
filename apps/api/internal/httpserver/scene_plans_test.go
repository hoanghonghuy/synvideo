package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

type fakeScenePlanService struct {
	listFn         func(ctx context.Context, principal project.Principal, projectID uuid.UUID) ([]sceneplan.Plan, error)
	getByVersionFn func(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int) (sceneplan.Plan, error)
	updateDraftFn  func(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, input sceneplan.PutInput) (sceneplan.Plan, error)
	approveFn      func(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, revision int) (sceneplan.Plan, error)
}

func (f *fakeScenePlanService) List(ctx context.Context, principal project.Principal, projectID uuid.UUID) ([]sceneplan.Plan, error) {
	if f.listFn != nil {
		return f.listFn(ctx, principal, projectID)
	}
	return nil, nil
}

func (f *fakeScenePlanService) GetByVersion(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int) (sceneplan.Plan, error) {
	if f.getByVersionFn != nil {
		return f.getByVersionFn(ctx, principal, projectID, version)
	}
	return sceneplan.Plan{}, nil
}

func (f *fakeScenePlanService) UpdateDraft(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, input sceneplan.PutInput) (sceneplan.Plan, error) {
	if f.updateDraftFn != nil {
		return f.updateDraftFn(ctx, principal, projectID, version, input)
	}
	return sceneplan.Plan{}, nil
}

func (f *fakeScenePlanService) Approve(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, revision int) (sceneplan.Plan, error) {
	if f.approveFn != nil {
		return f.approveFn(ctx, principal, projectID, version, revision)
	}
	return sceneplan.Plan{}, nil
}

type fakeScenePlanActorResolver struct {
	principal project.Principal
	err       error
}

func (r fakeScenePlanActorResolver) Resolve(_ *http.Request) (project.Principal, error) {
	return r.principal, r.err
}

func newTestScenePlanServer(svc ScenePlanService, resolver fakeScenePlanActorResolver) *http.Server {
	return New(
		config.Config{Addr: ":0", Environment: "test"},
		nil,
		nil,
		nil,
		nil,
		nil,
		svc,
		nil,
		nil,
		nil,
		nil,
		resolver,
	)
}

func TestScenePlanEndpoints(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	principal := project.Principal{OwnerID: ownerID}
	resolver := fakeScenePlanActorResolver{principal: principal}
	now := time.Now().UTC()

	samplePlan := sceneplan.Plan{
		ProjectID:             projectID,
		Version:               1,
		Revision:              1,
		Status:                sceneplan.StatusDraft,
		SourceScriptVersion:   1,
		SourceProposalVersion: 1,
		ContentLocale:         "vi",
		Scenes: []sceneplan.Scene{
			{
				Key:                     "intro",
				ScriptSectionKey:        "intro",
				Narration:               "Welcome to our tour.",
				VisualInstruction:       "Hero banner visual.",
				PlannedSourceType:       sceneplan.SourceTypeStock,
				ExpectedDurationSeconds: 10,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	t.Run("GET list returns summaries", func(t *testing.T) {
		svc := &fakeScenePlanService{
			listFn: func(ctx context.Context, p project.Principal, prjID uuid.UUID) ([]sceneplan.Plan, error) {
				return []sceneplan.Plan{samplePlan}, nil
			},
		}
		server := newTestScenePlanServer(svc, resolver)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/scene-plans", nil)
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var summaries []scenePlanSummaryResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &summaries); err != nil {
			t.Fatalf("unmarshal summaries: %v", err)
		}
		if len(summaries) != 1 || summaries[0].Version != 1 || summaries[0].Status != sceneplan.StatusDraft {
			t.Fatalf("unexpected summaries: %+v", summaries)
		}
	})

	t.Run("GET by version returns full scene plan", func(t *testing.T) {
		svc := &fakeScenePlanService{
			getByVersionFn: func(ctx context.Context, p project.Principal, prjID uuid.UUID, v int) (sceneplan.Plan, error) {
				if v == 1 {
					return samplePlan, nil
				}
				return sceneplan.Plan{}, sceneplan.ErrNotFound
			},
		}
		server := newTestScenePlanServer(svc, resolver)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/scene-plans/1", nil)
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp scenePlanResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if resp.Version != 1 || len(resp.Scenes) != 1 || resp.Scenes[0].Key != "intro" {
			t.Fatalf("unexpected scene plan response: %+v", resp)
		}

		// 404 case
		req404 := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/scene-plans/99", nil)
		rec404 := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec404, req404)
		if rec404.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec404.Code)
		}
	})

	t.Run("PUT update draft success and conflicts", func(t *testing.T) {
		svc := &fakeScenePlanService{
			updateDraftFn: func(ctx context.Context, p project.Principal, prjID uuid.UUID, v int, input sceneplan.PutInput) (sceneplan.Plan, error) {
				if input.Revision != nil && *input.Revision == 1 {
					updated := samplePlan
					updated.Revision = 2
					return updated, nil
				}
				return sceneplan.Plan{}, sceneplan.ErrStaleRevision
			},
		}
		server := newTestScenePlanServer(svc, resolver)

		body := `{"revision":1,"scenes":[{"key":"intro","script_section_key":"intro","narration":"Welcome to our tour.","visual_instruction":"Updated visual.","planned_source_type":"stock","expected_duration_seconds":10}]}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectID.String()+"/scene-plans/1", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		// Stale revision -> 409
		bodyStale := `{"revision":99,"scenes":[{"key":"intro","script_section_key":"intro","narration":"Welcome to our tour.","visual_instruction":"Updated visual.","planned_source_type":"stock","expected_duration_seconds":10}]}`
		reqStale := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectID.String()+"/scene-plans/1", bytes.NewBufferString(bodyStale))
		reqStale.Header.Set("Content-Type", "application/json")
		recStale := httptest.NewRecorder()
		server.Handler.ServeHTTP(recStale, reqStale)

		if recStale.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d: %s", recStale.Code, recStale.Body.String())
		}
	})

	t.Run("POST approve draft success and errors", func(t *testing.T) {
		svc := &fakeScenePlanService{
			approveFn: func(ctx context.Context, p project.Principal, prjID uuid.UUID, v int, rev int) (sceneplan.Plan, error) {
				if rev == 1 {
					approved := samplePlan
					approved.Status = sceneplan.StatusApproved
					tNow := time.Now().UTC()
					approved.ApprovedAt = &tNow
					return approved, nil
				}
				return sceneplan.Plan{}, sceneplan.ErrStaleRevision
			},
		}
		server := newTestScenePlanServer(svc, resolver)

		body := `{"revision":1}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/scene-plans/1/approve", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp scenePlanResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal approved: %v", err)
		}
		if resp.Status != sceneplan.StatusApproved || resp.ApprovedAt == nil {
			t.Fatalf("expected approved status and timestamp, got: %+v", resp)
		}

		// Missing revision -> 400
		bodyEmpty := `{}`
		reqEmpty := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/scene-plans/1/approve", bytes.NewBufferString(bodyEmpty))
		reqEmpty.Header.Set("Content-Type", "application/json")
		recEmpty := httptest.NewRecorder()
		server.Handler.ServeHTTP(recEmpty, reqEmpty)
		if recEmpty.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing revision, got %d", recEmpty.Code)
		}
	})

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		unauthResolver := fakeScenePlanActorResolver{err: errors.New("unauth")}
		server := newTestScenePlanServer(&fakeScenePlanService{}, unauthResolver)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/scene-plans", nil)
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}
