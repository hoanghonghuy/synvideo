package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
)

type ScenePlanRepository struct {
	pool *pgxpool.Pool
}

func NewScenePlanRepository(pool *pgxpool.Pool) *ScenePlanRepository {
	return &ScenePlanRepository{pool: pool}
}

var _ sceneplan.Repository = (*ScenePlanRepository)(nil)

const scenePlanSelectFields = `
	sp.project_id::text,
	sp.version,
	sp.revision,
	sp.status,
	sp.source_script_version,
	sp.source_proposal_version,
	sp.content_locale,
	sp.scenes,
	sp.created_at,
	sp.updated_at,
	sp.approved_at
`

const scenePlanReturningFields = `
	project_id::text,
	version,
	revision,
	status,
	source_script_version,
	source_proposal_version,
	content_locale,
	scenes,
	created_at,
	updated_at,
	approved_at
`

func scanScenePlan(row pgx.Row) (sceneplan.Plan, error) {
	var plan sceneplan.Plan
	var projectID string
	var status string
	var scenesJSON []byte

	err := row.Scan(
		&projectID,
		&plan.Version,
		&plan.Revision,
		&status,
		&plan.SourceScriptVersion,
		&plan.SourceProposalVersion,
		&plan.ContentLocale,
		&scenesJSON,
		&plan.CreatedAt,
		&plan.UpdatedAt,
		&plan.ApprovedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sceneplan.Plan{}, sceneplan.ErrNotFound
		}
		return sceneplan.Plan{}, err
	}

	plan.ProjectID, err = uuid.Parse(projectID)
	if err != nil {
		return sceneplan.Plan{}, fmt.Errorf("parse scene plan project id: %w", err)
	}
	plan.Status = sceneplan.Status(status)
	if err := json.Unmarshal(scenesJSON, &plan.Scenes); err != nil {
		return sceneplan.Plan{}, fmt.Errorf("unmarshal scene plan scenes: %w", err)
	}
	return plan, nil
}

func (r *ScenePlanRepository) ListVersions(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]sceneplan.Plan, error) {
	var visible bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM projects WHERE owner_id = $1 AND id = $2)
	`, ownerID.String(), projectID.String()).Scan(&visible); err != nil {
		return nil, fmt.Errorf("check project visibility for list scene plans: %w", err)
	}
	if !visible {
		return nil, sceneplan.ErrNotFound
	}

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT %s
		FROM scene_plans sp
		WHERE sp.project_id = $1
		ORDER BY version DESC
	`, scenePlanSelectFields), projectID.String())
	if err != nil {
		return nil, fmt.Errorf("query scene plan list: %w", err)
	}
	defer rows.Close()

	plans := make([]sceneplan.Plan, 0)
	for rows.Next() {
		plan, err := scanScenePlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scene plan list: %w", err)
	}
	return plans, nil
}

func (r *ScenePlanRepository) GetByVersion(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int) (sceneplan.Plan, error) {
	row := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s
		FROM scene_plans sp
		INNER JOIN projects p ON p.id = sp.project_id
		WHERE p.owner_id = $1 AND sp.project_id = $2 AND sp.version = $3
	`, scenePlanSelectFields), ownerID.String(), projectID.String(), version)
	return scanScenePlan(row)
}

func (r *ScenePlanRepository) CreateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, input sceneplan.CreateDraftInput) (sceneplan.Plan, error) {
	if err := input.NormalizeAndValidate(); err != nil {
		return sceneplan.Plan{}, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return sceneplan.Plan{}, fmt.Errorf("begin create scene plan draft tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var contentLocale string
	if err := tx.QueryRow(ctx, `
		SELECT locale FROM projects WHERE owner_id = $1 AND id = $2 FOR UPDATE
	`, ownerID.String(), projectID.String()).Scan(&contentLocale); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sceneplan.Plan{}, sceneplan.ErrNotFound
		}
		return sceneplan.Plan{}, fmt.Errorf("lock project for create scene plan draft: %w", err)
	}

	source, err := loadApprovedScript(ctx, tx, projectID, input.SourceScriptVersion)
	if err != nil {
		return sceneplan.Plan{}, err
	}
	if err := sceneplan.ValidateContentAgainstScript(&input.Content, source); err != nil {
		return sceneplan.Plan{}, err
	}

	var maxVersion int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0) FROM scene_plans WHERE project_id = $1
	`, projectID.String()).Scan(&maxVersion); err != nil {
		return sceneplan.Plan{}, fmt.Errorf("query max scene plan version: %w", err)
	}
	nextVersion := maxVersion + 1

	if _, err := tx.Exec(ctx, `
		UPDATE scene_plans SET status = 'superseded', updated_at = now()
		WHERE project_id = $1 AND status = 'draft'
	`, projectID.String()); err != nil {
		return sceneplan.Plan{}, fmt.Errorf("supersede existing scene plan draft: %w", err)
	}

	scenesJSON, err := json.Marshal(input.Scenes)
	if err != nil {
		return sceneplan.Plan{}, fmt.Errorf("marshal scene plan scenes: %w", err)
	}
	row := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO scene_plans (
			project_id, version, revision, status, source_script_version,
			source_proposal_version, content_locale, scenes, created_at, updated_at
		) VALUES ($1, $2, 1, 'draft', $3, $4, $5, $6, now(), now())
		RETURNING %s
	`, scenePlanReturningFields), projectID.String(), nextVersion, source.Version, source.SourceProposalVersion, contentLocale, scenesJSON)

	created, err := scanScenePlan(row)
	if err != nil {
		return sceneplan.Plan{}, fmt.Errorf("insert scene plan draft: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return sceneplan.Plan{}, fmt.Errorf("commit create scene plan draft: %w", err)
	}
	return created, nil
}

func (r *ScenePlanRepository) UpdateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, input sceneplan.PutInput) (sceneplan.Plan, error) {
	if err := input.NormalizeAndValidate(); err != nil {
		return sceneplan.Plan{}, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return sceneplan.Plan{}, fmt.Errorf("begin update scene plan tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var visible bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM projects WHERE owner_id = $1 AND id = $2)
	`, ownerID.String(), projectID.String()).Scan(&visible); err != nil {
		return sceneplan.Plan{}, fmt.Errorf("check project owner for update scene plan: %w", err)
	}
	if !visible {
		return sceneplan.Plan{}, sceneplan.ErrNotFound
	}

	var currentRevision int
	var currentStatus string
	var sourceVersion int
	if err := tx.QueryRow(ctx, `
		SELECT revision, status, source_script_version
		FROM scene_plans WHERE project_id = $1 AND version = $2 FOR UPDATE
	`, projectID.String(), version).Scan(&currentRevision, &currentStatus, &sourceVersion); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sceneplan.Plan{}, sceneplan.ErrNotFound
		}
		return sceneplan.Plan{}, fmt.Errorf("query current scene plan for update: %w", err)
	}
	if currentStatus != string(sceneplan.StatusDraft) {
		return sceneplan.Plan{}, sceneplan.ErrScenePlanImmutable
	}
	if input.Revision == nil || *input.Revision != currentRevision {
		return sceneplan.Plan{}, sceneplan.ErrStaleRevision
	}

	source, err := loadApprovedScript(ctx, tx, projectID, sourceVersion)
	if err != nil {
		return sceneplan.Plan{}, err
	}
	if err := sceneplan.ValidateContentAgainstScript(&input.Content, source); err != nil {
		return sceneplan.Plan{}, err
	}
	scenesJSON, err := json.Marshal(input.Scenes)
	if err != nil {
		return sceneplan.Plan{}, fmt.Errorf("marshal updated scene plan scenes: %w", err)
	}

	row := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE scene_plans
		SET revision = revision + 1, scenes = $1, updated_at = now()
		WHERE project_id = $2 AND version = $3
		RETURNING %s
	`, scenePlanReturningFields), scenesJSON, projectID.String(), version)
	updated, err := scanScenePlan(row)
	if err != nil {
		return sceneplan.Plan{}, fmt.Errorf("scan updated scene plan: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return sceneplan.Plan{}, fmt.Errorf("commit update scene plan: %w", err)
	}
	return updated, nil
}

func (r *ScenePlanRepository) Approve(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, revision int) (sceneplan.Plan, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return sceneplan.Plan{}, fmt.Errorf("begin approve scene plan tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var visible bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM projects WHERE owner_id = $1 AND id = $2)
	`, ownerID.String(), projectID.String()).Scan(&visible); err != nil {
		return sceneplan.Plan{}, fmt.Errorf("check project owner for approve scene plan: %w", err)
	}
	if !visible {
		return sceneplan.Plan{}, sceneplan.ErrNotFound
	}

	var currentRevision int
	var currentStatus string
	if err := tx.QueryRow(ctx, `
		SELECT revision, status FROM scene_plans
		WHERE project_id = $1 AND version = $2 FOR UPDATE
	`, projectID.String(), version).Scan(&currentRevision, &currentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sceneplan.Plan{}, sceneplan.ErrNotFound
		}
		return sceneplan.Plan{}, fmt.Errorf("query current scene plan for approve: %w", err)
	}
	if currentStatus != string(sceneplan.StatusDraft) {
		return sceneplan.Plan{}, sceneplan.ErrScenePlanImmutable
	}
	if revision != currentRevision {
		return sceneplan.Plan{}, sceneplan.ErrStaleRevision
	}

	row := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE scene_plans
		SET status = 'approved', approved_at = now(), updated_at = now()
		WHERE project_id = $1 AND version = $2
		RETURNING %s
	`, scenePlanReturningFields), projectID.String(), version)
	approved, err := scanScenePlan(row)
	if err != nil {
		return sceneplan.Plan{}, fmt.Errorf("scan approved scene plan: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return sceneplan.Plan{}, fmt.Errorf("commit approve scene plan: %w", err)
	}
	return approved, nil
}

func loadApprovedScript(ctx context.Context, tx pgx.Tx, projectID uuid.UUID, version int) (script.Script, error) {
	var status string
	var sourceProposalVersion int
	var contentLocale string
	var sectionsJSON []byte
	var proposalStatus string
	if err := tx.QueryRow(ctx, `
		SELECT s.status, s.source_proposal_version, s.content_locale, s.sections,
		       COALESCE(p.status, '')
		FROM scripts s
		LEFT JOIN creative_proposals p
			ON p.project_id = s.project_id AND p.version = s.source_proposal_version
		WHERE s.project_id = $1 AND s.version = $2
	`, projectID.String(), version).Scan(&status, &sourceProposalVersion, &contentLocale, &sectionsJSON, &proposalStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return script.Script{}, sceneplan.ErrScriptSourceInvalid
		}
		return script.Script{}, fmt.Errorf("load scene plan source script: %w", err)
	}
	if status != string(script.StatusApproved) {
		return script.Script{Status: script.Status(status)}, sceneplan.ErrScriptNotApproved
	}
	if proposalStatus != "approved" || sourceProposalVersion < 1 {
		return script.Script{}, sceneplan.ErrScriptSourceInvalid
	}

	var sections []script.Section
	if err := json.Unmarshal(sectionsJSON, &sections); err != nil {
		return script.Script{}, fmt.Errorf("unmarshal source script sections: %w", err)
	}
	return script.Script{
		ProjectID:             projectID,
		Version:               version,
		Status:                script.StatusApproved,
		SourceProposalVersion: sourceProposalVersion,
		ContentLocale:         contentLocale,
		Sections:              sections,
	}, nil
}
