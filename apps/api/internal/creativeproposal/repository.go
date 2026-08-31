package creativeproposal

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrNotFound          = errors.New("creative proposal not found")
	ErrStaleRevision     = errors.New("creative proposal stale revision")
	ErrProposalImmutable = errors.New("creative proposal is immutable")
	ErrUnauthenticated   = errors.New("request principal is required")
	ErrVersionInvalid    = errors.New("version must be a positive integer")
	ErrRevisionInvalid   = errors.New("revision must be a positive integer")
)

type Repository interface {
	List(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]CreativeProposal, error)
	Get(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int) (CreativeProposal, error)
	CreateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, input CreateDraftInput) (CreativeProposal, error)
	UpdateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, input PutInput) (CreativeProposal, error)
	Approve(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, revision int) (CreativeProposal, error)
}
