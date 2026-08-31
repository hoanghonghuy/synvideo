package project

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

const (
	defaultListLimit = 20
	maxListLimit     = 100
)

type Principal struct {
	OwnerID uuid.UUID
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Create(ctx context.Context, principal Principal, input CreateInput) (Project, error) {
	if err := requirePrincipal(principal); err != nil {
		return Project{}, err
	}
	if err := input.NormalizeAndValidate(); err != nil {
		return Project{}, err
	}
	return s.repository.Create(ctx, principal.OwnerID, input)
}

func (s *Service) List(ctx context.Context, principal Principal, limit int, cursorValue string) (ListResult, string, error) {
	if err := requirePrincipal(principal); err != nil {
		return ListResult{}, "", err
	}

	if limit == 0 {
		limit = defaultListLimit
	}
	if limit < 1 || limit > maxListLimit {
		return ListResult{}, "", ValidationError{Fields: map[string]string{"limit": "range_1_100"}}
	}

	options := ListOptions{Limit: limit}
	if cursorValue != "" {
		cursor, err := DecodeCursor(cursorValue)
		if err != nil {
			return ListResult{}, "", ValidationError{Fields: map[string]string{"cursor": "invalid"}}
		}
		options.Cursor = &cursor
	}

	result, err := s.repository.List(ctx, principal.OwnerID, options)
	if err != nil {
		return ListResult{}, "", err
	}

	nextCursor := ""
	if result.NextCursor != nil {
		nextCursor, err = EncodeCursor(*result.NextCursor)
		if err != nil {
			return ListResult{}, "", fmt.Errorf("encode next cursor: %w", err)
		}
	}

	return result, nextCursor, nil
}

func (s *Service) Get(ctx context.Context, principal Principal, id uuid.UUID) (Project, error) {
	if err := requirePrincipal(principal); err != nil {
		return Project{}, err
	}
	return s.repository.Get(ctx, principal.OwnerID, id)
}

func (s *Service) Update(ctx context.Context, principal Principal, id uuid.UUID, input UpdateInput) (Project, error) {
	if err := requirePrincipal(principal); err != nil {
		return Project{}, err
	}
	if err := input.NormalizeAndValidate(); err != nil {
		return Project{}, err
	}
	return s.repository.Update(ctx, principal.OwnerID, id, input)
}

func requirePrincipal(principal Principal) error {
	if principal.OwnerID == uuid.Nil {
		return ErrUnauthenticated
	}
	return nil
}

var ErrUnauthenticated = fmt.Errorf("request principal is required")
