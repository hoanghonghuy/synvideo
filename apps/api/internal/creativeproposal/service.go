package creativeproposal

import (
	"context"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) List(ctx context.Context, principal project.Principal, projectID uuid.UUID) ([]CreativeProposal, error) {
	if err := requirePrincipal(principal); err != nil {
		return nil, err
	}
	return s.repository.List(ctx, principal.OwnerID, projectID)
}

func (s *Service) Get(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int) (CreativeProposal, error) {
	if err := requirePrincipal(principal); err != nil {
		return CreativeProposal{}, err
	}
	if version < 1 {
		return CreativeProposal{}, ErrVersionInvalid
	}
	return s.repository.Get(ctx, principal.OwnerID, projectID, version)
}

func (s *Service) CreateDraft(ctx context.Context, principal project.Principal, projectID uuid.UUID, input CreateDraftInput) (CreativeProposal, error) {
	if err := requirePrincipal(principal); err != nil {
		return CreativeProposal{}, err
	}
	if err := input.NormalizeAndValidate(); err != nil {
		return CreativeProposal{}, err
	}
	return s.repository.CreateDraft(ctx, principal.OwnerID, projectID, input)
}

func (s *Service) UpdateDraft(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, input PutInput) (CreativeProposal, error) {
	if err := requirePrincipal(principal); err != nil {
		return CreativeProposal{}, err
	}
	if version < 1 {
		return CreativeProposal{}, ErrVersionInvalid
	}
	if err := input.NormalizeAndValidate(); err != nil {
		return CreativeProposal{}, err
	}
	return s.repository.UpdateDraft(ctx, principal.OwnerID, projectID, version, input)
}

func (s *Service) Approve(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, revision int) (CreativeProposal, error) {
	if err := requirePrincipal(principal); err != nil {
		return CreativeProposal{}, err
	}
	if version < 1 {
		return CreativeProposal{}, ErrVersionInvalid
	}
	if revision < 1 {
		return CreativeProposal{}, ValidationError{Fields: map[string]string{"revision": "positive"}}
	}
	return s.repository.Approve(ctx, principal.OwnerID, projectID, version, revision)
}

func requirePrincipal(principal project.Principal) error {
	if principal.OwnerID == uuid.Nil {
		return ErrUnauthenticated
	}
	return nil
}
