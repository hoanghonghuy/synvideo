package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
)

type ScriptRepository struct {
	pool *pgxpool.Pool
}

func NewScriptRepository(pool *pgxpool.Pool) *ScriptRepository {
	return &ScriptRepository{pool: pool}
}

var _ script.Repository = (*ScriptRepository)(nil)

const scriptSelectFields = `
	s.project_id::text,
	s.version,
	s.revision,
	s.status,
	s.source_proposal_version,
	s.content_locale,
	s.sections,
	s.estimated_duration_seconds,
	s.notes,
	s.created_at,
	s.updated_at,
	s.approved_at
`

const scriptReturningFields = `
	project_id::text,
	version,
	revision,
	status,
	source_proposal_version,
	content_locale,
	sections,
	estimated_duration_seconds,
	notes,
	created_at,
	updated_at,
	approved_at
`

func scanScript(row pgx.Row) (script.Script, error) {
	var s script.Script
	var projectIDStr string
	var statusStr string
	var sectionsBytes []byte

	err := row.Scan(
		&projectIDStr,
		&s.Version,
		&s.Revision,
		&statusStr,
		&s.SourceProposalVersion,
		&s.ContentLocale,
		&sectionsBytes,
		&s.EstimatedDurationSeconds,
		&s.Notes,
		&s.CreatedAt,
		&s.UpdatedAt,
		&s.ApprovedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return script.Script{}, script.ErrNotFound
		}
		return script.Script{}, err
	}

	pID, err := uuid.Parse(projectIDStr)
	if err != nil {
		return script.Script{}, fmt.Errorf("parse project id: %w", err)
	}
	s.ProjectID = pID
	s.Status = script.Status(statusStr)

	if len(sectionsBytes) > 0 {
		if err := json.Unmarshal(sectionsBytes, &s.Sections); err != nil {
			return script.Script{}, fmt.Errorf("unmarshal sections json: %w", err)
		}
	} else {
		s.Sections = []script.Section{}
	}

	return s, nil
}

func (r *ScriptRepository) ListVersions(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]script.Script, error) {
	var projectVisible bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM projects WHERE owner_id = $1 AND id = $2)
	`, ownerID.String(), projectID.String()).Scan(&projectVisible); err != nil {
		return nil, fmt.Errorf("check project visibility for list scripts: %w", err)
	}
	if !projectVisible {
		return nil, script.ErrNotFound
	}

	query := fmt.Sprintf(`
		SELECT %s
		FROM scripts s
		WHERE s.project_id = $1
		ORDER BY s.version DESC
	`, scriptSelectFields)

	rows, err := r.pool.Query(ctx, query, projectID.String())
	if err != nil {
		return nil, fmt.Errorf("query scripts list: %w", err)
	}
	defer rows.Close()

	items := make([]script.Script, 0)
	for rows.Next() {
		item, err := scanScript(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scripts: %w", err)
	}
	return items, nil
}

func (r *ScriptRepository) GetByVersion(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int) (script.Script, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM scripts s
		INNER JOIN projects p ON p.id = s.project_id
		WHERE p.owner_id = $1 AND s.project_id = $2 AND s.version = $3
	`, scriptSelectFields)

	row := r.pool.QueryRow(ctx, query, ownerID.String(), projectID.String(), version)
	return scanScript(row)
}

func (r *ScriptRepository) CreateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, input script.CreateDraftInput) (script.Script, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return script.Script{}, fmt.Errorf("begin create draft tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock the project row to serialize draft creation for this project and get content locale
	var projectLocale string
	if err := tx.QueryRow(ctx, `
		SELECT locale FROM projects WHERE owner_id = $1 AND id = $2 FOR UPDATE
	`, ownerID.String(), projectID.String()).Scan(&projectLocale); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return script.Script{}, script.ErrNotFound
		}
		return script.Script{}, fmt.Errorf("lock project for create script draft: %w", err)
	}

	contentLocale := projectLocale
	if strings.TrimSpace(input.ContentLocale) != "" {
		contentLocale = strings.TrimSpace(input.ContentLocale)
	}

	// Verify source proposal exists and is approved
	var propStatus string
	if err := tx.QueryRow(ctx, `
		SELECT status FROM creative_proposals WHERE project_id = $1 AND version = $2
	`, projectID.String(), input.SourceProposalVersion).Scan(&propStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return script.Script{}, script.ErrProposalNotApproved
		}
		return script.Script{}, fmt.Errorf("query source proposal status: %w", err)
	}
	if propStatus != string(creativeproposal.StatusApproved) {
		return script.Script{}, script.ErrProposalNotApproved
	}

	if input.SourceGenerationJobID != nil {
		existingRow := tx.QueryRow(ctx, `
			SELECT project_id::text, version, revision, status, source_proposal_version,
				content_locale, sections, estimated_duration_seconds, notes,
				created_at, updated_at, approved_at
			FROM scripts
			WHERE project_id = $1 AND source_generation_job_id = $2
		`, projectID.String(), input.SourceGenerationJobID.String())
		existing, err := scanScript(existingRow)
		if err == nil {
			_ = tx.Commit(ctx)
			existing.SourceGenerationJobID = input.SourceGenerationJobID
			return existing, nil
		} else if !errors.Is(err, script.ErrNotFound) {
			return script.Script{}, fmt.Errorf("check existing generation script: %w", err)
		}
	}

	var maxVersion int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0) FROM scripts WHERE project_id = $1
	`, projectID.String()).Scan(&maxVersion); err != nil {
		return script.Script{}, fmt.Errorf("query max script version: %w", err)
	}
	nextVersion := maxVersion + 1

	// Atomically supersede any existing unapproved draft for this project
	if _, err := tx.Exec(ctx, `
		UPDATE scripts
		SET status = 'superseded', updated_at = now()
		WHERE project_id = $1 AND status = 'draft'
	`, projectID.String()); err != nil {
		return script.Script{}, fmt.Errorf("supersede existing script draft: %w", err)
	}

	sectionsBytes, err := json.Marshal(input.Sections)
	if err != nil {
		return script.Script{}, fmt.Errorf("marshal sections json: %w", err)
	}

	var sourceJobID *string
	if input.SourceGenerationJobID != nil {
		str := input.SourceGenerationJobID.String()
		sourceJobID = &str
	}

	insertQuery := fmt.Sprintf(`
		INSERT INTO scripts (
			project_id,
			version,
			revision,
			status,
			source_proposal_version,
			content_locale,
			sections,
			estimated_duration_seconds,
			notes,
			source_generation_job_id,
			created_at,
			updated_at
		) VALUES ($1, $2, 1, 'draft', $3, $4, $5, $6, $7, $8, now(), now())
		RETURNING %s;
	`, scriptReturningFields)

	row := tx.QueryRow(ctx, insertQuery,
		projectID.String(),
		nextVersion,
		input.SourceProposalVersion,
		contentLocale,
		sectionsBytes,
		input.EstimatedDurationSeconds,
		input.Notes,
		sourceJobID,
	)

	created, err := scanScript(row)
	if err != nil {
		return script.Script{}, fmt.Errorf("insert script draft: %w", err)
	}
	created.SourceGenerationJobID = input.SourceGenerationJobID

	if err := tx.Commit(ctx); err != nil {
		return script.Script{}, fmt.Errorf("commit create script draft: %w", err)
	}

	return created, nil
}

func (r *ScriptRepository) UpdateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, input script.PutInput) (script.Script, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return script.Script{}, fmt.Errorf("begin update draft tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var projectVisible bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM projects WHERE owner_id = $1 AND id = $2)
	`, ownerID.String(), projectID.String()).Scan(&projectVisible); err != nil {
		return script.Script{}, fmt.Errorf("check project owner for update script: %w", err)
	}
	if !projectVisible {
		return script.Script{}, script.ErrNotFound
	}

	var currentRev int
	var currentStatus string
	if err := tx.QueryRow(ctx, `
		SELECT revision, status FROM scripts WHERE project_id = $1 AND version = $2 FOR UPDATE
	`, projectID.String(), version).Scan(&currentRev, &currentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return script.Script{}, script.ErrNotFound
		}
		return script.Script{}, fmt.Errorf("query current script for update: %w", err)
	}

	if currentStatus != string(script.StatusDraft) {
		return script.Script{}, script.ErrScriptImmutable
	}
	if input.Revision == nil || *input.Revision != currentRev {
		return script.Script{}, script.ErrStaleRevision
	}

	sectionsBytes, err := json.Marshal(input.Sections)
	if err != nil {
		return script.Script{}, fmt.Errorf("marshal sections json: %w", err)
	}

	updateQuery := fmt.Sprintf(`
		UPDATE scripts
		SET revision = revision + 1,
			sections = $1,
			estimated_duration_seconds = $2,
			notes = $3,
			updated_at = now()
		WHERE project_id = $4 AND version = $5
		RETURNING %s;
	`, scriptReturningFields)

	row := tx.QueryRow(ctx, updateQuery,
		sectionsBytes,
		input.EstimatedDurationSeconds,
		input.Notes,
		projectID.String(),
		version,
	)

	updated, err := scanScript(row)
	if err != nil {
		return script.Script{}, fmt.Errorf("scan updated script: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return script.Script{}, fmt.Errorf("commit update script tx: %w", err)
	}

	return updated, nil
}

func (r *ScriptRepository) Approve(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, revision int) (script.Script, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return script.Script{}, fmt.Errorf("begin approve script tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var projectVisible bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM projects WHERE owner_id = $1 AND id = $2)
	`, ownerID.String(), projectID.String()).Scan(&projectVisible); err != nil {
		return script.Script{}, fmt.Errorf("check project owner for approve script: %w", err)
	}
	if !projectVisible {
		return script.Script{}, script.ErrNotFound
	}

	var currentRev int
	var currentStatus string
	if err := tx.QueryRow(ctx, `
		SELECT revision, status FROM scripts WHERE project_id = $1 AND version = $2 FOR UPDATE
	`, projectID.String(), version).Scan(&currentRev, &currentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return script.Script{}, script.ErrNotFound
		}
		return script.Script{}, fmt.Errorf("query current script for approve: %w", err)
	}

	if currentStatus != string(script.StatusDraft) {
		return script.Script{}, script.ErrScriptImmutable
	}
	if revision != currentRev {
		return script.Script{}, script.ErrStaleRevision
	}

	updateQuery := fmt.Sprintf(`
		UPDATE scripts
		SET status = 'approved',
			approved_at = now(),
			updated_at = now()
		WHERE project_id = $1 AND version = $2
		RETURNING %s;
	`, scriptReturningFields)

	row := tx.QueryRow(ctx, updateQuery, projectID.String(), version)
	approved, err := scanScript(row)
	if err != nil {
		return script.Script{}, fmt.Errorf("scan approved script: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return script.Script{}, fmt.Errorf("commit approve script tx: %w", err)
	}

	return approved, nil
}
