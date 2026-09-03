package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
)

func (r *MediaAssetRepository) FindGeneratedByJob(ctx context.Context, ownerID, projectID, jobID uuid.UUID) (mediaasset.MediaAsset, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM media_assets
		WHERE owner_id = $1 AND project_id = $2
		  AND origin IN ('generated_image', 'generated_audio')
		  AND metadata ->> 'job_id' = $3
		  AND deletion_requested_at IS NULL
		ORDER BY created_at ASC, id ASC
		LIMIT 1;
	`, mediaAssetSelectFields)
	return scanMediaAsset(r.pool.QueryRow(ctx, query, ownerID, projectID, jobID.String()))
}
