package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/publishing"
)

var _ publishing.AttemptHistoryRepository = (*PublishingRepository)(nil)
var _ publishing.PublishArtifactRepository = (*PublishingRepository)(nil)

func (r *PublishingRepository) ListAttempts(ctx context.Context, ownerID, projectID uuid.UUID) ([]publishing.PublishAttempt, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil {
		return nil, publishing.ErrInvalidModel
	}
	query := fmt.Sprintf(`SELECT %s FROM publishing_attempts
		WHERE owner_id=$1 AND project_id=$2
		ORDER BY created_at DESC, id DESC
		LIMIT 100`, publishingAttemptFields)
	rows, err := r.pool.Query(ctx, query, ownerID, projectID)
	if err != nil {
		return nil, fmt.Errorf("list publishing attempts: %w", err)
	}
	defer rows.Close()
	items := make([]publishing.PublishAttempt, 0)
	for rows.Next() {
		item, err := scanPublishingAttempt(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list publishing attempts: %w", err)
	}
	return items, nil
}

func (r *PublishingRepository) ListPublishArtifacts(ctx context.Context, ownerID, projectID uuid.UUID) ([]publishing.PublishArtifactSummary, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil {
		return nil, publishing.ErrInvalidModel
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, byte_size, duration_ms, width, height, created_at
		FROM render_artifacts
		WHERE owner_id=$1 AND project_id=$2 AND mime_type='video/mp4'
		ORDER BY created_at DESC, id DESC
		LIMIT 100
	`, ownerID, projectID)
	if err != nil {
		return nil, fmt.Errorf("list publish artifacts: %w", err)
	}
	defer rows.Close()
	items := make([]publishing.PublishArtifactSummary, 0)
	for rows.Next() {
		var item publishing.PublishArtifactSummary
		if err := rows.Scan(&item.ID, &item.ByteSize, &item.DurationMS, &item.Width, &item.Height, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan publish artifact: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list publish artifacts: %w", err)
	}
	return items, nil
}
