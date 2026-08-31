package creativebrief

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

func TestServicePutRequiresPrincipal(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository)

	_, _, err := service.Put(context.Background(), project.Principal{}, uuid.New(), validPutInput())

	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected unauthenticated error, got %v", err)
	}
	if repository.putCalled {
		t.Fatal("repository must not be called without a principal")
	}
}

func TestServicePutValidatesInputBeforeRepository(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository)
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")

	_, _, err := service.Put(context.Background(), project.Principal{OwnerID: ownerID}, uuid.New(), PutInput{})

	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationErr.Fields["source_text"] != "required" {
		t.Fatalf("expected source_text required, got %#v", validationErr.Fields)
	}
	if repository.putCalled {
		t.Fatal("repository must not be called with invalid input")
	}
}

func TestServiceGetDelegatesOwnerScopedRepository(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	expected := CreativeBrief{ProjectID: projectID, Revision: 1, SourceText: "Intent"}
	repository := &fakeRepository{getResult: expected}
	service := NewService(repository)

	got, err := service.Get(context.Background(), project.Principal{OwnerID: ownerID}, projectID)

	if err != nil {
		t.Fatalf("get brief: %v", err)
	}
	if got.ProjectID != expected.ProjectID || got.Revision != expected.Revision {
		t.Fatalf("unexpected brief: %#v", got)
	}
	if repository.lastOwnerID != ownerID {
		t.Fatalf("expected owner %s, got %s", ownerID, repository.lastOwnerID)
	}
}

type fakeRepository struct {
	getResult   CreativeBrief
	putResult   CreativeBrief
	putCreated  bool
	lastOwnerID uuid.UUID
	putCalled   bool
}

func (r *fakeRepository) Get(_ context.Context, ownerID uuid.UUID, _ uuid.UUID) (CreativeBrief, error) {
	r.lastOwnerID = ownerID
	return r.getResult, nil
}

func (r *fakeRepository) Put(_ context.Context, ownerID uuid.UUID, _ uuid.UUID, _ PutInput) (CreativeBrief, bool, error) {
	r.lastOwnerID = ownerID
	r.putCalled = true
	return r.putResult, r.putCreated, nil
}
