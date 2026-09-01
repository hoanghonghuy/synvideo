package sceneplan

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

var ErrVersionInvalid = errors.New("invalid scene plan version")

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateDraft(ctx context.Context, principal project.Principal, projectID uuid.UUID, input CreateDraftInput) (Plan, error) {
	if err := requirePrincipal(principal); err != nil {
		return Plan{}, err
	}
	if projectID == uuid.Nil {
		return Plan{}, errors.Join(ErrInvalidInput, errors.New("project_id is required"))
	}
	if err := input.NormalizeAndValidate(); err != nil {
		return Plan{}, err
	}
	return s.repo.CreateDraft(ctx, principal.OwnerID, projectID, input)
}

func (s *Service) UpdateDraft(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, input PutInput) (Plan, error) {
	if err := requirePrincipal(principal); err != nil {
		return Plan{}, err
	}
	if projectID == uuid.Nil {
		return Plan{}, errors.Join(ErrInvalidInput, errors.New("project_id is required"))
	}
	if version < 1 {
		return Plan{}, ErrVersionInvalid
	}
	if err := input.NormalizeAndValidate(); err != nil {
		return Plan{}, err
	}
	return s.repo.UpdateDraft(ctx, principal.OwnerID, projectID, version, input)
}

func (s *Service) Approve(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, revision int) (Plan, error) {
	if err := requirePrincipal(principal); err != nil {
		return Plan{}, err
	}
	if projectID == uuid.Nil {
		return Plan{}, errors.Join(ErrInvalidInput, errors.New("project_id is required"))
	}
	if version < 1 {
		return Plan{}, ErrVersionInvalid
	}
	if revision < 1 {
		return Plan{}, ValidationError{Fields: map[string]string{"revision": "positive"}}
	}
	return s.repo.Approve(ctx, principal.OwnerID, projectID, version, revision)
}

func (s *Service) GetByVersion(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int) (Plan, error) {
	if err := requirePrincipal(principal); err != nil {
		return Plan{}, err
	}
	if projectID == uuid.Nil {
		return Plan{}, errors.Join(ErrInvalidInput, errors.New("project_id is required"))
	}
	if version < 1 {
		return Plan{}, ErrVersionInvalid
	}
	return s.repo.GetByVersion(ctx, principal.OwnerID, projectID, version)
}

func (s *Service) List(ctx context.Context, principal project.Principal, projectID uuid.UUID) ([]Plan, error) {
	if err := requirePrincipal(principal); err != nil {
		return nil, err
	}
	if projectID == uuid.Nil {
		return nil, errors.Join(ErrInvalidInput, errors.New("project_id is required"))
	}
	return s.repo.ListVersions(ctx, principal.OwnerID, projectID)
}

func requirePrincipal(principal project.Principal) error {
	if principal.OwnerID == uuid.Nil {
		return ErrUnauthenticated
	}
	return nil
}
