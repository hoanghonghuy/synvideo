package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
)

type CreativeProposalRepository struct {
	pool *pgxpool.Pool
}

func NewCreativeProposalRepository(pool *pgxpool.Pool) *CreativeProposalRepository {
	return &CreativeProposalRepository{pool: pool}
}

func (r *CreativeProposalRepository) List(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]creativeproposal.CreativeProposal, error) {
	var projectVisible bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM projects WHERE owner_id = $1 AND id = $2)
	`, ownerID.String(), projectID.String()).Scan(&projectVisible); err != nil {
		return nil, fmt.Errorf("check project visibility for list proposals: %w", err)
	}
	if !projectVisible {
		return nil, creativeproposal.ErrNotFound
	}

	rows, err := r.pool.Query(ctx, `
		SELECT cp.project_id::text, cp.version, cp.revision, cp.status, cp.source_brief_revision,
			cp.title_options, cp.hook_options, cp.audience_summary, cp.objective_summary,
			cp.narrative_angle, cp.estimated_duration_seconds, cp.format_rationale,
			cp.structure, cp.visual_direction, cp.voice_direction, cp.music_direction,
			cp.caption_direction, cp.call_to_action, cp.research_gaps, cp.warnings,
			cp.created_at, cp.updated_at, cp.approved_at
		FROM creative_proposals cp
		WHERE cp.project_id = $1
		ORDER BY cp.version DESC
	`, projectID.String())
	if err != nil {
		return nil, fmt.Errorf("query creative proposals list: %w", err)
	}
	defer rows.Close()

	items := make([]creativeproposal.CreativeProposal, 0)
	for rows.Next() {
		item, err := scanCreativeProposal(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate creative proposals: %w", err)
	}
	return items, nil
}

func (r *CreativeProposalRepository) Get(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int) (creativeproposal.CreativeProposal, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT cp.project_id::text, cp.version, cp.revision, cp.status, cp.source_brief_revision,
			cp.title_options, cp.hook_options, cp.audience_summary, cp.objective_summary,
			cp.narrative_angle, cp.estimated_duration_seconds, cp.format_rationale,
			cp.structure, cp.visual_direction, cp.voice_direction, cp.music_direction,
			cp.caption_direction, cp.call_to_action, cp.research_gaps, cp.warnings,
			cp.created_at, cp.updated_at, cp.approved_at
		FROM creative_proposals cp
		INNER JOIN projects p ON p.id = cp.project_id
		WHERE p.owner_id = $1 AND cp.project_id = $2 AND cp.version = $3
	`, ownerID.String(), projectID.String(), version)

	return scanCreativeProposal(row)
}

func (r *CreativeProposalRepository) CreateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, input creativeproposal.CreateDraftInput) (creativeproposal.CreativeProposal, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return creativeproposal.CreativeProposal{}, fmt.Errorf("begin create draft tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock the project row to serialize draft creation for this project
	var lockedID string
	if err := tx.QueryRow(ctx, `
		SELECT id::text FROM projects WHERE owner_id = $1 AND id = $2 FOR UPDATE
	`, ownerID.String(), projectID.String()).Scan(&lockedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return creativeproposal.CreativeProposal{}, creativeproposal.ErrNotFound
		}
		return creativeproposal.CreativeProposal{}, fmt.Errorf("lock project for create draft: %w", err)
	}

	var maxVersion int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0) FROM creative_proposals WHERE project_id = $1
	`, projectID.String()).Scan(&maxVersion); err != nil {
		return creativeproposal.CreativeProposal{}, fmt.Errorf("query max proposal version: %w", err)
	}
	nextVersion := maxVersion + 1

	// Atomically supersede any existing draft for this project
	if _, err := tx.Exec(ctx, `
		UPDATE creative_proposals
		SET status = 'superseded', updated_at = now()
		WHERE project_id = $1 AND status = 'draft'
	`, projectID.String()); err != nil {
		return creativeproposal.CreativeProposal{}, fmt.Errorf("supersede existing draft: %w", err)
	}

	structureBytes, err := json.Marshal(input.Structure)
	if err != nil {
		return creativeproposal.CreativeProposal{}, fmt.Errorf("marshal structure json: %w", err)
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO creative_proposals (
			project_id, version, revision, status, source_brief_revision,
			title_options, hook_options, audience_summary, objective_summary,
			narrative_angle, estimated_duration_seconds, format_rationale,
			structure, visual_direction, voice_direction, music_direction,
			caption_direction, call_to_action, research_gaps, warnings
		)
		VALUES (
			$1, $2, 1, 'draft', $3,
			$4, $5, $6, $7,
			$8, $9, $10,
			$11, $12, $13, $14,
			$15, $16, $17, $18
		)
		RETURNING project_id::text, version, revision, status, source_brief_revision,
			title_options, hook_options, audience_summary, objective_summary,
			narrative_angle, estimated_duration_seconds, format_rationale,
			structure, visual_direction, voice_direction, music_direction,
			caption_direction, call_to_action, research_gaps, warnings,
			created_at, updated_at, approved_at
	`,
		projectID.String(),
		nextVersion,
		input.SourceBriefRevision,
		stringSlice(input.TitleOptions),
		stringSlice(input.HookOptions),
		input.AudienceSummary,
		input.ObjectiveSummary,
		input.NarrativeAngle,
		input.EstimatedDurationSeconds,
		input.FormatRationale,
		structureBytes,
		input.VisualDirection,
		input.VoiceDirection,
		input.MusicDirection,
		input.CaptionDirection,
		input.CallToAction,
		stringSlice(input.ResearchGaps),
		stringSlice(input.Warnings),
	)

	created, err := scanCreativeProposal(row)
	if err != nil {
		return creativeproposal.CreativeProposal{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return creativeproposal.CreativeProposal{}, fmt.Errorf("commit create draft: %w", err)
	}
	return created, nil
}

func (r *CreativeProposalRepository) UpdateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, input creativeproposal.PutInput) (creativeproposal.CreativeProposal, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return creativeproposal.CreativeProposal{}, fmt.Errorf("begin update draft tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var projectVisible bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM projects WHERE owner_id = $1 AND id = $2)
	`, ownerID.String(), projectID.String()).Scan(&projectVisible); err != nil {
		return creativeproposal.CreativeProposal{}, fmt.Errorf("check project visibility for update draft: %w", err)
	}
	if !projectVisible {
		return creativeproposal.CreativeProposal{}, creativeproposal.ErrNotFound
	}

	structureBytes, err := json.Marshal(input.Structure)
	if err != nil {
		return creativeproposal.CreativeProposal{}, fmt.Errorf("marshal structure json: %w", err)
	}

	row := tx.QueryRow(ctx, `
		UPDATE creative_proposals
		SET revision = revision + 1,
			title_options = $4,
			hook_options = $5,
			audience_summary = $6,
			objective_summary = $7,
			narrative_angle = $8,
			estimated_duration_seconds = $9,
			format_rationale = $10,
			structure = $11,
			visual_direction = $12,
			voice_direction = $13,
			music_direction = $14,
			caption_direction = $15,
			call_to_action = $16,
			research_gaps = $17,
			warnings = $18,
			updated_at = now()
		WHERE project_id = $1 AND version = $2 AND revision = $3 AND status = 'draft'
		RETURNING project_id::text, version, revision, status, source_brief_revision,
			title_options, hook_options, audience_summary, objective_summary,
			narrative_angle, estimated_duration_seconds, format_rationale,
			structure, visual_direction, voice_direction, music_direction,
			caption_direction, call_to_action, research_gaps, warnings,
			created_at, updated_at, approved_at
	`,
		projectID.String(),
		version,
		*input.Revision,
		stringSlice(input.TitleOptions),
		stringSlice(input.HookOptions),
		input.AudienceSummary,
		input.ObjectiveSummary,
		input.NarrativeAngle,
		input.EstimatedDurationSeconds,
		input.FormatRationale,
		structureBytes,
		input.VisualDirection,
		input.VoiceDirection,
		input.MusicDirection,
		input.CaptionDirection,
		input.CallToAction,
		stringSlice(input.ResearchGaps),
		stringSlice(input.Warnings),
	)

	updated, err := scanCreativeProposal(row)
	if err != nil {
		if errors.Is(err, creativeproposal.ErrNotFound) {
			var currentStatus string
			var currentRevision int
			if checkErr := tx.QueryRow(ctx, `
				SELECT status, revision FROM creative_proposals WHERE project_id = $1 AND version = $2
			`, projectID.String(), version).Scan(&currentStatus, &currentRevision); checkErr != nil {
				if errors.Is(checkErr, pgx.ErrNoRows) {
					return creativeproposal.CreativeProposal{}, creativeproposal.ErrNotFound
				}
				return creativeproposal.CreativeProposal{}, fmt.Errorf("check proposal state: %w", checkErr)
			}
			if currentStatus != string(creativeproposal.StatusDraft) {
				return creativeproposal.CreativeProposal{}, creativeproposal.ErrProposalImmutable
			}
			if currentRevision != *input.Revision {
				return creativeproposal.CreativeProposal{}, creativeproposal.ErrStaleRevision
			}
		}
		return creativeproposal.CreativeProposal{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return creativeproposal.CreativeProposal{}, fmt.Errorf("commit update draft: %w", err)
	}
	return updated, nil
}

func (r *CreativeProposalRepository) Approve(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, revision int) (creativeproposal.CreativeProposal, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return creativeproposal.CreativeProposal{}, fmt.Errorf("begin approve tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var projectVisible bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM projects WHERE owner_id = $1 AND id = $2)
	`, ownerID.String(), projectID.String()).Scan(&projectVisible); err != nil {
		return creativeproposal.CreativeProposal{}, fmt.Errorf("check project visibility for approve: %w", err)
	}
	if !projectVisible {
		return creativeproposal.CreativeProposal{}, creativeproposal.ErrNotFound
	}

	row := tx.QueryRow(ctx, `
		UPDATE creative_proposals
		SET status = 'approved',
			approved_at = now(),
			updated_at = now()
		WHERE project_id = $1 AND version = $2 AND revision = $3 AND status = 'draft'
		RETURNING project_id::text, version, revision, status, source_brief_revision,
			title_options, hook_options, audience_summary, objective_summary,
			narrative_angle, estimated_duration_seconds, format_rationale,
			structure, visual_direction, voice_direction, music_direction,
			caption_direction, call_to_action, research_gaps, warnings,
			created_at, updated_at, approved_at
	`,
		projectID.String(),
		version,
		revision,
	)

	approved, err := scanCreativeProposal(row)
	if err != nil {
		if errors.Is(err, creativeproposal.ErrNotFound) {
			var currentStatus string
			var currentRevision int
			if checkErr := tx.QueryRow(ctx, `
				SELECT status, revision FROM creative_proposals WHERE project_id = $1 AND version = $2
			`, projectID.String(), version).Scan(&currentStatus, &currentRevision); checkErr != nil {
				if errors.Is(checkErr, pgx.ErrNoRows) {
					return creativeproposal.CreativeProposal{}, creativeproposal.ErrNotFound
				}
				return creativeproposal.CreativeProposal{}, fmt.Errorf("check proposal state on approve: %w", checkErr)
			}
			if currentStatus != string(creativeproposal.StatusDraft) {
				return creativeproposal.CreativeProposal{}, creativeproposal.ErrProposalImmutable
			}
			if currentRevision != revision {
				return creativeproposal.CreativeProposal{}, creativeproposal.ErrStaleRevision
			}
		}
		return creativeproposal.CreativeProposal{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return creativeproposal.CreativeProposal{}, fmt.Errorf("commit approve: %w", err)
	}
	return approved, nil
}

type creativeProposalRow interface {
	Scan(dest ...any) error
}

func scanCreativeProposal(row creativeProposalRow) (creativeproposal.CreativeProposal, error) {
	var item creativeproposal.CreativeProposal
	var projectIDStr string
	var statusStr string
	var structureBytes []byte

	err := row.Scan(
		&projectIDStr,
		&item.Version,
		&item.Revision,
		&statusStr,
		&item.SourceBriefRevision,
		&item.TitleOptions,
		&item.HookOptions,
		&item.AudienceSummary,
		&item.ObjectiveSummary,
		&item.NarrativeAngle,
		&item.EstimatedDurationSeconds,
		&item.FormatRationale,
		&structureBytes,
		&item.VisualDirection,
		&item.VoiceDirection,
		&item.MusicDirection,
		&item.CaptionDirection,
		&item.CallToAction,
		&item.ResearchGaps,
		&item.Warnings,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.ApprovedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return creativeproposal.CreativeProposal{}, creativeproposal.ErrNotFound
	}
	if err != nil {
		return creativeproposal.CreativeProposal{}, fmt.Errorf("scan creative proposal: %w", err)
	}

	parsedProjectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		return creativeproposal.CreativeProposal{}, fmt.Errorf("parse creative proposal project id: %w", err)
	}
	item.ProjectID = parsedProjectID
	item.Status = creativeproposal.Status(statusStr)

	if len(structureBytes) > 0 {
		if err := json.Unmarshal(structureBytes, &item.Structure); err != nil {
			return creativeproposal.CreativeProposal{}, fmt.Errorf("unmarshal structure json: %w", err)
		}
	}
	if item.TitleOptions == nil {
		item.TitleOptions = []string{}
	}
	if item.HookOptions == nil {
		item.HookOptions = []string{}
	}
	if item.Structure == nil {
		item.Structure = []creativeproposal.StructureItem{}
	}
	if item.ResearchGaps == nil {
		item.ResearchGaps = []string{}
	}
	if item.Warnings == nil {
		item.Warnings = []string{}
	}

	return item, nil
}

var _ creativeproposal.Repository = (*CreativeProposalRepository)(nil)
