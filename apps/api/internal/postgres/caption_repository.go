package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/captions"
)

type CaptionRepository struct{ pool *pgxpool.Pool }

func NewCaptionRepository(pool *pgxpool.Pool) *CaptionRepository {
	return &CaptionRepository{pool: pool}
}

var _ captions.Repository = (*CaptionRepository)(nil)

const captionFields = `
	id, owner_id, project_id, scene_plan_version, scene_key, revision,
	source_binding_id, source_asset_id, source_duration_ms, segments, style, created_at
`

func (r *CaptionRepository) GetLatest(ctx context.Context, ownerID, projectID uuid.UUID, planVersion int, sceneKey string) (captions.Document, error) {
	if !validCaptionIdentity(ownerID, projectID, planVersion, sceneKey) {
		return captions.Document{}, captions.ErrInvalidInput
	}
	row := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM caption_documents
		WHERE owner_id=$1 AND project_id=$2 AND scene_plan_version=$3 AND scene_key=$4
		ORDER BY revision DESC LIMIT 1`, captionFields), ownerID, projectID, planVersion, sceneKey)
	return scanCaption(row)
}

func (r *CaptionRepository) GetRevision(ctx context.Context, ownerID, projectID uuid.UUID, planVersion int, sceneKey string, revision int) (captions.Document, error) {
	if !validCaptionIdentity(ownerID, projectID, planVersion, sceneKey) || revision < 1 {
		return captions.Document{}, captions.ErrInvalidInput
	}
	row := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM caption_documents
		WHERE owner_id=$1 AND project_id=$2 AND scene_plan_version=$3 AND scene_key=$4 AND revision=$5`, captionFields), ownerID, projectID, planVersion, sceneKey, revision)
	return scanCaption(row)
}

func (r *CaptionRepository) ListHistory(ctx context.Context, ownerID, projectID uuid.UUID, planVersion int, sceneKey string) ([]captions.Document, error) {
	if !validCaptionIdentity(ownerID, projectID, planVersion, sceneKey) {
		return nil, captions.ErrInvalidInput
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT %s FROM caption_documents
		WHERE owner_id=$1 AND project_id=$2 AND scene_plan_version=$3 AND scene_key=$4
		ORDER BY revision DESC, id DESC`, captionFields), ownerID, projectID, planVersion, sceneKey)
	if err != nil {
		return nil, captionPersistence("list caption history", err)
	}
	defer rows.Close()
	items := make([]captions.Document, 0)
	for rows.Next() {
		item, err := scanCaption(rows)
		if err != nil {
			return nil, captionPersistence("scan caption history", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, captionPersistence("read caption history", err)
	}
	return items, nil
}

func (r *CaptionRepository) CreateInitial(ctx context.Context, doc captions.Document) (captions.Document, error) {
	if doc.Revision != 1 {
		return captions.Document{}, captions.ErrInvalidInput
	}
	return r.createRevision(ctx, doc, 0)
}

func (r *CaptionRepository) CreateRevision(ctx context.Context, doc captions.Document, expectedRevision int) (captions.Document, error) {
	if expectedRevision < 1 || doc.Revision != expectedRevision+1 {
		return captions.Document{}, captions.ErrInvalidInput
	}
	return r.createRevision(ctx, doc, expectedRevision)
}

func (r *CaptionRepository) createRevision(ctx context.Context, doc captions.Document, expectedRevision int) (captions.Document, error) {
	if !validCaptionIdentity(doc.OwnerID, doc.ProjectID, doc.ScenePlanVersion, doc.SceneKey) || doc.ID == uuid.Nil || doc.SourceBindingID == uuid.Nil || doc.SourceAssetID == uuid.Nil || doc.SourceDurationMS <= 0 {
		return captions.Document{}, captions.ErrInvalidInput
	}
	segments, err := captions.NormalizeSegments(doc.Segments, doc.SourceDurationMS)
	if err != nil {
		return captions.Document{}, err
	}
	style, err := captions.NormalizeStyle(doc.Style)
	if err != nil {
		return captions.Document{}, err
	}
	segmentsJSON, err := json.Marshal(segments)
	if err != nil {
		return captions.Document{}, captions.ErrInvalidInput
	}
	styleJSON, err := json.Marshal(style)
	if err != nil {
		return captions.Document{}, captions.ErrInvalidInput
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return captions.Document{}, captionPersistence("begin caption revision", err)
	}
	defer tx.Rollback(ctx)
	lockIdentity := fmt.Sprintf("caption:%s:%s:%d:%s", doc.OwnerID, doc.ProjectID, doc.ScenePlanVersion, doc.SceneKey)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockIdentity); err != nil {
		return captions.Document{}, captionPersistence("lock caption identity", err)
	}

	var currentRevision int
	err = tx.QueryRow(ctx, `SELECT revision FROM caption_documents
		WHERE owner_id=$1 AND project_id=$2 AND scene_plan_version=$3 AND scene_key=$4
		ORDER BY revision DESC LIMIT 1`, doc.OwnerID, doc.ProjectID, doc.ScenePlanVersion, doc.SceneKey).Scan(&currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		currentRevision = 0
	} else if err != nil {
		return captions.Document{}, captionPersistence("read caption revision", err)
	}
	if currentRevision != expectedRevision {
		return captions.Document{}, captions.ErrConflict
	}

	row := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO caption_documents (
		id, owner_id, project_id, scene_plan_version, scene_key, revision,
		source_binding_id, source_asset_id, source_duration_ms, segments, style, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now()) RETURNING %s`, captionFields),
		doc.ID, doc.OwnerID, doc.ProjectID, doc.ScenePlanVersion, doc.SceneKey, doc.Revision,
		doc.SourceBindingID, doc.SourceAssetID, doc.SourceDurationMS, segmentsJSON, styleJSON)
	created, err := scanCaption(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return captions.Document{}, captions.ErrConflict
		}
		return captions.Document{}, captionPersistence("insert caption revision", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return captions.Document{}, captionPersistence("commit caption revision", err)
	}
	return created, nil
}

func scanCaption(row interface{ Scan(...any) error }) (captions.Document, error) {
	var doc captions.Document
	var segmentsJSON, styleJSON []byte
	if err := row.Scan(
		&doc.ID,
		&doc.OwnerID,
		&doc.ProjectID,
		&doc.ScenePlanVersion,
		&doc.SceneKey,
		&doc.Revision,
		&doc.SourceBindingID,
		&doc.SourceAssetID,
		&doc.SourceDurationMS,
		&segmentsJSON,
		&styleJSON,
		&doc.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return captions.Document{}, captions.ErrNotFound
		}
		return captions.Document{}, err
	}
	if json.Unmarshal(segmentsJSON, &doc.Segments) != nil || json.Unmarshal(styleJSON, &doc.Style) != nil {
		return captions.Document{}, captions.ErrPersistence
	}
	return doc, nil
}

func validCaptionIdentity(ownerID, projectID uuid.UUID, planVersion int, sceneKey string) bool {
	return ownerID != uuid.Nil && projectID != uuid.Nil && planVersion > 0 && strings.TrimSpace(sceneKey) != ""
}

func captionPersistence(operation string, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: %s: %v", captions.ErrPersistence, operation, err)
}
