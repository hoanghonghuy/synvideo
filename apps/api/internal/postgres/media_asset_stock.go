package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
)

var _ mediaasset.StockOriginRepository = (*MediaAssetRepository)(nil)

func (r *MediaAssetRepository) FindStockOrigin(ctx context.Context, ownerID, projectID uuid.UUID, providerKey, resultID string, kind mediaasset.Kind) (mediaasset.MediaAsset, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM media_assets
		WHERE owner_id = $1
		  AND project_id = $2
		  AND origin = 'stock'
		  AND kind = $3
		  AND metadata ->> 'stock_provider' = $4
		  AND metadata ->> 'stock_result_id' = $5
		  AND deletion_requested_at IS NULL
		LIMIT 1;
	`, mediaAssetSelectFields)
	return scanMediaAsset(r.pool.QueryRow(ctx, query, ownerID, projectID, kind, providerKey, resultID))
}
