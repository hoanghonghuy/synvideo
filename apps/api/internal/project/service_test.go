package project

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestServiceCreateUsesPrincipalOwner(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	repository := &recordingRepository{}
	service := NewService(repository)

	created, err := service.Create(context.Background(), Principal{OwnerID: ownerID}, CreateInput{
		Title:         "Owned project",
		ContentFormat: ContentFormatShort,
		AspectRatio:   AspectRatio9x16,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if repository.createdOwnerID != ownerID {
		t.Fatalf("expected repository owner %s, got %s", ownerID, repository.createdOwnerID)
	}
	if created.OwnerID != ownerID {
		t.Fatalf("expected created owner %s, got %s", ownerID, created.OwnerID)
	}
}

func TestServiceRejectsMissingPrincipal(t *testing.T) {
	service := NewService(&recordingRepository{})

	_, err := service.Create(context.Background(), Principal{}, CreateInput{
		Title:         "No principal",
		ContentFormat: ContentFormatShort,
		AspectRatio:   AspectRatio9x16,
	})
	if err == nil {
		t.Fatal("expected missing principal to fail")
	}
}

func TestServiceRejectsInvalidCursor(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	service := NewService(&recordingRepository{})

	_, _, err := service.List(context.Background(), Principal{OwnerID: ownerID}, 20, "not-a-cursor")
	if err == nil {
		t.Fatal("expected invalid cursor to fail")
	}
}

type recordingRepository struct {
	createdOwnerID uuid.UUID
}

func (r *recordingRepository) Create(_ context.Context, ownerID uuid.UUID, input CreateInput) (Project, error) {
	r.createdOwnerID = ownerID
	now := time.Now().UTC()
	return Project{
		ID:            uuid.New(),
		OwnerID:       ownerID,
		Title:         input.Title,
		ContentFormat: input.ContentFormat,
		AspectRatio:   input.AspectRatio,
		Locale:        input.Locale,
		Status:        StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (r *recordingRepository) List(_ context.Context, _ uuid.UUID, _ ListOptions) (ListResult, error) {
	return ListResult{}, nil
}

func (r *recordingRepository) Get(_ context.Context, _ uuid.UUID, _ uuid.UUID) (Project, error) {
	return Project{}, ErrNotFound
}

func (r *recordingRepository) Update(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ UpdateInput) (Project, error) {
	return Project{}, ErrNotFound
}
