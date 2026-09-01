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
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
)

type fakeScriptService struct {
	listFn         func(ctx context.Context, principal project.Principal, projectID uuid.UUID) ([]script.Script, error)
	getByVersionFn func(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int) (script.Script, error)
	updateDraftFn  func(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, input script.PutInput) (script.Script, error)
	approveFn      func(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, revision int) (script.Script, error)
}

func (f *fakeScriptService) List(ctx context.Context, principal project.Principal, projectID uuid.UUID) ([]script.Script, error) {
	if f.listFn != nil {
		return f.listFn(ctx, principal, projectID)
	}
	return nil, nil
}

func (f *fakeScriptService) GetByVersion(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int) (script.Script, error) {
	if f.getByVersionFn != nil {
		return f.getByVersionFn(ctx, principal, projectID, version)
	}
	return script.Script{}, nil
}

func (f *fakeScriptService) UpdateDraft(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, input script.PutInput) (script.Script, error) {
	if f.updateDraftFn != nil {
		return f.updateDraftFn(ctx, principal, projectID, version, input)
	}
	return script.Script{}, nil
}

func (f *fakeScriptService) Approve(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, revision int) (script.Script, error) {
	if f.approveFn != nil {
		return f.approveFn(ctx, principal, projectID, version, revision)
	}
	return script.Script{}, nil
}

type fakeScriptActorResolver struct {
	principal project.Principal
	err       error
}

func (r fakeScriptActorResolver) Resolve(_ *http.Request) (project.Principal, error) {
	return r.principal, r.err
}

func newTestScriptServer(svc ScriptService, resolver fakeScriptActorResolver) *http.Server {
	return New(
		config.Config{Addr: ":0", Environment: "test"},
		nil,
		nil,
		nil,
		nil,
		svc,
		nil,
		nil,
		resolver,
	)
}

func TestScriptEndpoints(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	principal := project.Principal{OwnerID: ownerID}
	resolver := fakeScriptActorResolver{principal: principal}
	now := time.Now().UTC()

	sampleScript := script.Script{
		ProjectID:             projectID,
		Version:               1,
		Revision:              1,
		Status:                script.StatusDraft,
		SourceProposalVersion: 1,
		ContentLocale:         "vi",
		Sections: []script.Section{
			{Key: "intro", Heading: "Intro", Body: "Intro text"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	t.Run("GET list returns summaries", func(t *testing.T) {
		svc := &fakeScriptService{
			listFn: func(ctx context.Context, p project.Principal, pID uuid.UUID) ([]script.Script, error) {
				return []script.Script{sampleScript}, nil
			},
		}
		server := newTestScriptServer(svc, resolver)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/scripts", nil)
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp []scriptSummaryResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal list response: %v", err)
		}
		if len(resp) != 1 || resp[0].Version != 1 || resp[0].Status != script.StatusDraft {
			t.Fatalf("unexpected summary item: %#v", resp)
		}
	})

	t.Run("GET by version returns full details", func(t *testing.T) {
		svc := &fakeScriptService{
			getByVersionFn: func(ctx context.Context, p project.Principal, pID uuid.UUID, v int) (script.Script, error) {
				return sampleScript, nil
			},
		}
		server := newTestScriptServer(svc, resolver)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/scripts/1", nil)
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		var resp scriptResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if resp.Version != 1 || resp.Sections[0].Key != "intro" {
			t.Fatalf("unexpected detail: %#v", resp)
		}
	})

	t.Run("GET by missing version returns 404", func(t *testing.T) {
		svc := &fakeScriptService{
			getByVersionFn: func(ctx context.Context, p project.Principal, pID uuid.UUID, v int) (script.Script, error) {
				return script.Script{}, script.ErrNotFound
			},
		}
		server := newTestScriptServer(svc, resolver)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/scripts/999", nil)
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("PUT draft returns 200 and incremented revision", func(t *testing.T) {
		svc := &fakeScriptService{
			updateDraftFn: func(ctx context.Context, p project.Principal, pID uuid.UUID, v int, input script.PutInput) (script.Script, error) {
				updated := sampleScript
				updated.Revision = 2
				updated.Sections = input.Sections
				return updated, nil
			},
		}
		server := newTestScriptServer(svc, resolver)
		body := `{"revision":1,"sections":[{"key":"intro","heading":"New Intro","body":"New Body"}]}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectID.String()+"/scripts/1", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp scriptResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Revision != 2 {
			t.Fatalf("expected revision 2, got %d", resp.Revision)
		}
	})

	t.Run("PUT draft returns 409 STALE_REVISION", func(t *testing.T) {
		svc := &fakeScriptService{
			updateDraftFn: func(ctx context.Context, p project.Principal, pID uuid.UUID, v int, input script.PutInput) (script.Script, error) {
				return script.Script{}, script.ErrStaleRevision
			},
		}
		server := newTestScriptServer(svc, resolver)
		body := `{"revision":1,"sections":[{"key":"intro","body":"Body"}]}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectID.String()+"/scripts/1", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", rec.Code)
		}
		var errEnv errorEnvelope
		_ = json.Unmarshal(rec.Body.Bytes(), &errEnv)
		if errEnv.Error.Code != "STALE_REVISION" {
			t.Fatalf("expected STALE_REVISION, got %s", errEnv.Error.Code)
		}
	})

	t.Run("PUT draft returns 409 SCRIPT_IMMUTABLE", func(t *testing.T) {
		svc := &fakeScriptService{
			updateDraftFn: func(ctx context.Context, p project.Principal, pID uuid.UUID, v int, input script.PutInput) (script.Script, error) {
				return script.Script{}, script.ErrScriptImmutable
			},
		}
		server := newTestScriptServer(svc, resolver)
		body := `{"revision":1,"sections":[{"key":"intro","body":"Body"}]}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectID.String()+"/scripts/1", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", rec.Code)
		}
		var errEnv errorEnvelope
		_ = json.Unmarshal(rec.Body.Bytes(), &errEnv)
		if errEnv.Error.Code != "SCRIPT_IMMUTABLE" {
			t.Fatalf("expected SCRIPT_IMMUTABLE, got %s", errEnv.Error.Code)
		}
	})

	t.Run("POST approve returns 200 and approved script", func(t *testing.T) {
		apprTime := time.Now().UTC()
		svc := &fakeScriptService{
			approveFn: func(ctx context.Context, p project.Principal, pID uuid.UUID, v int, rev int) (script.Script, error) {
				approved := sampleScript
				approved.Status = script.StatusApproved
				approved.ApprovedAt = &apprTime
				return approved, nil
			},
		}
		server := newTestScriptServer(svc, resolver)
		body := `{"revision":1}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/scripts/1/approve", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp scriptResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Status != script.StatusApproved || resp.ApprovedAt == nil {
			t.Fatalf("expected approved status and timestamp: %#v", resp)
		}
	})

	t.Run("POST approve returns 409 STALE_REVISION", func(t *testing.T) {
		svc := &fakeScriptService{
			approveFn: func(ctx context.Context, p project.Principal, pID uuid.UUID, v int, rev int) (script.Script, error) {
				return script.Script{}, script.ErrStaleRevision
			},
		}
		server := newTestScriptServer(svc, resolver)
		body := `{"revision":1}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/scripts/1/approve", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", rec.Code)
		}
		var errEnv errorEnvelope
		_ = json.Unmarshal(rec.Body.Bytes(), &errEnv)
		if errEnv.Error.Code != "STALE_REVISION" {
			t.Fatalf("expected STALE_REVISION, got %s", errEnv.Error.Code)
		}
	})

	t.Run("Unauthenticated returns 401", func(t *testing.T) {
		unauthResolver := fakeScriptActorResolver{err: errors.New("unauth")}
		server := newTestScriptServer(&fakeScriptService{}, unauthResolver)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/scripts", nil)
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}
