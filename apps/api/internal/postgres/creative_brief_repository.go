package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativebrief"
)

type CreativeBriefRepository struct {
	pool *pgxpool.Pool
}

func NewCreativeBriefRepository(pool *pgxpool.Pool) *CreativeBriefRepository {
	return &CreativeBriefRepository{pool: pool}
}

func (r *CreativeBriefRepository) Get(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) (creativebrief.CreativeBrief, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT cb.project_id::text, cb.revision, cb.source_text, cb.target_audience, cb.objective,
			cb.desired_style, cb.tone, cb.distribution_targets, cb.call_to_action, cb.must_include,
			cb.must_avoid, cb.created_at, cb.updated_at
		FROM creative_briefs cb
		INNER JOIN projects p ON p.id = cb.project_id
		WHERE p.owner_id = $1 AND cb.project_id = $2
	`, ownerID.String(), projectID.String())
	return scanCreativeBrief(row)
}

func (r *CreativeBriefRepository) Put(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, input creativebrief.PutInput) (creativebrief.CreativeBrief, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return creativebrief.CreativeBrief{}, false, fmt.Errorf("begin creative brief put: %w", err)
	}
	defer tx.Rollback(ctx)

	var projectVisible bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM projects WHERE owner_id = $1 AND id = $2)
	`, ownerID.String(), projectID.String()).Scan(&projectVisible); err != nil {
		return creativebrief.CreativeBrief{}, false, fmt.Errorf("check project visibility: %w", err)
	}
	if !projectVisible {
		return creativebrief.CreativeBrief{}, false, creativebrief.ErrNotFound
	}

	if input.Revision == nil {
		created, err := insertCreativeBrief(ctx, tx, projectID, input)
		if err != nil {
			if isUniqueViolation(err) {
				return creativebrief.CreativeBrief{}, false, creativebrief.ErrRevisionRequired
			}
			return creativebrief.CreativeBrief{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return creativebrief.CreativeBrief{}, false, fmt.Errorf("commit creative brief create: %w", err)
		}
		return created, true, nil
	}

	updated, err := updateCreativeBrief(ctx, tx, projectID, input)
	if err != nil {
		if errors.Is(err, creativebrief.ErrNotFound) {
			var exists bool
			if checkErr := tx.QueryRow(ctx, `
				SELECT EXISTS (SELECT 1 FROM creative_briefs WHERE project_id = $1)
			`, projectID.String()).Scan(&exists); checkErr != nil {
				return creativebrief.CreativeBrief{}, false, fmt.Errorf("check creative brief revision: %w", checkErr)
			}
			if exists {
				return creativebrief.CreativeBrief{}, false, creativebrief.ErrStaleRevision
			}
			return creativebrief.CreativeBrief{}, false, creativebrief.ErrRevisionUnexpected
		}
		return creativebrief.CreativeBrief{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return creativebrief.CreativeBrief{}, false, fmt.Errorf("commit creative brief update: %w", err)
	}
	return updated, false, nil
}

type creativeBriefRow interface {
	Scan(dest ...any) error
}

type creativeBriefTx interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func insertCreativeBrief(ctx context.Context, tx creativeBriefTx, projectID uuid.UUID, input creativebrief.PutInput) (creativebrief.CreativeBrief, error) {
	row := tx.QueryRow(ctx, `
		INSERT INTO creative_briefs (
			project_id, revision, source_text, target_audience, objective, desired_style, tone,
			distribution_targets, call_to_action, must_include, must_avoid
		)
		VALUES ($1, 1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING project_id::text, revision, source_text, target_audience, objective,
			desired_style, tone, distribution_targets, call_to_action, must_include,
			must_avoid, created_at, updated_at
	`,
		projectID.String(),
		input.SourceText,
		input.TargetAudience,
		input.Objective,
		input.DesiredStyle,
		input.Tone,
		distributionTargetsToStrings(input.DistributionTargets),
		input.CallToAction,
		stringSlice(input.MustInclude),
		stringSlice(input.MustAvoid),
	)
	return scanCreativeBrief(row)
}

func updateCreativeBrief(ctx context.Context, tx creativeBriefTx, projectID uuid.UUID, input creativebrief.PutInput) (creativebrief.CreativeBrief, error) {
	row := tx.QueryRow(ctx, `
		UPDATE creative_briefs
		SET revision = revision + 1,
			source_text = $3,
			target_audience = $4,
			objective = $5,
			desired_style = $6,
			tone = $7,
			distribution_targets = $8,
			call_to_action = $9,
			must_include = $10,
			must_avoid = $11,
			updated_at = now()
		WHERE project_id = $1 AND revision = $2
		RETURNING project_id::text, revision, source_text, target_audience, objective,
			desired_style, tone, distribution_targets, call_to_action, must_include,
			must_avoid, created_at, updated_at
	`,
		projectID.String(),
		*input.Revision,
		input.SourceText,
		input.TargetAudience,
		input.Objective,
		input.DesiredStyle,
		input.Tone,
		distributionTargetsToStrings(input.DistributionTargets),
		input.CallToAction,
		stringSlice(input.MustInclude),
		stringSlice(input.MustAvoid),
	)
	return scanCreativeBrief(row)
}

func scanCreativeBrief(row creativeBriefRow) (creativebrief.CreativeBrief, error) {
	var item creativebrief.CreativeBrief
	var projectID string
	var distributionTargets []string

	err := row.Scan(
		&projectID,
		&item.Revision,
		&item.SourceText,
		&item.TargetAudience,
		&item.Objective,
		&item.DesiredStyle,
		&item.Tone,
		&distributionTargets,
		&item.CallToAction,
		&item.MustInclude,
		&item.MustAvoid,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return creativebrief.CreativeBrief{}, creativebrief.ErrNotFound
	}
	if err != nil {
		return creativebrief.CreativeBrief{}, fmt.Errorf("scan creative brief: %w", err)
	}

	parsedProjectID, err := uuid.Parse(projectID)
	if err != nil {
		return creativebrief.CreativeBrief{}, fmt.Errorf("parse creative brief project id: %w", err)
	}
	item.ProjectID = parsedProjectID
	item.DistributionTargets = stringsToDistributionTargets(distributionTargets)
	return item, nil
}

func distributionTargetsToStrings(values []creativebrief.DistributionTarget) []string {
	converted := make([]string, 0, len(values))
	for _, value := range values {
		converted = append(converted, string(value))
	}
	return converted
}

func stringsToDistributionTargets(values []string) []creativebrief.DistributionTarget {
	converted := make([]creativebrief.DistributionTarget, 0, len(values))
	for _, value := range values {
		converted = append(converted, creativebrief.DistributionTarget(value))
	}
	return converted
}

func stringSlice(values []string) []string {
	converted := make([]string, 0, len(values))
	converted = append(converted, values...)
	return converted
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
