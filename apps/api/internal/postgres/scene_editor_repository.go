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

	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

type SceneEditorRepository struct{ pool *pgxpool.Pool }

func NewSceneEditorRepository(pool *pgxpool.Pool) *SceneEditorRepository {
	return &SceneEditorRepository{pool: pool}
}

var _ sceneeditor.Repository = (*SceneEditorRepository)(nil)

const sceneEditorFields = `id, owner_id, project_id, revision, scene_plan_version, document, created_at, updated_at`

func (r *SceneEditorRepository) GetLatest(ctx context.Context, ownerID, projectID uuid.UUID) (sceneeditor.Document, error) {
	if !validSceneEditorIdentity(ownerID, projectID) {
		return sceneeditor.Document{}, sceneeditor.ErrInvalidInput
	}
	row := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM scene_editor_compositions
		WHERE owner_id=$1 AND project_id=$2 ORDER BY revision DESC LIMIT 1`, sceneEditorFields), ownerID, projectID)
	return scanSceneEditor(row)
}

func (r *SceneEditorRepository) GetRevision(ctx context.Context, ownerID, projectID uuid.UUID, revision int) (sceneeditor.Document, error) {
	if !validSceneEditorIdentity(ownerID, projectID) || revision < 1 {
		return sceneeditor.Document{}, sceneeditor.ErrInvalidInput
	}
	row := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM scene_editor_compositions
		WHERE owner_id=$1 AND project_id=$2 AND revision=$3`, sceneEditorFields), ownerID, projectID, revision)
	return scanSceneEditor(row)
}

func (r *SceneEditorRepository) CreateInitial(ctx context.Context, doc sceneeditor.Document) (sceneeditor.Document, error) {
	if doc.Revision != 1 {
		return sceneeditor.Document{}, sceneeditor.ErrInvalidInput
	}
	return r.createRevision(ctx, doc, 0)
}

func (r *SceneEditorRepository) CreateRevision(ctx context.Context, doc sceneeditor.Document, expectedRevision int) (sceneeditor.Document, error) {
	if expectedRevision < 1 || doc.Revision != expectedRevision+1 {
		return sceneeditor.Document{}, sceneeditor.ErrInvalidInput
	}
	return r.createRevision(ctx, doc, expectedRevision)
}

func (r *SceneEditorRepository) createRevision(ctx context.Context, doc sceneeditor.Document, expectedRevision int) (sceneeditor.Document, error) {
	if !validSceneEditorIdentity(doc.OwnerID, doc.ProjectID) || doc.ID == uuid.Nil || doc.ScenePlanVersion < 1 || doc.CreatedAt.IsZero() || doc.UpdatedAt.IsZero() {
		return sceneeditor.Document{}, sceneeditor.ErrInvalidInput
	}
	if err := sceneeditor.ValidateDocument(doc); err != nil {
		return sceneeditor.Document{}, err
	}
	payload, err := json.Marshal(doc)
	if err != nil {
		return sceneeditor.Document{}, sceneeditor.ErrInvalidInput
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return sceneeditor.Document{}, sceneEditorPersistence("begin scene editor revision", err)
	}
	defer tx.Rollback(ctx)

	lockIdentity := fmt.Sprintf("scene-editor:%s:%s", doc.OwnerID, doc.ProjectID)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockIdentity); err != nil {
		return sceneeditor.Document{}, sceneEditorPersistence("lock scene editor identity", err)
	}

	var projectOwned bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM projects WHERE id=$1 AND owner_id=$2)`, doc.ProjectID, doc.OwnerID).Scan(&projectOwned); err != nil {
		return sceneeditor.Document{}, sceneEditorPersistence("validate scene editor project", err)
	}
	if !projectOwned {
		return sceneeditor.Document{}, sceneeditor.ErrNotFound
	}

	var currentRevision int
	err = tx.QueryRow(ctx, `SELECT revision FROM scene_editor_compositions
		WHERE owner_id=$1 AND project_id=$2 ORDER BY revision DESC LIMIT 1`, doc.OwnerID, doc.ProjectID).Scan(&currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		currentRevision = 0
	} else if err != nil {
		return sceneeditor.Document{}, sceneEditorPersistence("read scene editor revision", err)
	}
	if currentRevision != expectedRevision {
		return sceneeditor.Document{}, sceneeditor.ErrConflict
	}

	row := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO scene_editor_compositions (
		id, owner_id, project_id, revision, scene_plan_version, document, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING %s`, sceneEditorFields),
		doc.ID, doc.OwnerID, doc.ProjectID, doc.Revision, doc.ScenePlanVersion, payload, doc.CreatedAt, doc.UpdatedAt)
	created, err := scanSceneEditor(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return sceneeditor.Document{}, sceneeditor.ErrConflict
		}
		return sceneeditor.Document{}, sceneEditorPersistence("insert scene editor revision", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return sceneeditor.Document{}, sceneEditorPersistence("commit scene editor revision", err)
	}
	return created, nil
}

func (r *SceneEditorRepository) CreateSnapshot(ctx context.Context, ownerID uuid.UUID, snapshot sceneeditor.Snapshot) (sceneeditor.Snapshot, error) {
	if ownerID == uuid.Nil || snapshot.CompositionID == uuid.Nil || snapshot.ProjectID == uuid.Nil || snapshot.Revision < 1 || snapshot.Digest == "" {
		return sceneeditor.Snapshot{}, sceneeditor.ErrInvalidInput
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return sceneeditor.Snapshot{}, sceneeditor.ErrInvalidInput
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO scene_editor_snapshots
		(composition_id, revision, owner_id, project_id, digest, snapshot)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (owner_id, project_id, digest) DO NOTHING`,
		snapshot.CompositionID, snapshot.Revision, ownerID, snapshot.ProjectID, snapshot.Digest, payload)
	if err != nil {
		return sceneeditor.Snapshot{}, sceneEditorPersistence("insert scene editor snapshot", err)
	}
	return r.GetSnapshot(ctx, ownerID, snapshot.ProjectID, snapshot.Digest)
}

func (r *SceneEditorRepository) GetSnapshot(ctx context.Context, ownerID, projectID uuid.UUID, digest string) (sceneeditor.Snapshot, error) {
	if !validSceneEditorIdentity(ownerID, projectID) || digest == "" {
		return sceneeditor.Snapshot{}, sceneeditor.ErrInvalidInput
	}
	var payload []byte
	if err := r.pool.QueryRow(ctx, `SELECT snapshot FROM scene_editor_snapshots
		WHERE owner_id=$1 AND project_id=$2 AND digest=$3`, ownerID, projectID, digest).Scan(&payload); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sceneeditor.Snapshot{}, sceneeditor.ErrNotFound
		}
		return sceneeditor.Snapshot{}, sceneEditorPersistence("read scene editor snapshot", err)
	}
	var snapshot sceneeditor.Snapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return sceneeditor.Snapshot{}, sceneEditorPersistence("decode scene editor snapshot", err)
	}
	return snapshot, nil
}

func scanSceneEditor(row interface{ Scan(...any) error }) (sceneeditor.Document, error) {
	var doc sceneeditor.Document
	var payload []byte
	var id, ownerID, projectID uuid.UUID
	var revision, scenePlanVersion int
	var createdAt, updatedAt time.Time
	if err := row.Scan(&id, &ownerID, &projectID, &revision, &scenePlanVersion, &payload, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sceneeditor.Document{}, sceneeditor.ErrNotFound
		}
		return sceneeditor.Document{}, err
	}
	if err := json.Unmarshal(payload, &doc); err != nil {
		return sceneeditor.Document{}, sceneeditor.ErrPersistence
	}
	doc.ID = id
	doc.OwnerID = ownerID
	doc.ProjectID = projectID
	doc.Revision = revision
	doc.ScenePlanVersion = scenePlanVersion
	doc.CreatedAt = createdAt
	doc.UpdatedAt = updatedAt
	return doc, nil
}

func validSceneEditorIdentity(ownerID, projectID uuid.UUID) bool {
	return ownerID != uuid.Nil && projectID != uuid.Nil
}

func sceneEditorPersistence(operation string, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: %s: %v", sceneeditor.ErrPersistence, operation, err)
}
