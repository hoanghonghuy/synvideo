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
	finished_at
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
