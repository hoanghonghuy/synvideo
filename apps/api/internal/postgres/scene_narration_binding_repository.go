package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarration"
)

type SceneNarrationBindingRepository struct {
	pool *pgxpool.Pool
}

func NewSceneNarrationBindingRepository(pool *pgxpool.Pool) *SceneNarrationBindingRepository {
	return &SceneNarrationBindingRepository{pool: pool}
}

var _ scenenarration.Repository = (*SceneNarrationBindingRepository)(nil)

const sceneNarrationBindingFields = `
	id,
	owner_id,
	project_id,
	scene_plan_version,
	scene_key,
	role,
	binding_version,
	asset_id,
	status,
	created_at,
	superseded_at
`

func (r *SceneNarrationBindingRepository) Assign(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int, sceneKey string, assetID uuid.UUID) (scenenarration.Binding, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil || scenePlanVersion < 1 || strings.TrimSpace(sceneKey) == "" || assetID == uuid.Nil {
		return scenenarration.Binding{}, scenenarration.ErrInvalidInput
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return scenenarration.Binding{}, persistenceNarrationError("begin assign scene narration binding", err)
	}
	defer tx.Rollback(ctx)

	if err := checkApprovedPlanAndSceneForNarration(ctx, tx, ownerID, projectID, scenePlanVersion, sceneKey); err != nil {
		return scenenarration.Binding{}, err
	}
	assetLockIdentity := fmt.Sprintf("media-asset:%s:%s:%s", ownerID, projectID, assetID)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, assetLockIdentity); err != nil {
		return scenenarration.Binding{}, persistenceNarrationError("lock media asset for scene narration binding", err)
	}
	if err := checkAudioAsset(ctx, tx, ownerID, projectID, assetID); err != nil {
		return scenenarration.Binding{}, err
	}

	// Transaction-scoped advisory lock serializes assignment for one scene narration
	lockIdentity := fmt.Sprintf("%s:%s:%d:%s:%s", ownerID, projectID, scenePlanVersion, sceneKey, scenenarration.RoleNarration)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockIdentity); err != nil {
		return scenenarration.Binding{}, persistenceNarrationError("lock scene narration binding identity", err)
	}

	active, err := findActiveNarrationBinding(ctx, tx, ownerID, projectID, scenePlanVersion, sceneKey)
	if err != nil && !errors.Is(err, scenenarration.ErrNotFound) {
		return scenenarration.Binding{}, err
	}
	if err == nil {
		if active.AssetID == assetID {
			if err := tx.Commit(ctx); err != nil {
				return scenenarration.Binding{}, persistenceNarrationError("commit idempotent scene narration binding", err)
			}
			return active, nil
		}
		if _, err := tx.Exec(ctx, `
			UPDATE scene_narration_bindings
			SET status = 'superseded', superseded_at = now()
			WHERE id = $1 AND status = 'active'
		`, active.ID); err != nil {
			return scenenarration.Binding{}, persistenceNarrationError("supersede scene narration binding", err)
		}
	}

	var nextVersion int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(binding_version), 0) + 1
		FROM scene_narration_bindings
		WHERE owner_id = $1 AND project_id = $2 AND scene_plan_version = $3
		  AND scene_key = $4 AND role = $5
	`, ownerID, projectID, scenePlanVersion, sceneKey, scenenarration.RoleNarration).Scan(&nextVersion); err != nil {
		return scenenarration.Binding{}, persistenceNarrationError("allocate scene narration binding version", err)
	}

	row := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO scene_narration_bindings (
			owner_id, project_id, scene_plan_version, scene_key, role,
			binding_version, asset_id, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'active', now())
		RETURNING %s
	`, sceneNarrationBindingFields), ownerID, projectID, scenePlanVersion, sceneKey, scenenarration.RoleNarration, nextVersion, assetID)
	created, err := scanSceneNarrationBinding(row)
	if err != nil {
		return scenenarration.Binding{}, persistenceNarrationError("insert scene narration binding", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return scenenarration.Binding{}, persistenceNarrationError("commit scene narration binding", err)
	}
	return created, nil
}

func (r *SceneNarrationBindingRepository) GetActive(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int, sceneKey string) (scenenarration.Binding, error) {
	if err := checkApprovedPlanAndSceneForNarration(ctx, r.pool, ownerID, projectID, scenePlanVersion, sceneKey); err != nil {
		return scenenarration.Binding{}, err
	}
	row := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s
		FROM scene_narration_bindings
		WHERE owner_id = $1 AND project_id = $2 AND scene_plan_version = $3
		  AND scene_key = $4 AND role = $5 AND status = 'active'
	`, sceneNarrationBindingFields), ownerID, projectID, scenePlanVersion, sceneKey, scenenarration.RoleNarration)
	return scanSceneNarrationBinding(row)
}

func (r *SceneNarrationBindingRepository) ListActiveForPlan(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int) ([]scenenarration.Binding, error) {
	if err := checkApprovedPlanAndSceneForNarration(ctx, r.pool, ownerID, projectID, scenePlanVersion, ""); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT %s
		FROM scene_narration_bindings
		WHERE owner_id = $1 AND project_id = $2 AND scene_plan_version = $3
		  AND role = $4 AND status = 'active'
		ORDER BY scene_key ASC, binding_version DESC
	`, sceneNarrationBindingFields), ownerID, projectID, scenePlanVersion, scenenarration.RoleNarration)
	if err != nil {
		return nil, persistenceNarrationError("list active scene narration bindings", err)
	}
	defer rows.Close()
	items := make([]scenenarration.Binding, 0)
	for rows.Next() {
		item, err := scanSceneNarrationBinding(rows)
		if err != nil {
			return nil, persistenceNarrationError("scan active scene narration binding", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, persistenceNarrationError("read active scene narration bindings", err)
	}
	return items, nil
}

func (r *SceneNarrationBindingRepository) ListHistory(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int, sceneKey string) ([]scenenarration.Binding, error) {
	if err := checkApprovedPlanAndSceneForNarration(ctx, r.pool, ownerID, projectID, scenePlanVersion, sceneKey); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT %s
		FROM scene_narration_bindings
		WHERE owner_id = $1 AND project_id = $2 AND scene_plan_version = $3
		  AND scene_key = $4 AND role = $5
		ORDER BY binding_version DESC, id DESC
	`, sceneNarrationBindingFields), ownerID, projectID, scenePlanVersion, sceneKey, scenenarration.RoleNarration)
	if err != nil {
		return nil, persistenceNarrationError("list scene narration binding history", err)
	}
	defer rows.Close()
	items := make([]scenenarration.Binding, 0)
	for rows.Next() {
		item, err := scanSceneNarrationBinding(rows)
		if err != nil {
			return nil, persistenceNarrationError("scan scene narration binding history", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, persistenceNarrationError("read scene narration binding history", err)
	}
	return items, nil
}

func checkApprovedPlanAndSceneForNarration(ctx context.Context, db queryRower, ownerID, projectID uuid.UUID, version int, sceneKey string) error {
	if ownerID == uuid.Nil || projectID == uuid.Nil || version < 1 {
		return scenenarration.ErrInvalidInput
	}
	var status string
	var sceneExists bool
	row := db.QueryRow(ctx, `
		SELECT sp.status,
		       CASE WHEN $4 = '' THEN true ELSE EXISTS (
				SELECT 1 FROM jsonb_array_elements(sp.scenes) AS scene
				WHERE scene->>'key' = $4
		       ) END
		FROM scene_plans sp
		INNER JOIN projects p ON p.id = sp.project_id
		WHERE p.owner_id = $1 AND sp.project_id = $2 AND sp.version = $3
	`, ownerID, projectID, version, sceneKey)
	if err := row.Scan(&status, &sceneExists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return scenenarration.ErrScenePlanNotFound
		}
		return persistenceNarrationError("check scene plan for narration binding", err)
	}
	if status != "approved" {
		return scenenarration.ErrScenePlanNotApproved
	}
	if !sceneExists {
		return scenenarration.ErrSceneKeyNotFound
	}
	return nil
}

func checkAudioAsset(ctx context.Context, db queryRower, ownerID, projectID, assetID uuid.UUID) error {
	var kind string
	if err := db.QueryRow(ctx, `
		SELECT kind FROM media_assets
		WHERE owner_id = $1 AND project_id = $2 AND id = $3
		  AND deletion_requested_at IS NULL
	`, ownerID, projectID, assetID).Scan(&kind); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return scenenarration.ErrMediaAssetNotFound
		}
		return persistenceNarrationError("check media asset for narration binding", err)
	}
	if kind != "audio" {
		return scenenarration.ErrMediaAssetNotAudio
	}
	return nil
}

func findActiveNarrationBinding(ctx context.Context, db queryRower, ownerID, projectID uuid.UUID, version int, sceneKey string) (scenenarration.Binding, error) {
	row := db.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s
		FROM scene_narration_bindings
		WHERE owner_id = $1 AND project_id = $2 AND scene_plan_version = $3
		  AND scene_key = $4 AND role = $5 AND status = 'active'
		FOR UPDATE
	`, sceneNarrationBindingFields), ownerID, projectID, version, sceneKey, scenenarration.RoleNarration)
	return scanSceneNarrationBinding(row)
}

func scanSceneNarrationBinding(row interface{ Scan(...any) error }) (scenenarration.Binding, error) {
	var item scenenarration.Binding
	var role, status string
	if err := row.Scan(
		&item.ID,
		&item.OwnerID,
		&item.ProjectID,
		&item.ScenePlanVersion,
		&item.SceneKey,
		&role,
		&item.BindingVersion,
		&item.AssetID,
		&status,
		&item.CreatedAt,
		&item.SupersededAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return scenenarration.Binding{}, scenenarration.ErrNotFound
		}
		return scenenarration.Binding{}, err
	}
	item.Role = scenenarration.Role(role)
	item.Status = scenenarration.Status(status)
	return item, nil
}

func persistenceNarrationError(operation string, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: %s: %v", scenenarration.ErrPersistenceFailed, operation, err)
}
