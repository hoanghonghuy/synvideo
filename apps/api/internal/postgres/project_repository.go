package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type ProjectRepository struct {
	pool *pgxpool.Pool
}

func NewProjectRepository(pool *pgxpool.Pool) *ProjectRepository {
	return &ProjectRepository{pool: pool}
}

func (r *ProjectRepository) Ready(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *ProjectRepository) Create(ctx context.Context, ownerID uuid.UUID, input project.CreateInput) (project.Project, error) {
	id := uuid.New()
	row := r.pool.QueryRow(ctx, `
		INSERT INTO projects (
			id, owner_id, title, description, content_format, aspect_ratio,
			target_duration_seconds, locale, status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'active')
		RETURNING id::text, owner_id::text, title, description, content_format, aspect_ratio,
			target_duration_seconds, locale, status, created_at, updated_at
	`,
		id.String(),
		ownerID.String(),
		input.Title,
		input.Description,
		string(input.ContentFormat),
		string(input.AspectRatio),
		input.TargetDurationSeconds,
		string(input.Locale),
	)
	return scanProject(row)
}

func (r *ProjectRepository) List(ctx context.Context, ownerID uuid.UUID, options project.ListOptions) (project.ListResult, error) {
	limit := options.Limit
	if limit < 1 {
		limit = 20
	}

	var cursorUpdatedAt *time.Time
	var cursorID *string
	if options.Cursor != nil {
		updatedAt := options.Cursor.UpdatedAt
		id := options.Cursor.ID.String()
		cursorUpdatedAt = &updatedAt
		cursorID = &id
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id::text, owner_id::text, title, description, content_format, aspect_ratio,
			target_duration_seconds, locale, status, created_at, updated_at
		FROM projects
		WHERE owner_id = $1
			AND (
				$2::timestamptz IS NULL
				OR (updated_at, id) < ($2::timestamptz, $3::uuid)
			)
		ORDER BY updated_at DESC, id DESC
		LIMIT $4
	`, ownerID.String(), cursorUpdatedAt, cursorID, limit+1)
	if err != nil {
		return project.ListResult{}, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	projects := make([]project.Project, 0, limit)
	for rows.Next() {
		item, err := scanProject(rows)
		if err != nil {
			return project.ListResult{}, err
		}
		projects = append(projects, item)
	}
	if err := rows.Err(); err != nil {
		return project.ListResult{}, fmt.Errorf("read projects: %w", err)
	}

	var nextCursor *project.ListCursor
	if len(projects) > limit {
		nextItem := projects[limit-1]
		nextCursor = &project.ListCursor{UpdatedAt: nextItem.UpdatedAt, ID: nextItem.ID}
		projects = projects[:limit]
	}

	return project.ListResult{Projects: projects, NextCursor: nextCursor}, nil
}

func (r *ProjectRepository) Get(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) (project.Project, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id::text, owner_id::text, title, description, content_format, aspect_ratio,
			target_duration_seconds, locale, status, created_at, updated_at
		FROM projects
		WHERE owner_id = $1 AND id = $2
	`, ownerID.String(), id.String())
	return scanProject(row)
}

func (r *ProjectRepository) Update(ctx context.Context, ownerID uuid.UUID, id uuid.UUID, input project.UpdateInput) (project.Project, error) {
	if input.Title == nil &&
		input.Description == nil &&
		input.ContentFormat == nil &&
		input.AspectRatio == nil &&
		input.TargetDurationSeconds == nil &&
		input.Locale == nil &&
		input.Status == nil {
		return r.Get(ctx, ownerID, id)
	}

	targetSet := input.TargetDurationSeconds != nil

	var targetValue *int
	if targetSet {
		targetValue = *input.TargetDurationSeconds
	}

	row := r.pool.QueryRow(ctx, `
		UPDATE projects
		SET title = COALESCE($3, title),
			description = COALESCE($4, description),
			content_format = COALESCE($5, content_format),
			aspect_ratio = COALESCE($6, aspect_ratio),
			target_duration_seconds = CASE WHEN $7 THEN $8::integer ELSE target_duration_seconds END,
			locale = COALESCE($9, locale),
			status = COALESCE($10, status),
			updated_at = now()
		WHERE owner_id = $1 AND id = $2
		RETURNING id::text, owner_id::text, title, description, content_format, aspect_ratio,
			target_duration_seconds, locale, status, created_at, updated_at
	`,
		ownerID.String(),
		id.String(),
		stringPtr(input.Title),
		stringPtr(input.Description),
		contentFormatPtr(input.ContentFormat),
		aspectRatioPtr(input.AspectRatio),
		targetSet,
		targetValue,
		localePtr(input.Locale),
		statusPtr(input.Status),
	)
	return scanProject(row)
}

type projectRow interface {
	Scan(dest ...any) error
}

func scanProject(row projectRow) (project.Project, error) {
	var item project.Project
	var id string
	var ownerID string
	var contentFormat string
	var aspectRatio string
	var locale string
	var status string
	var targetDuration pgtype.Int4

	err := row.Scan(
		&id,
		&ownerID,
		&item.Title,
		&item.Description,
		&contentFormat,
		&aspectRatio,
		&targetDuration,
		&locale,
		&status,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return project.Project{}, project.ErrNotFound
	}
	if err != nil {
		return project.Project{}, fmt.Errorf("scan project: %w", err)
	}

	parsedID, err := uuid.Parse(id)
	if err != nil {
		return project.Project{}, fmt.Errorf("parse project id: %w", err)
	}
	parsedOwnerID, err := uuid.Parse(ownerID)
	if err != nil {
		return project.Project{}, fmt.Errorf("parse project owner id: %w", err)
	}
	item.ID = parsedID
	item.OwnerID = parsedOwnerID
	item.ContentFormat = project.ContentFormat(contentFormat)
	item.AspectRatio = project.AspectRatio(aspectRatio)
	item.Locale = project.Locale(locale)
	item.Status = project.Status(status)
	if targetDuration.Valid {
		value := int(targetDuration.Int32)
		item.TargetDurationSeconds = &value
	}

	return item, nil
}

func stringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	return value
}

func contentFormatPtr(value *project.ContentFormat) *string {
	if value == nil {
		return nil
	}
	stringValue := string(*value)
	return &stringValue
}

func aspectRatioPtr(value *project.AspectRatio) *string {
	if value == nil {
		return nil
	}
	stringValue := string(*value)
	return &stringValue
}

func localePtr(value *project.Locale) *string {
	if value == nil {
		return nil
	}
	stringValue := string(*value)
	return &stringValue
}

func statusPtr(value *project.Status) *string {
	if value == nil {
		return nil
	}
	stringValue := string(*value)
	return &stringValue
}
