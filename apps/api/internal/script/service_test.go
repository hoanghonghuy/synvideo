package script_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
)

type fakeScriptRepository struct {
	createDraftFn  func(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, input script.CreateDraftInput) (script.Script, error)
	updateDraftFn  func(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, input script.PutInput) (script.Script, error)
	approveFn      func(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, revision int) (script.Script, error)
	getByVersionFn func(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int) (script.Script, error)
	listVersionsFn func(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]script.Script, error)
}

func (f *fakeScriptRepository) CreateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, input script.CreateDraftInput) (script.Script, error) {
	if f.createDraftFn != nil {
		return f.createDraftFn(ctx, ownerID, projectID, input)
	}
	return script.Script{}, errors.New("not implemented")
}

func (f *fakeScriptRepository) UpdateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, input script.PutInput) (script.Script, error) {
	if f.updateDraftFn != nil {
		return f.updateDraftFn(ctx, ownerID, projectID, version, input)
	}
	return script.Script{}, errors.New("not implemented")
}

func (f *fakeScriptRepository) Approve(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, revision int) (script.Script, error) {
	if f.approveFn != nil {
		return f.approveFn(ctx, ownerID, projectID, version, revision)
	}
	return script.Script{}, errors.New("not implemented")
}

func (f *fakeScriptRepository) GetByVersion(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int) (script.Script, error) {
	if f.getByVersionFn != nil {
		return f.getByVersionFn(ctx, ownerID, projectID, version)
	}
	return script.Script{}, errors.New("not implemented")
}

func (f *fakeScriptRepository) ListVersions(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]script.Script, error) {
	if f.listVersionsFn != nil {
		return f.listVersionsFn(ctx, ownerID, projectID)
	}
	return nil, errors.New("not implemented")
}

func TestServiceUnauthenticated(t *testing.T) {
	svc := script.NewService(&fakeScriptRepository{})
	unauth := project.Principal{}
	projectID := uuid.New()

	if _, err := svc.List(context.Background(), unauth, projectID); !errors.Is(err, script.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated on List, got %v", err)
	}
	if _, err := svc.GetByVersion(context.Background(), unauth, projectID, 1); !errors.Is(err, script.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated on GetByVersion, got %v", err)
	}
	rev := 1
	if _, err := svc.UpdateDraft(context.Background(), unauth, projectID, 1, script.PutInput{Revision: &rev}); !errors.Is(err, script.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated on UpdateDraft, got %v", err)
	}
	if _, err := svc.Approve(context.Background(), unauth, projectID, 1, 1); !errors.Is(err, script.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated on Approve, got %v", err)
	}
	if _, err := svc.CreateDraft(context.Background(), unauth, projectID, script.CreateDraftInput{SourceProposalVersion: 1}); !errors.Is(err, script.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated on CreateDraft, got %v", err)
	}
}

func TestServiceSuccessDelegation(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	principal := project.Principal{OwnerID: ownerID}
	now := time.Now().UTC()

	fake := &fakeScriptRepository{
		createDraftFn: func(ctx context.Context, oID, pID uuid.UUID, input script.CreateDraftInput) (script.Script, error) {
			return script.Script{
				ProjectID:             pID,
				Version:               1,
				Revision:              1,
				Status:                script.StatusDraft,
				SourceProposalVersion: input.SourceProposalVersion,
				ContentLocale:         "vi",
				Sections:              input.Sections,
				CreatedAt:             now,
				UpdatedAt:             now,
			}, nil
		},
	}
	svc := script.NewService(fake)

	created, err := svc.CreateDraft(context.Background(), principal, projectID, script.CreateDraftInput{
		SourceProposalVersion: 1,
		Content:               validContent(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating draft: %v", err)
	}
	if created.Version != 1 || created.Status != script.StatusDraft {
		t.Fatalf("unexpected created script: %#v", created)
	}
}
