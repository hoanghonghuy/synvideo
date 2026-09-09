package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
)

type JobRepository struct {
	pool *pgxpool.Pool
}

func NewJobRepository(pool *pgxpool.Pool) *JobRepository {
	return &JobRepository{pool: pool}
}

var _ jobs.Repository = (*JobRepository)(nil)

const jobSelectFields = `
	id,
	owner_id,
	project_id,
	kind,
	dedupe_key,
	state,
	attempt,
	max_attempts,
	available_at,
	lease_token,
	lease_until,
	payload,
	result,
	error_code,
	created_at,
	updated_at,
	started_at,
	finished_at,
	cancel_requested_at
`

func scanJob(row pgx.Row) (jobs.Job, error) {
	var j jobs.Job
	var stateStr string
	err := row.Scan(
		&j.ID,
		&j.OwnerID,
		&j.ProjectID,
		&j.Kind,
		&j.DedupeKey,
		&stateStr,
		&j.Attempt,
		&j.MaxAttempts,
		&j.AvailableAt,
		&j.LeaseToken,
		&j.LeaseUntil,
		&j.Payload,
		&j.Result,
		&j.ErrorCode,
		&j.CreatedAt,
		&j.UpdatedAt,
		&j.StartedAt,
		&j.FinishedAt,
		&j.CancelRequestedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return jobs.Job{}, jobs.ErrJobNotFound
		}
		return jobs.Job{}, err
	}
	j.State = jobs.State(stateStr)
	return j, nil
}

func (r *JobRepository) Enqueue(ctx context.Context, in jobs.EnqueueInput) (jobs.Job, error) {
	if in.MaxAttempts <= 0 {
		in.MaxAttempts = 3
	}
	if err := in.Validate(); err != nil {
		return jobs.Job{}, err
	}

	id := in.ID
	if id == uuid.Nil {
		id = uuid.New()
	}

	avail := time.Now().UTC()
	if in.AvailableAt != nil {
		avail = in.AvailableAt.UTC()
	}

	payload := in.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	query := fmt.Sprintf(`
		INSERT INTO jobs (
			id,
			owner_id,
			project_id,
			kind,
			dedupe_key,
			state,
			attempt,
			max_attempts,
			available_at,
			payload,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, 'queued', 0, $6, $7, $8, now(), now())
		RETURNING %s;
	`, jobSelectFields)

	row := r.pool.QueryRow(ctx, query,
		id,
		in.OwnerID,
		in.ProjectID,
		in.Kind,
		in.DedupeKey,
		in.MaxAttempts,
		avail,
		payload,
	)

	job, err := scanJob(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return jobs.Job{}, jobs.ErrDuplicateJob
		}
		return jobs.Job{}, fmt.Errorf("enqueue job: %w", err)
	}

	return job, nil
}

func (r *JobRepository) GetByID(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) (jobs.Job, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM jobs
		WHERE id = $1 AND owner_id = $2;
	`, jobSelectFields)

	row := r.pool.QueryRow(ctx, query, id, ownerID)
	return scanJob(row)
}

func (r *JobRepository) GetByDedupeKey(ctx context.Context, ownerID uuid.UUID, kind string, dedupeKey string) (jobs.Job, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM jobs
		WHERE owner_id = $1 AND kind = $2 AND dedupe_key = $3;
	`, jobSelectFields)
	row := r.pool.QueryRow(ctx, query, ownerID, kind, dedupeKey)
	return scanJob(row)
}

func (r *JobRepository) ListByProjectKind(ctx context.Context, options jobs.ListByProjectKindOptions) ([]jobs.Job, *jobs.ListCursor, error) {
	if options.OwnerID == uuid.Nil || options.ProjectID == uuid.Nil || strings.TrimSpace(options.Kind) == "" {
		return nil, nil, errors.Join(jobs.ErrInvalidInput, errors.New("owner_id, project_id and kind are required"))
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var cursorCreatedAt *time.Time
	var cursorID *uuid.UUID
	if options.Cursor != nil {
		cursorCreatedAt = &options.Cursor.CreatedAt
		cursorID = &options.Cursor.ID
	}

	query := fmt.Sprintf(`
		SELECT %s
		FROM jobs
		WHERE owner_id = $1
		  AND project_id = $2
		  AND kind = $3
		  AND (
			$4::timestamptz IS NULL
			OR created_at < $4::timestamptz
			OR (created_at = $4::timestamptz AND id < $5::uuid)
		  )
		ORDER BY created_at DESC, id DESC
		LIMIT $6;
	`, jobSelectFields)

	rows, err := r.pool.Query(ctx, query, options.OwnerID, options.ProjectID, options.Kind, cursorCreatedAt, cursorID, limit+1)
	if err != nil {
		return nil, nil, fmt.Errorf("list jobs by project kind: %w", err)
	}
	defer rows.Close()

	items := make([]jobs.Job, 0, limit+1)
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, job)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	var next *jobs.ListCursor
	if len(items) > limit {
		last := items[limit-1]
		next = &jobs.ListCursor{CreatedAt: last.CreatedAt, ID: last.ID}
		items = items[:limit]
	}
	return items, next, nil
}

func (r *JobRepository) GetByIDForProject(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, id uuid.UUID) (jobs.Job, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM jobs
		WHERE id = $1 AND owner_id = $2 AND project_id = $3;
	`, jobSelectFields)

	row := r.pool.QueryRow(ctx, query, id, ownerID, projectID)
	return scanJob(row)
}

func (r *JobRepository) ClaimNext(ctx context.Context, options jobs.ClaimOptions) (jobs.Job, error) {
	if len(options.Kinds) == 0 {
		return jobs.Job{}, jobs.ErrNoJobAvailable
	}
	if options.LeaseDuration <= 0 {
		return jobs.Job{}, errors.Join(jobs.ErrInvalidInput, errors.New("lease_duration must be positive"))
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return jobs.Job{}, fmt.Errorf("begin claim tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE jobs
		SET state = 'failed',
			error_code = $1,
			lease_token = NULL,
			lease_until = NULL,
			finished_at = COALESCE(finished_at, now()),
			updated_at = now()
		WHERE kind = ANY($2)
		  AND (
			(state = 'running' AND lease_until <= now() AND attempt >= max_attempts)
			OR (state = 'queued' AND attempt >= max_attempts)
		  );
	`, jobs.ErrorCodeMaxAttemptsExceeded, options.Kinds)
	if err != nil {
		return jobs.Job{}, fmt.Errorf("finalize exhausted jobs: %w", err)
	}

	var jobID uuid.UUID
	findQuery := `
		SELECT id
		FROM jobs
		WHERE (
			(state = 'queued' AND available_at <= now() AND attempt < max_attempts)
			OR
			(state = 'running' AND lease_until <= now() AND attempt < max_attempts)
		)
		AND kind = ANY($1)
		ORDER BY available_at ASC, created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1;
	`

	err = tx.QueryRow(ctx, findQuery, options.Kinds).Scan(&jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if err := tx.Commit(ctx); err != nil {
				return jobs.Job{}, fmt.Errorf("commit exhausted job cleanup: %w", err)
			}
			return jobs.Job{}, jobs.ErrNoJobAvailable
		}
		return jobs.Job{}, fmt.Errorf("find claimable job: %w", err)
	}

	leaseToken := uuid.New()
	leaseInterval := fmt.Sprintf("%d milliseconds", options.LeaseDuration.Milliseconds())

	updateQuery := fmt.Sprintf(`
		UPDATE jobs
		SET state = 'running',
			attempt = attempt + 1,
			lease_token = $1,
			lease_until = now() + $2::interval,
			started_at = COALESCE(started_at, now()),
			updated_at = now()
		WHERE id = $3
		RETURNING %s;
	`, jobSelectFields)

	row := tx.QueryRow(ctx, updateQuery, leaseToken, leaseInterval, jobID)
	job, err := scanJob(row)
	if err != nil {
		return jobs.Job{}, fmt.Errorf("claim job update: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return jobs.Job{}, fmt.Errorf("commit claim tx: %w", err)
	}

	return job, nil
}

func (r *JobRepository) RenewLease(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, extendDuration time.Duration) (jobs.Job, error) {
	if extendDuration <= 0 {
		return jobs.Job{}, errors.Join(jobs.ErrInvalidInput, errors.New("extendDuration must be positive"))
	}
	leaseInterval := fmt.Sprintf("%d milliseconds", extendDuration.Milliseconds())

	query := fmt.Sprintf(`
		UPDATE jobs
		SET lease_until = now() + $1::interval,
			updated_at = now()
		WHERE id = $2 AND lease_token = $3 AND state = 'running'
		RETURNING %s;
	`, jobSelectFields)

	row := r.pool.QueryRow(ctx, query, leaseInterval, id, leaseToken)
	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, jobs.ErrJobNotFound) {
			return jobs.Job{}, jobs.ErrStaleLease
		}
		return jobs.Job{}, err
	}
	return job, nil
}

func (r *JobRepository) RequestCancel(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, id uuid.UUID) (jobs.Job, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil || id == uuid.Nil {
		return jobs.Job{}, jobs.ErrInvalidInput
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return jobs.Job{}, fmt.Errorf("begin cancel tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var current jobs.Job
	row := tx.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM jobs WHERE id = $1 AND owner_id = $2 AND project_id = $3 FOR UPDATE;`, jobSelectFields), id, ownerID, projectID)
	current, err = scanJob(row)
	if err != nil {
		return jobs.Job{}, err
	}

	switch current.State {
	case jobs.StateCancelled:
		if err := tx.Commit(ctx); err != nil {
			return jobs.Job{}, fmt.Errorf("commit cancel tx: %w", err)
		}
		return current, nil
	case jobs.StateSucceeded, jobs.StateFailed:
		return jobs.Job{}, jobs.ErrJobTerminal
	case jobs.StateQueued:
		cancelled, err := scanJob(tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE jobs
			SET state = 'cancelled',
				error_code = $1,
				lease_token = NULL,
				lease_until = NULL,
				finished_at = now(),
				updated_at = now()
			WHERE id = $2 AND owner_id = $3 AND project_id = $4 AND state = 'queued'
			RETURNING %s;
		`, jobSelectFields), jobs.ErrorCodeCancelled, id, ownerID, projectID))
		if err != nil {
			return jobs.Job{}, fmt.Errorf("cancel queued job: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return jobs.Job{}, fmt.Errorf("commit cancel tx: %w", err)
		}
		return cancelled, nil
	case jobs.StateRunning:
		if current.CancelRequestedAt != nil {
			if err := tx.Commit(ctx); err != nil {
				return jobs.Job{}, fmt.Errorf("commit cancel tx: %w", err)
			}
			return current, nil
		}
		requested, err := scanJob(tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE jobs
			SET cancel_requested_at = now(),
				updated_at = now()
			WHERE id = $1 AND owner_id = $2 AND project_id = $3 AND state = 'running'
			RETURNING %s;
		`, jobSelectFields), id, ownerID, projectID))
		if err != nil {
			return jobs.Job{}, fmt.Errorf("request running job cancel: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return jobs.Job{}, fmt.Errorf("commit cancel tx: %w", err)
		}
		return requested, nil
	default:
		return jobs.Job{}, jobs.ErrInvalidInput
	}
}

func (r *JobRepository) IsCancelRequested(ctx context.Context, id uuid.UUID) (bool, error) {
	var requested bool
	if err := r.pool.QueryRow(ctx, `
		SELECT cancel_requested_at IS NOT NULL
		FROM jobs
		WHERE id = $1;
	`, id).Scan(&requested); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, jobs.ErrJobNotFound
		}
		return false, fmt.Errorf("check cancel request: %w", err)
	}
	return requested, nil
}

func (r *JobRepository) MarkCancelled(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID) (jobs.Job, error) {
	query := fmt.Sprintf(`
		UPDATE jobs
		SET state = 'cancelled',
			error_code = $1,
			lease_token = NULL,
			lease_until = NULL,
			finished_at = now(),
			updated_at = now()
		WHERE id = $2 AND lease_token = $3 AND state = 'running'
		  AND cancel_requested_at IS NOT NULL
		RETURNING %s;
	`, jobSelectFields)

	row := r.pool.QueryRow(ctx, query, jobs.ErrorCodeCancelled, id, leaseToken)
	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, jobs.ErrJobNotFound) {
			return jobs.Job{}, jobs.ErrStaleLease
		}
		return jobs.Job{}, err
	}
	return job, nil
}

func (r *JobRepository) MarkSuccess(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, result json.RawMessage) (jobs.Job, error) {
	if err := jobs.ValidateJSONObject(result); err != nil {
		return jobs.Job{}, err
	}

	query := fmt.Sprintf(`
		UPDATE jobs
		SET state = 'succeeded',
			result = $1,
			lease_token = NULL,
			lease_until = NULL,
			finished_at = now(),
			updated_at = now()
		WHERE id = $2 AND lease_token = $3 AND state = 'running'
		  AND cancel_requested_at IS NULL
		RETURNING %s;
	`, jobSelectFields)

	row := r.pool.QueryRow(ctx, query, result, id, leaseToken)
	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, jobs.ErrJobNotFound) {
			return jobs.Job{}, jobs.ErrStaleLease
		}
		return jobs.Job{}, err
	}
	return job, nil
}

func (r *JobRepository) MarkRetryableFailure(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, errorCode string, nextAvailableAt time.Time) (jobs.Job, error) {
	query := fmt.Sprintf(`
		UPDATE jobs
		SET state = 'queued',
			available_at = $1,
			lease_token = NULL,
			lease_until = NULL,
			error_code = $2,
			updated_at = now()
		WHERE id = $3 AND lease_token = $4 AND state = 'running'
		  AND attempt < max_attempts
		RETURNING %s;
	`, jobSelectFields)

	row := r.pool.QueryRow(ctx, query, nextAvailableAt.UTC(), errorCode, id, leaseToken)
	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, jobs.ErrJobNotFound) {
			return jobs.Job{}, jobs.ErrStaleLease
		}
		return jobs.Job{}, err
	}
	return job, nil
}

func (r *JobRepository) MarkTerminalFailure(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, errorCode string) (jobs.Job, error) {
	query := fmt.Sprintf(`
		UPDATE jobs
		SET state = 'failed',
			error_code = $1,
			lease_token = NULL,
			lease_until = NULL,
			finished_at = now(),
			updated_at = now()
		WHERE id = $2 AND lease_token = $3 AND state = 'running'
		RETURNING %s;
	`, jobSelectFields)

	row := r.pool.QueryRow(ctx, query, errorCode, id, leaseToken)
	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, jobs.ErrJobNotFound) {
			return jobs.Job{}, jobs.ErrStaleLease
		}
		return jobs.Job{}, err
	}
	return job, nil
}
