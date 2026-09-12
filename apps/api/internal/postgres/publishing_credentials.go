package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/publishing"
)

func (r *PublishingRepository) GetRefreshTokenEnvelope(ctx context.Context, ownerID, connectionID uuid.UUID) (publishing.RefreshTokenEnvelope, error) {
	if ownerID == uuid.Nil || connectionID == uuid.Nil {
		return publishing.RefreshTokenEnvelope{}, publishing.ErrInvalidModel
	}

	var envelope publishing.RefreshTokenEnvelope
	err := r.pool.QueryRow(ctx, `
		SELECT encrypted_refresh_token, token_nonce, token_key_id
		FROM publishing_channel_connections
		WHERE owner_id=$1 AND id=$2
	`, ownerID, connectionID).Scan(&envelope.Ciphertext, &envelope.Nonce, &envelope.KeyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return publishing.RefreshTokenEnvelope{}, publishing.ErrConnectionNotFound
		}
		return publishing.RefreshTokenEnvelope{}, fmt.Errorf("get publishing refresh token envelope: %w", err)
	}
	if len(envelope.Ciphertext) == 0 || len(envelope.Nonce) == 0 || envelope.KeyID == "" {
		return publishing.RefreshTokenEnvelope{}, publishing.ErrInvalidModel
	}
	return envelope, nil
}
