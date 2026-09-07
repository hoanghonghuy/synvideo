package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

// SceneEditorDependencyResolver derives composition dependency state from
// authoritative same-project PostgreSQL records. Browser-provided identities
// are never treated as evidence that a dependency exists or is current.
type SceneEditorDependencyResolver struct {
	pool *pgxpool.Pool
}

func NewSceneEditorDependencyResolver(pool *pgxpool.Pool) *SceneEditorDependencyResolver {
	return &SceneEditorDependencyResolver{pool: pool}
}

var _ sceneeditor.DependencyResolver = (*SceneEditorDependencyResolver)(nil)

func (r *SceneEditorDependencyResolver) State(ctx context.Context, ownerID uuid.UUID, doc sceneeditor.Document) ([]sceneeditor.DependencyState, error) {
	if r == nil || r.pool == nil || ownerID == uuid.Nil || doc.ProjectID == uuid.Nil {
		return []sceneeditor.DependencyState{{State: sceneeditor.StateBroken, Reason: "PROJECT_UNRESOLVABLE"}}, nil
	}

	states := make([]sceneeditor.DependencyState, 0, 1+len(doc.Scenes)*3+1)
	planState, err := r.scenePlanState(ctx, ownerID, doc.ProjectID, doc.ScenePlanVersion)
	if err != nil {
		return nil, err
	}
	states = append(states, planState)

	for _, scene := range doc.Scenes {
		if scene.Visual != nil {
			state, err := r.visualState(ctx, ownerID, doc.ProjectID, doc.ScenePlanVersion, scene.SceneKey, *scene.Visual)
			if err != nil {
				return nil, err
			}
			states = append(states, state)
		}
		if scene.Narration != nil {
			state, err := r.narrationState(ctx, ownerID, doc.ProjectID, doc.ScenePlanVersion, scene.SceneKey, *scene.Narration)
			if err != nil {
				return nil, err
			}
			states = append(states, state)
		}
		if scene.Caption != nil {
			state, err := r.captionState(ctx, ownerID, doc.ProjectID, doc.ScenePlanVersion, scene.SceneKey, *scene.Caption)
			if err != nil {
				return nil, err
			}
			states = append(states, state)
		}
	}
	if doc.AudioMix != nil {
		state, err := r.audioMixState(ctx, ownerID, doc.ProjectID, *doc.AudioMix)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, nil
}

func (r *SceneEditorDependencyResolver) scenePlanState(ctx context.Context, ownerID, projectID uuid.UUID, version int) (sceneeditor.DependencyState, error) {
	var status string
	err := r.pool.QueryRow(ctx, `
		SELECT sp.status
		FROM scene_plans sp
		JOIN projects p ON p.id = sp.project_id
		WHERE p.owner_id = $1 AND sp.project_id = $2 AND sp.version = $3
	`, ownerID, projectID, version).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return sceneeditor.DependencyState{State: sceneeditor.StateBroken, Reason: "SCENE_PLAN_MISSING"}, nil
	}
	if err != nil {
		return sceneeditor.DependencyState{}, fmt.Errorf("resolve scene editor scene plan: %w", err)
	}
	if status != "approved" && status != "superseded" {
		return sceneeditor.DependencyState{State: sceneeditor.StateBroken, Reason: "SCENE_PLAN_NOT_ACCEPTED"}, nil
	}

	var latestApproved int
	if err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(sp.version), 0)
		FROM scene_plans sp
		JOIN projects p ON p.id = sp.project_id
		WHERE p.owner_id = $1 AND sp.project_id = $2 AND sp.status = 'approved'
	`, ownerID, projectID).Scan(&latestApproved); err != nil {
		return sceneeditor.DependencyState{}, fmt.Errorf("resolve latest approved scene plan: %w", err)
	}
	if latestApproved != version {
		return sceneeditor.DependencyState{State: sceneeditor.StateStale, Reason: "SCENE_PLAN_SUPERSEDED"}, nil
	}
	return sceneeditor.DependencyState{State: sceneeditor.StateCurrent}, nil
}

func (r *SceneEditorDependencyResolver) visualState(ctx context.Context, ownerID, projectID uuid.UUID, planVersion int, sceneKey string, ref sceneeditor.VisualRef) (sceneeditor.DependencyState, error) {
	var kind string
	err := r.pool.QueryRow(ctx, `
		SELECT a.kind
		FROM scene_media_bindings b
		JOIN media_assets a ON a.id = b.asset_id
		WHERE b.id = $1 AND b.asset_id = $2
		  AND b.owner_id = $3 AND b.project_id = $4 AND b.scene_plan_version = $5
		  AND b.scene_key = $6 AND b.role = 'primary_visual'
		  AND a.owner_id = $3 AND a.project_id = $4 AND a.deletion_requested_at IS NULL
	`, ref.BindingID, ref.AssetID, ownerID, projectID, planVersion, sceneKey).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return sceneeditor.DependencyState{State: sceneeditor.StateBroken, Reason: "VISUAL_IDENTITY_INVALID"}, nil
	}
	if err != nil {
		return sceneeditor.DependencyState{}, fmt.Errorf("validate scene editor visual identity: %w", err)
	}
	if kind != "image" && kind != "video" {
		return sceneeditor.DependencyState{State: sceneeditor.StateBroken, Reason: "VISUAL_KIND_INVALID"}, nil
	}

	var bindingID, assetID uuid.UUID
	err = r.pool.QueryRow(ctx, `
		SELECT b.id, b.asset_id
		FROM scene_media_bindings b
		JOIN media_assets a ON a.id = b.asset_id
		WHERE b.owner_id = $1 AND b.project_id = $2 AND b.scene_plan_version = $3
		  AND b.scene_key = $4 AND b.role = 'primary_visual' AND b.status = 'active'
		  AND a.owner_id = $1 AND a.project_id = $2 AND a.deletion_requested_at IS NULL
	`, ownerID, projectID, planVersion, sceneKey).Scan(&bindingID, &assetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sceneeditor.DependencyState{State: sceneeditor.StateBroken, Reason: "VISUAL_MISSING"}, nil
	}
	if err != nil {
		return sceneeditor.DependencyState{}, fmt.Errorf("resolve scene editor visual: %w", err)
	}
	if bindingID != ref.BindingID || assetID != ref.AssetID {
		return sceneeditor.DependencyState{State: sceneeditor.StateStale, Reason: "VISUAL_REPLACED"}, nil
	}
	return sceneeditor.DependencyState{State: sceneeditor.StateCurrent}, nil
}

func (r *SceneEditorDependencyResolver) narrationState(ctx context.Context, ownerID, projectID uuid.UUID, planVersion int, sceneKey string, ref sceneeditor.NarrationRef) (sceneeditor.DependencyState, error) {
	var metadata json.RawMessage
	err := r.pool.QueryRow(ctx, `
		SELECT a.metadata
		FROM scene_narration_bindings b
		JOIN media_assets a ON a.id = b.asset_id
		WHERE b.id = $1 AND b.asset_id = $2
		  AND b.owner_id = $3 AND b.project_id = $4 AND b.scene_plan_version = $5
		  AND b.scene_key = $6 AND b.role = 'narration'
		  AND a.owner_id = $3 AND a.project_id = $4 AND a.kind = 'audio'
		  AND a.deletion_requested_at IS NULL
	`, ref.BindingID, ref.AssetID, ownerID, projectID, planVersion, sceneKey).Scan(&metadata)
	if errors.Is(err, pgx.ErrNoRows) {
		return sceneeditor.DependencyState{State: sceneeditor.StateBroken, Reason: "NARRATION_IDENTITY_INVALID"}, nil
	}
	if err != nil {
		return sceneeditor.DependencyState{}, fmt.Errorf("validate scene editor narration identity: %w", err)
	}
	durationMS, err := sceneEditorDurationMSFromMetadata(metadata)
	if err != nil {
		return sceneeditor.DependencyState{State: sceneeditor.StateBroken, Reason: "NARRATION_DURATION_UNRESOLVABLE"}, nil
	}
	if durationMS != ref.DurationMS {
		return sceneeditor.DependencyState{State: sceneeditor.StateBroken, Reason: "NARRATION_DURATION_MISMATCH"}, nil
	}
	if sceneEditorNarrationLineageID(planVersion, sceneKey, ref.BindingID, ref.AssetID, durationMS) != ref.LineageID {
		return sceneeditor.DependencyState{State: sceneeditor.StateBroken, Reason: "NARRATION_LINEAGE_MISMATCH"}, nil
	}

	var bindingID, assetID uuid.UUID
	err = r.pool.QueryRow(ctx, `
		SELECT b.id, b.asset_id
		FROM scene_narration_bindings b
		JOIN media_assets a ON a.id = b.asset_id
		WHERE b.owner_id = $1 AND b.project_id = $2 AND b.scene_plan_version = $3
		  AND b.scene_key = $4 AND b.role = 'narration' AND b.status = 'active'
		  AND a.owner_id = $1 AND a.project_id = $2 AND a.kind = 'audio'
		  AND a.deletion_requested_at IS NULL
	`, ownerID, projectID, planVersion, sceneKey).Scan(&bindingID, &assetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sceneeditor.DependencyState{State: sceneeditor.StateBroken, Reason: "NARRATION_MISSING"}, nil
	}
	if err != nil {
		return sceneeditor.DependencyState{}, fmt.Errorf("resolve scene editor narration: %w", err)
	}
	if bindingID != ref.BindingID || assetID != ref.AssetID {
		return sceneeditor.DependencyState{State: sceneeditor.StateStale, Reason: "NARRATION_REPLACED"}, nil
	}
	return sceneeditor.DependencyState{State: sceneeditor.StateCurrent}, nil
}

func (r *SceneEditorDependencyResolver) captionState(ctx context.Context, ownerID, projectID uuid.UUID, planVersion int, sceneKey string, ref sceneeditor.CaptionRef) (sceneeditor.DependencyState, error) {
	var sourceBindingID, sourceAssetID uuid.UUID
	var sourceDurationMS int64
	var segments json.RawMessage
	var bindingStatus string
	err := r.pool.QueryRow(ctx, `
		SELECT c.source_binding_id, c.source_asset_id, c.source_duration_ms, c.segments, nb.status
		FROM caption_documents c
		JOIN scene_narration_bindings nb ON nb.id = c.source_binding_id
		JOIN media_assets a ON a.id = c.source_asset_id
		WHERE c.id = $1 AND c.owner_id = $2 AND c.project_id = $3
		  AND c.scene_plan_version = $4 AND c.scene_key = $5 AND c.revision = $6
		  AND nb.owner_id = $2 AND nb.project_id = $3 AND nb.scene_plan_version = $4
		  AND nb.scene_key = $5 AND nb.asset_id = c.source_asset_id
		  AND a.owner_id = $2 AND a.project_id = $3 AND a.kind = 'audio'
		  AND a.deletion_requested_at IS NULL
	`, ref.DocumentID, ownerID, projectID, planVersion, sceneKey, ref.Revision).Scan(&sourceBindingID, &sourceAssetID, &sourceDurationMS, &segments, &bindingStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return sceneeditor.DependencyState{State: sceneeditor.StateBroken, Reason: "CAPTION_REVISION_MISSING"}, nil
	}
	if err != nil {
		return sceneeditor.DependencyState{}, fmt.Errorf("resolve scene editor caption: %w", err)
	}
	if sceneEditorNarrationLineageID(planVersion, sceneKey, sourceBindingID, sourceAssetID, sourceDurationMS) != ref.LineageID {
		return sceneeditor.DependencyState{State: sceneeditor.StateBroken, Reason: "CAPTION_LINEAGE_MISMATCH"}, nil
	}
	lastEndMS, err := sceneEditorCaptionLastEndMS(segments)
	if err != nil {
		return sceneeditor.DependencyState{State: sceneeditor.StateBroken, Reason: "CAPTION_TIMING_UNRESOLVABLE"}, nil
	}
	if lastEndMS != ref.LastEndMS {
		return sceneeditor.DependencyState{State: sceneeditor.StateBroken, Reason: "CAPTION_TIMING_MISMATCH"}, nil
	}
	if bindingStatus != "active" {
		return sceneeditor.DependencyState{State: sceneeditor.StateStale, Reason: "CAPTION_SOURCE_SUPERSEDED"}, nil
	}

	var latestRevision int
	if err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(revision), 0)
		FROM caption_documents
		WHERE owner_id = $1 AND project_id = $2 AND scene_plan_version = $3 AND scene_key = $4
	`, ownerID, projectID, planVersion, sceneKey).Scan(&latestRevision); err != nil {
		return sceneeditor.DependencyState{}, fmt.Errorf("resolve latest scene editor caption: %w", err)
	}
	if latestRevision != ref.Revision {
		return sceneeditor.DependencyState{State: sceneeditor.StateStale, Reason: "CAPTION_SUPERSEDED"}, nil
	}
	return sceneeditor.DependencyState{State: sceneeditor.StateCurrent}, nil
}

func (r *SceneEditorDependencyResolver) audioMixState(ctx context.Context, ownerID, projectID uuid.UUID, ref sceneeditor.AudioMixRef) (sceneeditor.DependencyState, error) {
	var musicAssetID, narrationLineageID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT m.music_asset_id, m.narration_lineage_id
		FROM audio_mix_documents m
		JOIN media_assets a ON a.id = m.music_asset_id
		WHERE m.id = $1 AND m.owner_id = $2 AND m.project_id = $3 AND m.revision = $4
		  AND a.owner_id = $2 AND a.project_id = $3 AND a.kind = 'audio'
		  AND a.deletion_requested_at IS NULL
	`, ref.DocumentID, ownerID, projectID, ref.Revision).Scan(&musicAssetID, &narrationLineageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sceneeditor.DependencyState{State: sceneeditor.StateBroken, Reason: "AUDIO_MIX_REVISION_MISSING"}, nil
	}
	if err != nil {
		return sceneeditor.DependencyState{}, fmt.Errorf("resolve scene editor audio mix: %w", err)
	}
	if musicAssetID != ref.MusicAssetID || narrationLineageID != ref.NarrationLineageID {
		return sceneeditor.DependencyState{State: sceneeditor.StateBroken, Reason: "AUDIO_MIX_IDENTITY_MISMATCH"}, nil
	}

	var latestRevision int
	if err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(revision), 0)
		FROM audio_mix_documents
		WHERE owner_id = $1 AND project_id = $2
	`, ownerID, projectID).Scan(&latestRevision); err != nil {
		return sceneeditor.DependencyState{}, fmt.Errorf("resolve latest scene editor audio mix: %w", err)
	}
	if latestRevision != ref.Revision {
		return sceneeditor.DependencyState{State: sceneeditor.StateStale, Reason: "AUDIO_MIX_SUPERSEDED"}, nil
	}
	return sceneeditor.DependencyState{State: sceneeditor.StateCurrent}, nil
}

func sceneEditorDurationMSFromMetadata(raw json.RawMessage) (int64, error) {
	var metadata struct {
		DurationSeconds float64 `json:"duration_seconds"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil || metadata.DurationSeconds <= 0 || math.IsNaN(metadata.DurationSeconds) || math.IsInf(metadata.DurationSeconds, 0) {
		return 0, errors.New("invalid duration metadata")
	}
	durationMS := int64(math.Round(metadata.DurationSeconds * 1000))
	if durationMS <= 0 {
		return 0, errors.New("invalid duration metadata")
	}
	return durationMS, nil
}

func sceneEditorNarrationLineageID(planVersion int, sceneKey string, bindingID, assetID uuid.UUID, durationMS int64) uuid.UUID {
	value := fmt.Sprintf("scene-editor-narration-v1|plan:%d|scene:%s|binding:%s|asset:%s|duration_ms:%d", planVersion, sceneKey, bindingID, assetID, durationMS)
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(value))
}

func sceneEditorCaptionLastEndMS(raw json.RawMessage) (int64, error) {
	var segments []struct {
		EndMS int64 `json:"end_ms"`
	}
	if err := json.Unmarshal(raw, &segments); err != nil || len(segments) == 0 {
		return 0, errors.New("invalid caption segments")
	}
	var lastEndMS int64
	for _, segment := range segments {
		if segment.EndMS <= 0 {
			return 0, errors.New("invalid caption segment timing")
		}
		if segment.EndMS > lastEndMS {
			lastEndMS = segment.EndMS
		}
	}
	return lastEndMS, nil
}
