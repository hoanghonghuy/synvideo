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
	if _, err := svc.ForkApprovedDraft(context.Background(), unauth, projectID, 1); !errors.Is(err, script.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated on ForkApprovedDraft, got %v", err)
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

func TestForkApprovedDraftCopiesAuthoritativeContent(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	principal := project.Principal{OwnerID: ownerID}
	approved := script.Script{
		ProjectID:             projectID,
		Version:               4,
		Revision:              2,
		Status:                script.StatusApproved,
		SourceProposalVersion: 3,
		ContentLocale:         "vi",
		Sections:              []script.Section{{Key: "intro", Heading: "Intro", Body: "Approved text"}},
		Notes:                 "keep lineage",
	}
	var forkInput script.CreateDraftInput
	fake := &fakeScriptRepository{
		getByVersionFn: func(ctx context.Context, oID, pID uuid.UUID, version int) (script.Script, error) {
			if oID != ownerID || pID != projectID || version != approved.Version {
				t.Fatalf("unexpected source lookup: owner=%s project=%s version=%d", oID, pID, version)
			}
			return approved, nil
		},
		createDraftFn: func(ctx context.Context, oID, pID uuid.UUID, input script.CreateDraftInput) (script.Script, error) {
			forkInput = input
			return script.Script{ProjectID: pID, Version: 5, Revision: 1, Status: script.StatusDraft, Sections: input.Sections}, nil
		},
	}

	forked, err := script.NewService(fake).ForkApprovedDraft(context.Background(), principal, projectID, approved.Version)
	if err != nil {
		t.Fatalf("fork approved draft: %v", err)
	}
	if forked.Version != 5 || forked.Status != script.StatusDraft {
		t.Fatalf("unexpected fork result: %#v", forked)
	}
	if forkInput.SourceProposalVersion != approved.SourceProposalVersion || forkInput.ContentLocale != approved.ContentLocale {
		t.Fatalf("fork lost authoritative lineage: %#v", forkInput)
	}
	if len(forkInput.Sections) != 1 || forkInput.Sections[0].Body != "Approved text" || forkInput.Notes != approved.Notes {
		t.Fatalf("fork did not copy approved content: %#v", forkInput)
	}
}

func TestForkApprovedDraftRejectsMutableSource(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	principal := project.Principal{OwnerID: ownerID}
	fake := &fakeScriptRepository{
		getByVersionFn: func(context.Context, uuid.UUID, uuid.UUID, int) (script.Script, error) {
			return script.Script{ProjectID: projectID, Version: 2, Status: script.StatusDraft}, nil
		},
	}

	_, err := script.NewService(fake).ForkApprovedDraft(context.Background(), principal, projectID, 2)
	if !errors.Is(err, script.ErrForkSourceNotApproved) {
		t.Fatalf("expected ErrForkSourceNotApproved, got %v", err)
	}
}
