package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/identity"
)

type IdentityMappingRepository struct {
	pool *pgxpool.Pool
}

func NewIdentityMappingRepository(pool *pgxpool.Pool) *IdentityMappingRepository {
	return &IdentityMappingRepository{pool: pool}
}

func (r *IdentityMappingRepository) ResolveOrCreateOwner(ctx context.Context, external identity.ExternalIdentity) (uuid.UUID, error) {
	issuer := external.Issuer
	subject := external.Subject
	if issuer == "" || subject == "" {
		return uuid.Nil, errors.New("issuer and subject are required")
	}

	ownerID := uuid.New()
	var resolved uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO identity_principal_mappings (issuer, subject, owner_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (issuer, subject) DO NOTHING
		RETURNING owner_id
	`, issuer, subject, ownerID).Scan(&resolved)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("insert identity mapping: %w", err)
	}

	if err := r.pool.QueryRow(ctx, `
		SELECT owner_id FROM identity_principal_mappings
		WHERE issuer = $1 AND subject = $2
	`, issuer, subject).Scan(&resolved); err != nil {
		return uuid.Nil, fmt.Errorf("load identity mapping: %w", err)
	}
	return resolved, nil
}
