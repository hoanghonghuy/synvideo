package creativeproposal

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type mockRepository struct {
	listFn        func(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]CreativeProposal, error)
	getFn         func(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int) (CreativeProposal, error)
	createDraftFn func(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, input CreateDraftInput) (CreativeProposal, error)
	updateDraftFn func(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, input PutInput) (CreativeProposal, error)
	approveFn     func(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, revision int) (CreativeProposal, error)
}

func (m *mockRepository) List(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]CreativeProposal, error) {
	if m.listFn != nil {
		return m.listFn(ctx, ownerID, projectID)
	}
	return nil, nil
}

func (m *mockRepository) Get(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int) (CreativeProposal, error) {
	if m.getFn != nil {
		return m.getFn(ctx, ownerID, projectID, version)
	}
	return CreativeProposal{}, nil
}

func (m *mockRepository) CreateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, input CreateDraftInput) (CreativeProposal, error) {
	if m.createDraftFn != nil {
		return m.createDraftFn(ctx, ownerID, projectID, input)
	}
	return CreativeProposal{}, nil
}

func (m *mockRepository) UpdateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, input PutInput) (CreativeProposal, error) {
	if m.updateDraftFn != nil {
		return m.updateDraftFn(ctx, ownerID, projectID, version, input)
	}
	return CreativeProposal{}, nil
}

func (m *mockRepository) Approve(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, revision int) (CreativeProposal, error) {
	if m.approveFn != nil {
		return m.approveFn(ctx, ownerID, projectID, version, revision)
	}
	return CreativeProposal{}, nil
}

func TestServiceRequiresPrincipal(t *testing.T) {
	svc := NewService(&mockRepository{})
	ctx := context.Background()
	projectID := uuid.New()
	emptyPrincipal := project.Principal{}

	if _, err := svc.List(ctx, emptyPrincipal, projectID); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated on List, got %v", err)
	}
	if _, err := svc.Get(ctx, emptyPrincipal, projectID, 1); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated on Get, got %v", err)
	}
	if _, err := svc.CreateDraft(ctx, emptyPrincipal, projectID, CreateDraftInput{}); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated on CreateDraft, got %v", err)
	}
	if _, err := svc.UpdateDraft(ctx, emptyPrincipal, projectID, 1, PutInput{}); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated on UpdateDraft, got %v", err)
	}
	if _, err := svc.Approve(ctx, emptyPrincipal, projectID, 1, 1); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated on Approve, got %v", err)
	}
}

func TestServiceValidatesVersion(t *testing.T) {
	svc := NewService(&mockRepository{})
	ctx := context.Background()
	principal := project.Principal{OwnerID: uuid.New()}
	projectID := uuid.New()

	if _, err := svc.Get(ctx, principal, projectID, 0); !errors.Is(err, ErrVersionInvalid) {
		t.Fatalf("expected ErrVersionInvalid on Get version 0, got %v", err)
	}
	if _, err := svc.UpdateDraft(ctx, principal, projectID, -1, PutInput{}); !errors.Is(err, ErrVersionInvalid) {
		t.Fatalf("expected ErrVersionInvalid on UpdateDraft version -1, got %v", err)
	}
	if _, err := svc.Approve(ctx, principal, projectID, 0, 1); !errors.Is(err, ErrVersionInvalid) {
		t.Fatalf("expected ErrVersionInvalid on Approve version 0, got %v", err)
	}
}

func TestServiceValidatesApproveRevision(t *testing.T) {
	svc := NewService(&mockRepository{})
	ctx := context.Background()
	principal := project.Principal{OwnerID: uuid.New()}
	projectID := uuid.New()

	if _, err := svc.Approve(ctx, principal, projectID, 1, 0); err == nil {
		t.Fatalf("expected error on Approve revision 0, got nil")
	}
}

func TestServiceDelegatesToRepository(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	principal := project.Principal{OwnerID: ownerID}
	rev := 2
	expected := CreativeProposal{
		ProjectID: projectID,
		Version:   1,
		Revision:  rev,
		Status:    StatusDraft,
	}

	repo := &mockRepository{
		listFn: func(ctx context.Context, o uuid.UUID, p uuid.UUID) ([]CreativeProposal, error) {
			if o != ownerID || p != projectID {
				t.Fatalf("unexpected list params: o=%v, p=%v", o, p)
			}
			return []CreativeProposal{expected}, nil
		},
		getFn: func(ctx context.Context, o uuid.UUID, p uuid.UUID, v int) (CreativeProposal, error) {
			if o != ownerID || p != projectID || v != 1 {
				t.Fatalf("unexpected get params: o=%v, p=%v, v=%d", o, p, v)
			}
			return expected, nil
		},
		createDraftFn: func(ctx context.Context, o uuid.UUID, p uuid.UUID, input CreateDraftInput) (CreativeProposal, error) {
			if o != ownerID || p != projectID {
				t.Fatalf("unexpected create params: o=%v, p=%v", o, p)
			}
			return expected, nil
		},
		updateDraftFn: func(ctx context.Context, o uuid.UUID, p uuid.UUID, v int, input PutInput) (CreativeProposal, error) {
			if o != ownerID || p != projectID || v != 1 {
				t.Fatalf("unexpected update params: o=%v, p=%v, v=%d", o, p, v)
			}
			return expected, nil
		},
		approveFn: func(ctx context.Context, o uuid.UUID, p uuid.UUID, v int, r int) (CreativeProposal, error) {
			if o != ownerID || p != projectID || v != 1 || r != 2 {
				t.Fatalf("unexpected approve params: o=%v, p=%v, v=%d, r=%d", o, p, v, r)
			}
			approved := expected
			approved.Status = StatusApproved
			return approved, nil
		},
	}

	svc := NewService(repo)
	ctx := context.Background()

	listRes, err := svc.List(ctx, principal, projectID)
	if err != nil || len(listRes) != 1 {
		t.Fatalf("unexpected list: %v, err=%v", listRes, err)
	}

	getRes, err := svc.Get(ctx, principal, projectID, 1)
	if err != nil || getRes.Version != 1 {
		t.Fatalf("unexpected get: %v, err=%v", getRes, err)
	}

	createInput := CreateDraftInput{
		SourceBriefRevision: 1,
		Content:             validTestContent(),
	}
	createRes, err := svc.CreateDraft(ctx, principal, projectID, createInput)
	if err != nil || createRes.Version != 1 {
		t.Fatalf("unexpected create: %v, err=%v", createRes, err)
	}

	putInput := PutInput{
		Revision: &rev,
		Content:  validTestContent(),
	}
	updateRes, err := svc.UpdateDraft(ctx, principal, projectID, 1, putInput)
	if err != nil || updateRes.Version != 1 {
		t.Fatalf("unexpected update: %v, err=%v", updateRes, err)
	}

	approveRes, err := svc.Approve(ctx, principal, projectID, 1, 2)
	if err != nil || approveRes.Status != StatusApproved {
		t.Fatalf("unexpected approve: %v, err=%v", approveRes, err)
	}
}
