package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarrationjob"
)

type TemporaryObjectRepository struct {
	pool *pgxpool.Pool
}

func NewTemporaryObjectRepository(pool *pgxpool.Pool) *TemporaryObjectRepository {
	return &TemporaryObjectRepository{pool: pool}
}

var _ scenenarrationjob.TemporaryObjectRepository = (*TemporaryObjectRepository)(nil)

func (r *TemporaryObjectRepository) Track(ctx context.Context, object scenenarrationjob.TemporaryObject) error {
	if object.ID == uuid.Nil || object.ProjectID == uuid.Nil || object.JobID == uuid.Nil || object.ObjectKey == "" {
		return errors.New("temporary object identity is incomplete")
	}
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO temporary_objects (
			id, owner_id, project_id, job_id, object_key, state, next_cleanup_at, created_at, updated_at
		)
		SELECT $1, j.owner_id, $2, $3, $4, 'recoverable', now(), now(), now()
		FROM jobs j
		WHERE j.id = $3 AND j.project_id = $2
		ON CONFLICT (project_id, job_id, object_key) DO UPDATE
		SET owner_id = EXCLUDED.owner_id,
			state = 'recoverable',
			updated_at = now()
		WHERE temporary_objects.removed_at IS NULL;
	`, object.ID, object.ProjectID, object.JobID, object.ObjectKey)
	if err != nil {
		return fmt.Errorf("track temporary object: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return errors.New("temporary object job identity is not authoritative or object was already removed")
	}
	return nil
}

func (r *TemporaryObjectRepository) MarkRemoved(ctx context.Context, projectID, jobID uuid.UUID, objectKey string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE temporary_objects
		SET state = 'removed',
			removed_at = COALESCE(removed_at, now()),
			claim_token = NULL,
			claim_until = NULL,
			last_error_code = NULL,
			updated_at = now()
		WHERE project_id = $1 AND job_id = $2 AND object_key = $3 AND removed_at IS NULL;
	`, projectID, jobID, objectKey)
	if err != nil {
		return fmt.Errorf("mark temporary object removed: %w", err)
	}
	return nil
}

func (r *TemporaryObjectRepository) ClaimCleanup(ctx context.Context, limit int, leaseDuration time.Duration) ([]scenenarrationjob.TemporaryObject, error) {
	if limit <= 0 || limit > scenenarrationjob.DefaultCleanupBatchSize || leaseDuration <= 0 {
		return nil, errors.New("invalid temporary object claim options")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin temporary object claim: %w", err)
	}
	defer tx.Rollback(ctx)

	leaseInterval := fmt.Sprintf("%d milliseconds", leaseDuration.Milliseconds())
	claimToken := uuid.New()
	rows, err := tx.Query(ctx, `
		WITH candidates AS (
			SELECT t.id
			FROM temporary_objects t
			LEFT JOIN jobs j
			  ON j.id = t.job_id
			 AND j.owner_id = t.owner_id
			 AND j.project_id = t.project_id
			WHERE t.removed_at IS NULL
			  AND t.next_cleanup_at <= now()
			  AND (t.claim_until IS NULL OR t.claim_until <= now())
			  AND (
				j.state IN ('succeeded', 'failed')
				OR (j.id IS NULL AND t.created_at <= now() - interval '72 hours')
			  )
			ORDER BY t.next_cleanup_at ASC, t.created_at ASC
			FOR UPDATE OF t SKIP LOCKED
			LIMIT $1
		)
		UPDATE temporary_objects t
		SET state = 'cleanup_pending',
			claim_token = $2,
			claim_until = now() + $3::interval,
			cleanup_attempts = cleanup_attempts + 1,
			updated_at = now()
		FROM candidates c
		WHERE t.id = c.id
		RETURNING t.id, t.owner_id, t.project_id, t.job_id, t.object_key, t.claim_token;
	`, limit, claimToken, leaseInterval)
	if err != nil {
		return nil, fmt.Errorf("claim temporary objects: %w", err)
	}
	defer rows.Close()

	objects := make([]scenenarrationjob.TemporaryObject, 0, limit)
	for rows.Next() {
		var object scenenarrationjob.TemporaryObject
		if err := rows.Scan(&object.ID, &object.OwnerID, &object.ProjectID, &object.JobID, &object.ObjectKey, &object.ClaimToken); err != nil {
			return nil, fmt.Errorf("scan temporary object claim: %w", err)
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate temporary object claims: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit temporary object claim: %w", err)
	}
	return objects, nil
}

func (r *TemporaryObjectRepository) CompleteCleanup(ctx context.Context, id, claimToken uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE temporary_objects
		SET state = 'removed',
			removed_at = now(),
			claim_token = NULL,
			claim_until = NULL,
			last_error_code = NULL,
			updated_at = now()
		WHERE id = $1 AND claim_token = $2 AND state = 'cleanup_pending' AND removed_at IS NULL;
	`, id, claimToken)
	if err != nil {
		return fmt.Errorf("complete temporary object cleanup: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return errors.New("temporary object cleanup claim is stale")
	}
	return nil
}

func (r *TemporaryObjectRepository) RetryCleanup(ctx context.Context, id, claimToken uuid.UUID, errorCode string, nextAttempt time.Time) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE temporary_objects
		SET state = 'recoverable',
			next_cleanup_at = $1,
			claim_token = NULL,
			claim_until = NULL,
			last_error_code = $2,
			updated_at = now()
		WHERE id = $3 AND claim_token = $4 AND state = 'cleanup_pending' AND removed_at IS NULL;
	`, nextAttempt.UTC(), errorCode, id, claimToken)
	if err != nil {
		return fmt.Errorf("retry temporary object cleanup: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return errors.New("temporary object cleanup claim is stale")
	}
	return nil
}
