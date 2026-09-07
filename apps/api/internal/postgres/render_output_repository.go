package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
)

func (r *MediaAssetRepository) FindSystemRenderByJob(ctx context.Context, ownerID, projectID, jobID uuid.UUID) (mediaasset.MediaAsset, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil || jobID == uuid.Nil {
		return mediaasset.MediaAsset{}, mediaasset.ErrNotFound
	}
	query := fmt.Sprintf(`
		SELECT %s FROM media_assets
		WHERE owner_id=$1 AND project_id=$2
		  AND kind='video' AND origin='system'
		  AND deletion_requested_at IS NULL
		  AND metadata->>'source'=$3
		  AND metadata->>'render_job_id'=$4
		LIMIT 1
	`, mediaAssetSelectFields)
	return scanMediaAsset(r.pool.QueryRow(ctx, query, ownerID, projectID, "render_export_v1", jobID.String()))
}
