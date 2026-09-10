package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/publishing"
)

type PublishingRepository struct{ pool *pgxpool.Pool }

func NewPublishingRepository(pool *pgxpool.Pool) *PublishingRepository {
	return &PublishingRepository{pool: pool}
}

var _ publishing.ConnectionRepository = (*PublishingRepository)(nil)
var _ publishing.AttemptRepository = (*PublishingRepository)(nil)

const publishingConnectionFields = `
	id, owner_id, provider, remote_channel_id, display_name, state,
	can_upload, can_publish, can_schedule, created_at, updated_at
`

func (r *PublishingRepository) UpsertConnection(ctx context.Context, connection publishing.ChannelConnection, encryptedRefreshToken, tokenNonce []byte, tokenKeyID string) (publishing.ChannelConnection, error) {
	if err := connection.Validate(); err != nil {
		return publishing.ChannelConnection{}, err
	}
	if len(encryptedRefreshToken) == 0 || len(tokenNonce) == 0 || strings.TrimSpace(tokenKeyID) == "" {
		return publishing.ChannelConnection{}, publishing.ErrInvalidModel
	}
	query := fmt.Sprintf(`
		INSERT INTO publishing_channel_connections (
			id, owner_id, provider, remote_channel_id, display_name, state,
			can_upload, can_publish, can_schedule, encrypted_refresh_token,
			token_nonce, token_key_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (owner_id, provider, remote_channel_id) DO UPDATE SET
			display_name=EXCLUDED.display_name,
			state=EXCLUDED.state,
			can_upload=EXCLUDED.can_upload,
			can_publish=EXCLUDED.can_publish,
			can_schedule=EXCLUDED.can_schedule,
			encrypted_refresh_token=EXCLUDED.encrypted_refresh_token,
			token_nonce=EXCLUDED.token_nonce,
			token_key_id=EXCLUDED.token_key_id,
			updated_at=EXCLUDED.updated_at
		RETURNING %s
	`, publishingConnectionFields)
	return scanPublishingConnection(r.pool.QueryRow(ctx, query,
		connection.ID, connection.OwnerID, connection.Provider, connection.RemoteChannelID,
		connection.DisplayName, connection.State, connection.Capabilities.CanUpload,
		connection.Capabilities.CanPublish, connection.Capabilities.CanSchedule,
		encryptedRefreshToken, tokenNonce, strings.TrimSpace(tokenKeyID), connection.CreatedAt, connection.UpdatedAt,
	))
}

func (r *PublishingRepository) GetConnection(ctx context.Context, ownerID, connectionID uuid.UUID) (publishing.ChannelConnection, error) {
	if ownerID == uuid.Nil || connectionID == uuid.Nil {
		return publishing.ChannelConnection{}, publishing.ErrInvalidModel
	}
	query := fmt.Sprintf(`SELECT %s FROM publishing_channel_connections WHERE owner_id=$1 AND id=$2`, publishingConnectionFields)
	return scanPublishingConnection(r.pool.QueryRow(ctx, query, ownerID, connectionID))
}

func (r *PublishingRepository) GetConnectionByRemoteChannel(ctx context.Context, ownerID uuid.UUID, provider publishing.Provider, remoteChannelID string) (publishing.ChannelConnection, error) {
	remoteChannelID = strings.TrimSpace(remoteChannelID)
	if ownerID == uuid.Nil || provider == "" || remoteChannelID == "" {
		return publishing.ChannelConnection{}, publishing.ErrInvalidModel
	}
	query := fmt.Sprintf(`SELECT %s FROM publishing_channel_connections WHERE owner_id=$1 AND provider=$2 AND remote_channel_id=$3`, publishingConnectionFields)
	return scanPublishingConnection(r.pool.QueryRow(ctx, query, ownerID, provider, remoteChannelID))
}

func (r *PublishingRepository) ListConnections(ctx context.Context, ownerID uuid.UUID) ([]publishing.ChannelConnection, error) {
	if ownerID == uuid.Nil {
		return nil, publishing.ErrInvalidModel
	}
	query := fmt.Sprintf(`SELECT %s FROM publishing_channel_connections WHERE owner_id=$1 ORDER BY updated_at DESC, id DESC`, publishingConnectionFields)
	rows, err := r.pool.Query(ctx, query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list publishing connections: %w", err)
	}
	defer rows.Close()
	connections := make([]publishing.ChannelConnection, 0)
	for rows.Next() {
		connection, err := scanPublishingConnection(rows)
		if err != nil {
			return nil, err
		}
		connections = append(connections, connection)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list publishing connections: %w", err)
	}
	return connections, nil
}

const publishingAttemptFields = `
	id, owner_id, project_id, connection_id, render_artifact_id, request_id,
	provider, state, remote_video_id, title, description, scheduled_at,
	created_at, updated_at
`

func (r *PublishingRepository) CreateAttempt(ctx context.Context, attempt publishing.PublishAttempt) (publishing.PublishAttempt, error) {
	if err := attempt.Validate(); err != nil {
		return publishing.PublishAttempt{}, err
	}
	query := fmt.Sprintf(`
		INSERT INTO publishing_attempts (
			id, owner_id, project_id, connection_id, render_artifact_id, request_id,
			provider, state, remote_video_id, title, description, scheduled_at,
			created_at, updated_at
		)
		SELECT $1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10,$11,$12,$13,$14
		FROM publishing_channel_connections c
		JOIN render_artifacts a ON a.id=$5
		WHERE c.id=$4 AND c.owner_id=$2 AND c.provider=$7
		  AND c.state='connected' AND c.can_upload=true
		  AND a.owner_id=$2 AND a.project_id=$3 AND a.mime_type='video/mp4'
		ON CONFLICT (owner_id, project_id, connection_id, render_artifact_id, request_id)
		DO UPDATE SET updated_at=publishing_attempts.updated_at
		WHERE publishing_attempts.provider=EXCLUDED.provider
		  AND publishing_attempts.title=EXCLUDED.title
		  AND publishing_attempts.description=EXCLUDED.description
		  AND publishing_attempts.scheduled_at IS NOT DISTINCT FROM EXCLUDED.scheduled_at
		RETURNING %s
	`, publishingAttemptFields)
	created, err := scanPublishingAttempt(r.pool.QueryRow(ctx, query,
		attempt.ID, attempt.OwnerID, attempt.ProjectID, attempt.ConnectionID,
		attempt.RenderArtifactID, attempt.RequestID, attempt.Provider, attempt.State,
		attempt.RemoteVideoID, attempt.Title, attempt.Description, attempt.ScheduledAt,
		attempt.CreatedAt, attempt.UpdatedAt,
	))
	if err == nil {
		return created, nil
	}
	if errors.Is(err, publishing.ErrAttemptNotFound) {
		_, existingErr := r.GetAttemptByRequest(ctx, attempt.OwnerID, attempt.ProjectID, attempt.ConnectionID, attempt.RenderArtifactID, attempt.RequestID)
		if existingErr == nil {
			return publishing.PublishAttempt{}, publishing.ErrAttemptConflict
		}
		if !errors.Is(existingErr, publishing.ErrAttemptNotFound) {
			return publishing.PublishAttempt{}, existingErr
		}
		return publishing.PublishAttempt{}, publishing.ErrAttemptNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return publishing.PublishAttempt{}, publishing.ErrAttemptConflict
	}
	return publishing.PublishAttempt{}, err
}

func (r *PublishingRepository) GetAttempt(ctx context.Context, ownerID, projectID, attemptID uuid.UUID) (publishing.PublishAttempt, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil || attemptID == uuid.Nil {
		return publishing.PublishAttempt{}, publishing.ErrInvalidModel
	}
	query := fmt.Sprintf(`SELECT %s FROM publishing_attempts WHERE id=$1 AND owner_id=$2 AND project_id=$3`, publishingAttemptFields)
	return scanPublishingAttempt(r.pool.QueryRow(ctx, query, attemptID, ownerID, projectID))
}

func (r *PublishingRepository) GetAttemptByRequest(ctx context.Context, ownerID, projectID, connectionID, renderArtifactID, requestID uuid.UUID) (publishing.PublishAttempt, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil || connectionID == uuid.Nil || renderArtifactID == uuid.Nil || requestID == uuid.Nil {
		return publishing.PublishAttempt{}, publishing.ErrInvalidModel
	}
	query := fmt.Sprintf(`SELECT %s FROM publishing_attempts WHERE owner_id=$1 AND project_id=$2 AND connection_id=$3 AND render_artifact_id=$4 AND request_id=$5`, publishingAttemptFields)
	return scanPublishingAttempt(r.pool.QueryRow(ctx, query, ownerID, projectID, connectionID, renderArtifactID, requestID))
}

func (r *PublishingRepository) SaveAttemptProgress(ctx context.Context, attempt publishing.PublishAttempt, resumableSessionURI string, uploadedBytes int64, lastErrorCode string) (publishing.PublishAttempt, error) {
	if err := attempt.Validate(); err != nil || uploadedBytes < 0 {
		if err != nil {
			return publishing.PublishAttempt{}, err
		}
		return publishing.PublishAttempt{}, publishing.ErrInvalidModel
	}
	query := fmt.Sprintf(`
		UPDATE publishing_attempts SET
			state=$4, remote_video_id=NULLIF($5,''), scheduled_at=$6,
			resumable_session_uri=NULLIF($7,''), uploaded_bytes=$8,
			last_error_code=NULLIF($9,''), updated_at=$10
		WHERE id=$1 AND owner_id=$2 AND project_id=$3
		RETURNING %s
	`, publishingAttemptFields)
	return scanPublishingAttempt(r.pool.QueryRow(ctx, query,
		attempt.ID, attempt.OwnerID, attempt.ProjectID, attempt.State,
		attempt.RemoteVideoID, attempt.ScheduledAt, strings.TrimSpace(resumableSessionURI),
		uploadedBytes, strings.TrimSpace(lastErrorCode), attempt.UpdatedAt,
	))
}

type publishingRow interface{ Scan(...any) error }

func scanPublishingConnection(row publishingRow) (publishing.ChannelConnection, error) {
	var connection publishing.ChannelConnection
	if err := row.Scan(
		&connection.ID, &connection.OwnerID, &connection.Provider, &connection.RemoteChannelID,
		&connection.DisplayName, &connection.State, &connection.Capabilities.CanUpload,
		&connection.Capabilities.CanPublish, &connection.Capabilities.CanSchedule,
		&connection.CreatedAt, &connection.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return publishing.ChannelConnection{}, publishing.ErrConnectionNotFound
		}
		return publishing.ChannelConnection{}, fmt.Errorf("scan publishing connection: %w", err)
	}
	return connection, nil
}

func scanPublishingAttempt(row publishingRow) (publishing.PublishAttempt, error) {
	var attempt publishing.PublishAttempt
	if err := row.Scan(
		&attempt.ID, &attempt.OwnerID, &attempt.ProjectID, &attempt.ConnectionID,
		&attempt.RenderArtifactID, &attempt.RequestID, &attempt.Provider, &attempt.State,
		&attempt.RemoteVideoID, &attempt.Title, &attempt.Description, &attempt.ScheduledAt,
		&attempt.CreatedAt, &attempt.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return publishing.PublishAttempt{}, publishing.ErrAttemptNotFound
		}
		return publishing.PublishAttempt{}, fmt.Errorf("scan publishing attempt: %w", err)
	}
	return attempt, nil
}
