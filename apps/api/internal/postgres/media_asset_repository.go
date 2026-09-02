package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
)

type MediaAssetRepository struct{ pool *pgxpool.Pool }

func NewMediaAssetRepository(pool *pgxpool.Pool) *MediaAssetRepository {
	return &MediaAssetRepository{pool: pool}
}

var _ mediaasset.Repository = (*MediaAssetRepository)(nil)

const mediaAssetSelectFields = `
	id, owner_id, project_id, kind, origin, object_key, mime_type,
	byte_size, sha256, original_filename, metadata, created_at, updated_at,
	deletion_requested_at
`

func (r *MediaAssetRepository) Create(ctx context.Context, asset mediaasset.MediaAsset) (mediaasset.MediaAsset, error) {
	metadata := asset.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
		asset.Metadata = metadata
	}
	if err := asset.Validate(); err != nil {
		return mediaasset.MediaAsset{}, err
	}
	query := fmt.Sprintf(`
		INSERT INTO media_assets (
			id, owner_id, project_id, kind, origin, object_key, mime_type,
			byte_size, sha256, original_filename, metadata, created_at, updated_at
		)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		FROM projects
		WHERE id = $3 AND owner_id = $2
		RETURNING %s;
	`, mediaAssetSelectFields)
	return scanMediaAsset(r.pool.QueryRow(ctx, query,
		asset.ID, asset.OwnerID, asset.ProjectID, asset.Kind, asset.Origin,
		asset.ObjectKey, asset.MimeType, asset.ByteSize, asset.SHA256,
		asset.OriginalFilename, metadata, asset.CreatedAt, asset.UpdatedAt,
	))
}

func (r *MediaAssetRepository) Get(ctx context.Context, ownerID, projectID, assetID uuid.UUID) (mediaasset.MediaAsset, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM media_assets
		WHERE owner_id = $1 AND project_id = $2 AND id = $3
		  AND deletion_requested_at IS NULL;
	`, mediaAssetSelectFields)
	return scanMediaAsset(r.pool.QueryRow(ctx, query, ownerID, projectID, assetID))
}

func (r *MediaAssetRepository) List(ctx context.Context, ownerID, projectID uuid.UUID, options mediaasset.ListOptions) (mediaasset.ListResult, error) {
	limit := options.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	query := fmt.Sprintf(`
		SELECT %s FROM media_assets
		WHERE owner_id = $1 AND project_id = $2 AND deletion_requested_at IS NULL
		ORDER BY created_at DESC, id DESC
		LIMIT $3;
	`, mediaAssetSelectFields)
	rows, err := r.pool.Query(ctx, query, ownerID, projectID, limit)
	if err != nil {
		return mediaasset.ListResult{}, fmt.Errorf("list media assets: %w", err)
	}
	defer rows.Close()
	assets := make([]mediaasset.MediaAsset, 0, limit)
	for rows.Next() {
		asset, err := scanMediaAsset(rows)
		if err != nil {
			return mediaasset.ListResult{}, err
		}
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil {
		return mediaasset.ListResult{}, fmt.Errorf("read media assets: %w", err)
	}
	return mediaasset.ListResult{Assets: assets}, nil
}

func (r *MediaAssetRepository) Delete(ctx context.Context, ownerID, projectID, assetID uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		DELETE FROM media_assets WHERE owner_id = $1 AND project_id = $2 AND id = $3;
	`, ownerID, projectID, assetID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "scene_media_bindings_asset_fk" {
			return mediaasset.ErrInUse
		}
		return fmt.Errorf("delete media asset: %w", err)
	}
	if result.RowsAffected() == 0 {
		return mediaasset.ErrNotFound
	}
	return nil
}

func (r *MediaAssetRepository) HasReferences(ctx context.Context, ownerID, projectID, assetID uuid.UUID) (bool, error) {
	var referenced bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM scene_media_bindings
			WHERE owner_id = $1 AND project_id = $2 AND asset_id = $3
		)
	`, ownerID, projectID, assetID).Scan(&referenced)
	if err != nil {
		return false, fmt.Errorf("check media asset references: %w", err)
	}
	return referenced, nil
}

func (r *MediaAssetRepository) BeginDeletion(ctx context.Context, ownerID, projectID, assetID uuid.UUID) (mediaasset.MediaAsset, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return mediaasset.MediaAsset{}, fmt.Errorf("begin media asset deletion: %w", err)
	}
	defer tx.Rollback(ctx)

	identity := fmt.Sprintf("media-asset:%s:%s:%s", ownerID, projectID, assetID)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, identity); err != nil {
		return mediaasset.MediaAsset{}, fmt.Errorf("lock media asset deletion: %w", err)
	}
	row := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s FROM media_assets
		WHERE owner_id = $1 AND project_id = $2 AND id = $3
		FOR UPDATE
	`, mediaAssetSelectFields), ownerID, projectID, assetID)
	asset, err := scanMediaAsset(row)
	if err != nil {
		return mediaasset.MediaAsset{}, err
	}
	var referenced bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM scene_media_bindings
			WHERE owner_id = $1 AND project_id = $2 AND asset_id = $3
		)
	`, ownerID, projectID, assetID).Scan(&referenced); err != nil {
		return mediaasset.MediaAsset{}, fmt.Errorf("check media asset references: %w", err)
	}
	if referenced {
		return mediaasset.MediaAsset{}, mediaasset.ErrInUse
	}
	if _, err := tx.Exec(ctx, `
		UPDATE media_assets
		SET deletion_requested_at = COALESCE(deletion_requested_at, now()), updated_at = now()
		WHERE owner_id = $1 AND project_id = $2 AND id = $3
	`, ownerID, projectID, assetID); err != nil {
		return mediaasset.MediaAsset{}, fmt.Errorf("mark media asset deletion: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return mediaasset.MediaAsset{}, fmt.Errorf("commit media asset deletion tombstone: %w", err)
	}
	markedAt := time.Now().UTC()
	asset.DeletionRequestedAt = &markedAt
	return asset, nil
}

func (r *MediaAssetRepository) FinalizeDeletion(ctx context.Context, ownerID, projectID, assetID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM media_assets
		WHERE owner_id = $1 AND project_id = $2 AND id = $3 AND deletion_requested_at IS NOT NULL
	`, ownerID, projectID, assetID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "scene_media_bindings_asset_fk" {
			return mediaasset.ErrInUse
		}
		return fmt.Errorf("finalize media asset deletion: %w", err)
	}
	return nil
}

type mediaAssetRow interface{ Scan(...any) error }

func scanMediaAsset(row mediaAssetRow) (mediaasset.MediaAsset, error) {
	var asset mediaasset.MediaAsset
	var kind, origin string
	var metadata []byte
	err := row.Scan(
		&asset.ID, &asset.OwnerID, &asset.ProjectID, &kind, &origin,
		&asset.ObjectKey, &asset.MimeType, &asset.ByteSize, &asset.SHA256,
		&asset.OriginalFilename, &metadata, &asset.CreatedAt, &asset.UpdatedAt,
		&asset.DeletionRequestedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return mediaasset.MediaAsset{}, mediaasset.ErrNotFound
	}
	if err != nil {
		return mediaasset.MediaAsset{}, fmt.Errorf("scan media asset: %w", err)
	}
	asset.Kind = mediaasset.Kind(kind)
	asset.Origin = mediaasset.Origin(origin)
	asset.Metadata = append(json.RawMessage(nil), metadata...)
	return asset, nil
}
