package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenemedia"
)

type SceneMediaBindingRepository struct {
	pool *pgxpool.Pool
}

func NewSceneMediaBindingRepository(pool *pgxpool.Pool) *SceneMediaBindingRepository {
	return &SceneMediaBindingRepository{pool: pool}
}

var _ scenemedia.Repository = (*SceneMediaBindingRepository)(nil)

const sceneMediaBindingFields = `
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

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r *SceneMediaBindingRepository) AssignPrimaryVisual(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int, sceneKey string, assetID uuid.UUID) (scenemedia.Binding, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil || scenePlanVersion < 1 || strings.TrimSpace(sceneKey) == "" || assetID == uuid.Nil {
		return scenemedia.Binding{}, scenemedia.ErrInvalidInput
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return scenemedia.Binding{}, persistenceError("begin assign scene media binding", err)
	}
	defer tx.Rollback(ctx)

	if err := checkApprovedPlanAndScene(ctx, tx, ownerID, projectID, scenePlanVersion, sceneKey); err != nil {
		return scenemedia.Binding{}, err
	}
	if err := checkVisualAsset(ctx, tx, ownerID, projectID, assetID); err != nil {
		return scenemedia.Binding{}, err
	}

	// A transaction-scoped advisory lock serializes first assignment and all
	// replacements for one scene without locking unrelated scenes.
	lockIdentity := fmt.Sprintf("%s:%s:%d:%s:%s", ownerID, projectID, scenePlanVersion, sceneKey, scenemedia.RolePrimaryVisual)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockIdentity); err != nil {
		return scenemedia.Binding{}, persistenceError("lock scene media binding identity", err)
	}

	active, err := findActiveBinding(ctx, tx, ownerID, projectID, scenePlanVersion, sceneKey)
	if err != nil && !errors.Is(err, scenemedia.ErrNotFound) {
		return scenemedia.Binding{}, err
	}
	if err == nil {
		if active.AssetID == assetID {
			if err := tx.Commit(ctx); err != nil {
				return scenemedia.Binding{}, persistenceError("commit idempotent scene media binding", err)
			}
			return active, nil
		}
		if _, err := tx.Exec(ctx, `
			UPDATE scene_media_bindings
			SET status = 'superseded', superseded_at = now()
			WHERE id = $1 AND status = 'active'
		`, active.ID); err != nil {
			return scenemedia.Binding{}, persistenceError("supersede scene media binding", err)
		}
	}

	var nextVersion int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(binding_version), 0) + 1
		FROM scene_media_bindings
		WHERE owner_id = $1 AND project_id = $2 AND scene_plan_version = $3
		  AND scene_key = $4 AND role = $5
	`, ownerID, projectID, scenePlanVersion, sceneKey, scenemedia.RolePrimaryVisual).Scan(&nextVersion); err != nil {
		return scenemedia.Binding{}, persistenceError("allocate scene media binding version", err)
	}

	row := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO scene_media_bindings (
			owner_id, project_id, scene_plan_version, scene_key, role,
			binding_version, asset_id, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'active', now())
		RETURNING %s
	`, sceneMediaBindingFields), ownerID, projectID, scenePlanVersion, sceneKey, scenemedia.RolePrimaryVisual, nextVersion, assetID)
	created, err := scanSceneMediaBinding(row)
	if err != nil {
		return scenemedia.Binding{}, persistenceError("insert scene media binding", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return scenemedia.Binding{}, persistenceError("commit scene media binding", err)
	}
	return created, nil
}

func (r *SceneMediaBindingRepository) GetCurrent(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int, sceneKey string) (scenemedia.Binding, error) {
	if err := checkApprovedPlanAndScene(ctx, r.pool, ownerID, projectID, scenePlanVersion, sceneKey); err != nil {
		return scenemedia.Binding{}, err
	}
	row := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s
		FROM scene_media_bindings
		WHERE owner_id = $1 AND project_id = $2 AND scene_plan_version = $3
		  AND scene_key = $4 AND role = $5 AND status = 'active'
	`, sceneMediaBindingFields), ownerID, projectID, scenePlanVersion, sceneKey, scenemedia.RolePrimaryVisual)
	return scanSceneMediaBinding(row)
}

func (r *SceneMediaBindingRepository) ListCurrent(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int) ([]scenemedia.Binding, error) {
	if err := checkApprovedPlanAndScene(ctx, r.pool, ownerID, projectID, scenePlanVersion, ""); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT %s
		FROM scene_media_bindings
		WHERE owner_id = $1 AND project_id = $2 AND scene_plan_version = $3
		  AND role = $4 AND status = 'active'
		ORDER BY scene_key ASC, binding_version DESC
	`, sceneMediaBindingFields), ownerID, projectID, scenePlanVersion, scenemedia.RolePrimaryVisual)
	if err != nil {
		return nil, persistenceError("list current scene media bindings", err)
	}
	defer rows.Close()
	items := make([]scenemedia.Binding, 0)
	for rows.Next() {
		item, err := scanSceneMediaBinding(rows)
		if err != nil {
			return nil, persistenceError("scan current scene media binding", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, persistenceError("read current scene media bindings", err)
	}
	return items, nil
}

func (r *SceneMediaBindingRepository) ListHistory(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int, sceneKey string) ([]scenemedia.Binding, error) {
	if err := checkApprovedPlanAndScene(ctx, r.pool, ownerID, projectID, scenePlanVersion, sceneKey); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT %s
		FROM scene_media_bindings
		WHERE owner_id = $1 AND project_id = $2 AND scene_plan_version = $3
		  AND scene_key = $4 AND role = $5
		ORDER BY binding_version DESC, id DESC
	`, sceneMediaBindingFields), ownerID, projectID, scenePlanVersion, sceneKey, scenemedia.RolePrimaryVisual)
	if err != nil {
		return nil, persistenceError("list scene media binding history", err)
	}
	defer rows.Close()
	items := make([]scenemedia.Binding, 0)
	for rows.Next() {
		item, err := scanSceneMediaBinding(rows)
		if err != nil {
			return nil, persistenceError("scan scene media binding history", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, persistenceError("read scene media binding history", err)
	}
	return items, nil
}

func checkApprovedPlanAndScene(ctx context.Context, db queryRower, ownerID, projectID uuid.UUID, version int, sceneKey string) error {
	if ownerID == uuid.Nil || projectID == uuid.Nil || version < 1 {
		return scenemedia.ErrInvalidInput
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
			return scenemedia.ErrScenePlanNotFound
		}
		return persistenceError("check scene plan for media binding", err)
	}
	if status != "approved" {
		return scenemedia.ErrScenePlanNotApproved
	}
	if !sceneExists {
		return scenemedia.ErrSceneKeyNotFound
	}
	return nil
}

func checkVisualAsset(ctx context.Context, db queryRower, ownerID, projectID, assetID uuid.UUID) error {
	var kind string
	if err := db.QueryRow(ctx, `
		SELECT kind FROM media_assets
		WHERE owner_id = $1 AND project_id = $2 AND id = $3
	`, ownerID, projectID, assetID).Scan(&kind); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return scenemedia.ErrMediaAssetNotFound
		}
		return persistenceError("check media asset for binding", err)
	}
	if kind != "image" && kind != "video" {
		return scenemedia.ErrMediaAssetNotVisual
	}
	return nil
}

func findActiveBinding(ctx context.Context, db queryRower, ownerID, projectID uuid.UUID, version int, sceneKey string) (scenemedia.Binding, error) {
	row := db.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s
		FROM scene_media_bindings
		WHERE owner_id = $1 AND project_id = $2 AND scene_plan_version = $3
		  AND scene_key = $4 AND role = $5 AND status = 'active'
		FOR UPDATE
	`, sceneMediaBindingFields), ownerID, projectID, version, sceneKey, scenemedia.RolePrimaryVisual)
	return scanSceneMediaBinding(row)
}

func scanSceneMediaBinding(row interface{ Scan(...any) error }) (scenemedia.Binding, error) {
	var item scenemedia.Binding
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
			return scenemedia.Binding{}, scenemedia.ErrNotFound
		}
		return scenemedia.Binding{}, err
	}
	item.Role = scenemedia.Role(role)
	item.Status = scenemedia.Status(status)
	return item, nil
}

func persistenceError(operation string, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: %s", scenemedia.ErrPersistenceFailed, operation)
}
