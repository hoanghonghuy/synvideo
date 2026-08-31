package creativebrief

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

func (s *Service) Get(ctx context.Context, principal project.Principal, projectID uuid.UUID) (CreativeBrief, error) {
	if err := requirePrincipal(principal); err != nil {
		return CreativeBrief{}, err
	}
	return s.repository.Get(ctx, principal.OwnerID, projectID)
}

func (s *Service) Put(ctx context.Context, principal project.Principal, projectID uuid.UUID, input PutInput) (CreativeBrief, bool, error) {
	if err := requirePrincipal(principal); err != nil {
		return CreativeBrief{}, false, err
	}
	if err := input.NormalizeAndValidate(); err != nil {
		return CreativeBrief{}, false, err
	}
	return s.repository.Put(ctx, principal.OwnerID, projectID, input)
}

func requirePrincipal(principal project.Principal) error {
	if principal.OwnerID == uuid.Nil {
		return ErrUnauthenticated
	}
	return nil
}
