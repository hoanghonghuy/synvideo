package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	byte_size, sha256, original_filename, metadata, created_at, updated_at
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
		WHERE owner_id = $1 AND project_id = $2 AND id = $3;
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
		WHERE owner_id = $1 AND project_id = $2
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
		return fmt.Errorf("delete media asset: %w", err)
	}
	if result.RowsAffected() == 0 {
		return mediaasset.ErrNotFound
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
