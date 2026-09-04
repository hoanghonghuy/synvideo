package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenevideojob"
)

type SceneVideoOperationRepository struct{ pool *pgxpool.Pool }

func NewSceneVideoOperationRepository(pool *pgxpool.Pool) *SceneVideoOperationRepository {
	return &SceneVideoOperationRepository{pool: pool}
}

func (r *SceneVideoOperationRepository) Get(ctx context.Context, principal project.Principal, projectID, jobID uuid.UUID) (scenevideojob.OperationCheckpoint, error) {
	if r == nil || r.pool == nil || principal.OwnerID == uuid.Nil || projectID == uuid.Nil || jobID == uuid.Nil {
		return scenevideojob.OperationCheckpoint{}, scenevideojob.ErrCheckpointNotFound
	}
	var cp scenevideojob.OperationCheckpoint
	var state string
	if err := r.pool.QueryRow(ctx, `
		SELECT job_id, project_id, state, COALESCE(external_operation_id, '')
		FROM scene_video_operations
		WHERE owner_id = $1 AND project_id = $2 AND job_id = $3
	`, principal.OwnerID, projectID, jobID).Scan(&cp.JobID, &cp.ProjectID, &state, &cp.ExternalOperationID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return scenevideojob.OperationCheckpoint{}, scenevideojob.ErrCheckpointNotFound
		}
		return scenevideojob.OperationCheckpoint{}, fmt.Errorf("get scene video operation checkpoint: %w", err)
	}
	cp.State = scenevideojob.OperationState(state)
	return cp, nil
}

func (r *SceneVideoOperationRepository) SaveSubmitted(ctx context.Context, principal project.Principal, projectID, jobID uuid.UUID, externalID string) (scenevideojob.OperationCheckpoint, error) {
	if r == nil || r.pool == nil || principal.OwnerID == uuid.Nil || projectID == uuid.Nil || jobID == uuid.Nil || externalID == "" {
		return scenevideojob.OperationCheckpoint{}, errors.New("invalid scene video submitted checkpoint")
	}
	var cp scenevideojob.OperationCheckpoint
	var state string
	if err := r.pool.QueryRow(ctx, `
		INSERT INTO scene_video_operations (job_id, owner_id, project_id, state, external_operation_id)
		VALUES ($1, $2, $3, 'submitted', $4)
		ON CONFLICT (job_id) DO UPDATE SET updated_at = now()
		WHERE scene_video_operations.owner_id = EXCLUDED.owner_id
		  AND scene_video_operations.project_id = EXCLUDED.project_id
		  AND scene_video_operations.state = 'submitted'
		  AND scene_video_operations.external_operation_id = EXCLUDED.external_operation_id
		RETURNING job_id, project_id, state, external_operation_id
	`, jobID, principal.OwnerID, projectID, externalID).Scan(&cp.JobID, &cp.ProjectID, &state, &cp.ExternalOperationID); err != nil {
		return scenevideojob.OperationCheckpoint{}, fmt.Errorf("save scene video submitted checkpoint: %w", err)
	}
	cp.State = scenevideojob.OperationState(state)
	return cp, nil
}

func (r *SceneVideoOperationRepository) SaveAmbiguous(ctx context.Context, principal project.Principal, projectID, jobID uuid.UUID) (scenevideojob.OperationCheckpoint, error) {
	if r == nil || r.pool == nil || principal.OwnerID == uuid.Nil || projectID == uuid.Nil || jobID == uuid.Nil {
		return scenevideojob.OperationCheckpoint{}, errors.New("invalid scene video ambiguous checkpoint")
	}
	var cp scenevideojob.OperationCheckpoint
	var state string
	if err := r.pool.QueryRow(ctx, `
		INSERT INTO scene_video_operations (job_id, owner_id, project_id, state, external_operation_id)
		VALUES ($1, $2, $3, 'ambiguous', NULL)
		ON CONFLICT (job_id) DO UPDATE SET updated_at = now()
		WHERE scene_video_operations.owner_id = EXCLUDED.owner_id
		  AND scene_video_operations.project_id = EXCLUDED.project_id
		  AND scene_video_operations.state = 'ambiguous'
		RETURNING job_id, project_id, state, COALESCE(external_operation_id, '')
	`, jobID, principal.OwnerID, projectID).Scan(&cp.JobID, &cp.ProjectID, &state, &cp.ExternalOperationID); err != nil {
		return scenevideojob.OperationCheckpoint{}, fmt.Errorf("save scene video ambiguous checkpoint: %w", err)
	}
	cp.State = scenevideojob.OperationState(state)
	return cp, nil
}
