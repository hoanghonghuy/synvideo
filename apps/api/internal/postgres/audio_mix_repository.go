package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/audiomix"
)

type AudioMixRepository struct{ pool *pgxpool.Pool }

func NewAudioMixRepository(pool *pgxpool.Pool) *AudioMixRepository {
	return &AudioMixRepository{pool: pool}
}

var _ audiomix.Repository = (*AudioMixRepository)(nil)

const audioMixFields = `
	id, owner_id, project_id, revision, scene_plan_version,
	music_asset_id, music_duration_ms, narration_lineage_id,
	narration_duration_ms, config, created_at, updated_at
`

func (r *AudioMixRepository) GetLatest(ctx context.Context, ownerID, projectID uuid.UUID) (audiomix.Document, error) {
	if !validAudioMixIdentity(ownerID, projectID) {
		return audiomix.Document{}, audiomix.ErrInvalidInput
	}
	row := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM audio_mix_documents
		WHERE owner_id=$1 AND project_id=$2 ORDER BY revision DESC LIMIT 1`, audioMixFields), ownerID, projectID)
	return scanAudioMix(row)
}

func (r *AudioMixRepository) GetRevision(ctx context.Context, ownerID, projectID uuid.UUID, revision int) (audiomix.Document, error) {
	if !validAudioMixIdentity(ownerID, projectID) || revision < 1 {
		return audiomix.Document{}, audiomix.ErrInvalidInput
	}
	row := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM audio_mix_documents
		WHERE owner_id=$1 AND project_id=$2 AND revision=$3`, audioMixFields), ownerID, projectID, revision)
	return scanAudioMix(row)
}

func (r *AudioMixRepository) ListHistory(ctx context.Context, ownerID, projectID uuid.UUID) ([]audiomix.Document, error) {
	if !validAudioMixIdentity(ownerID, projectID) {
		return nil, audiomix.ErrInvalidInput
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT %s FROM audio_mix_documents
		WHERE owner_id=$1 AND project_id=$2 ORDER BY revision DESC, id DESC`, audioMixFields), ownerID, projectID)
	if err != nil {
		return nil, audioMixPersistence("list audio mix history", err)
	}
	defer rows.Close()
	items := make([]audiomix.Document, 0)
	for rows.Next() {
		item, err := scanAudioMix(rows)
		if err != nil {
			return nil, audioMixPersistence("scan audio mix history", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, audioMixPersistence("read audio mix history", err)
	}
	return items, nil
}

func (r *AudioMixRepository) CreateInitial(ctx context.Context, doc audiomix.Document) (audiomix.Document, error) {
	if doc.Revision != 1 {
		return audiomix.Document{}, audiomix.ErrInvalidInput
	}
	return r.createRevision(ctx, doc, 0)
}

func (r *AudioMixRepository) CreateRevision(ctx context.Context, doc audiomix.Document, expectedRevision int) (audiomix.Document, error) {
	if expectedRevision < 1 || doc.Revision != expectedRevision+1 {
		return audiomix.Document{}, audiomix.ErrInvalidInput
	}
	return r.createRevision(ctx, doc, expectedRevision)
}

func (r *AudioMixRepository) createRevision(ctx context.Context, doc audiomix.Document, expectedRevision int) (audiomix.Document, error) {
	if !validAudioMixIdentity(doc.OwnerID, doc.ProjectID) || doc.ID == uuid.Nil || doc.ScenePlanVersion < 1 || doc.MusicAssetID == uuid.Nil || doc.MusicDurationMS <= 0 || doc.NarrationLineageID == uuid.Nil || doc.NarrationDurationMS <= 0 || doc.CreatedAt.IsZero() || doc.UpdatedAt.IsZero() {
		return audiomix.Document{}, audiomix.ErrInvalidInput
	}
	if err := audiomix.ValidateConfig(doc.Config, doc.MusicDurationMS, doc.NarrationDurationMS); err != nil {
		return audiomix.Document{}, err
	}
	configJSON, err := json.Marshal(doc.Config)
	if err != nil {
		return audiomix.Document{}, audiomix.ErrInvalidInput
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return audiomix.Document{}, audioMixPersistence("begin audio mix revision", err)
	}
	defer tx.Rollback(ctx)
	lockIdentity := fmt.Sprintf("audio-mix:%s:%s", doc.OwnerID, doc.ProjectID)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockIdentity); err != nil {
		return audiomix.Document{}, audioMixPersistence("lock audio mix identity", err)
	}

	var musicEligible bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM media_assets WHERE id=$1 AND owner_id=$2 AND project_id=$3
		AND kind='audio' AND deletion_requested_at IS NULL
	)`, doc.MusicAssetID, doc.OwnerID, doc.ProjectID).Scan(&musicEligible); err != nil {
		return audiomix.Document{}, audioMixPersistence("validate audio mix music asset", err)
	}
	if !musicEligible {
		return audiomix.Document{}, audiomix.ErrMusicMissing
	}

	var currentRevision int
	err = tx.QueryRow(ctx, `SELECT revision FROM audio_mix_documents
		WHERE owner_id=$1 AND project_id=$2 ORDER BY revision DESC LIMIT 1`, doc.OwnerID, doc.ProjectID).Scan(&currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		currentRevision = 0
	} else if err != nil {
		return audiomix.Document{}, audioMixPersistence("read audio mix revision", err)
	}
	if currentRevision != expectedRevision {
		return audiomix.Document{}, audiomix.ErrConflict
	}

	row := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO audio_mix_documents (
		id, owner_id, project_id, revision, scene_plan_version,
		music_asset_id, music_duration_ms, narration_lineage_id,
		narration_duration_ms, config, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING %s`, audioMixFields),
		doc.ID, doc.OwnerID, doc.ProjectID, doc.Revision, doc.ScenePlanVersion,
		doc.MusicAssetID, doc.MusicDurationMS, doc.NarrationLineageID,
		doc.NarrationDurationMS, configJSON, doc.CreatedAt, doc.UpdatedAt)
	created, err := scanAudioMix(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return audiomix.Document{}, audiomix.ErrConflict
			case "23503":
				return audiomix.Document{}, audiomix.ErrMusicMissing
			}
		}
		return audiomix.Document{}, audioMixPersistence("insert audio mix revision", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return audiomix.Document{}, audioMixPersistence("commit audio mix revision", err)
	}
	return created, nil
}

func scanAudioMix(row interface{ Scan(...any) error }) (audiomix.Document, error) {
	var doc audiomix.Document
	var configJSON []byte
	if err := row.Scan(
		&doc.ID,
		&doc.OwnerID,
		&doc.ProjectID,
		&doc.Revision,
		&doc.ScenePlanVersion,
		&doc.MusicAssetID,
		&doc.MusicDurationMS,
		&doc.NarrationLineageID,
		&doc.NarrationDurationMS,
		&configJSON,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return audiomix.Document{}, audiomix.ErrNotFound
		}
		return audiomix.Document{}, err
	}
	if json.Unmarshal(configJSON, &doc.Config) != nil {
		return audiomix.Document{}, audiomix.ErrPersistence
	}
	return doc, nil
}

func validAudioMixIdentity(ownerID, projectID uuid.UUID) bool {
	return ownerID != uuid.Nil && projectID != uuid.Nil
}

func audioMixPersistence(operation string, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: %s: %v", audiomix.ErrPersistence, operation, err)
}
