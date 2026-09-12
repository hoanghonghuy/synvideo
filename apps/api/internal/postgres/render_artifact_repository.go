package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/renderexport"
)

type RenderArtifactRepository struct{ pool *pgxpool.Pool }

func NewRenderArtifactRepository(pool *pgxpool.Pool) *RenderArtifactRepository {
	return &RenderArtifactRepository{pool: pool}
}

var _ renderexport.ArtifactRepository = (*RenderArtifactRepository)(nil)

const renderArtifactFields = `
	id, owner_id, project_id, job_id, snapshot_digest, profile_id,
	media_asset_id, subtitle_media_asset_id, byte_size, sha256, mime_type, duration_ms, width,
	height, toolchain_version, created_at
`

func (r *RenderArtifactRepository) CreateForLease(ctx context.Context, leaseToken uuid.UUID, artifact renderexport.RenderArtifact) (renderexport.RenderArtifact, error) {
	if leaseToken == uuid.Nil {
		return renderexport.RenderArtifact{}, renderexport.ErrStaleRenderLease
	}
	if err := artifact.Validate(); err != nil {
		return renderexport.RenderArtifact{}, err
	}
	query := fmt.Sprintf(`
		INSERT INTO render_artifacts (
			id, owner_id, project_id, job_id, snapshot_digest, profile_id,
			media_asset_id, subtitle_media_asset_id, byte_size, sha256, mime_type, duration_ms, width,
			height, toolchain_version, created_at
		)
		SELECT $1,$2,$3,$4,$5::char(64),$6,$7,$8::uuid,$9,$10::char(64),$11,$12,$13,$14,$15,$16
		FROM jobs j
		JOIN media_assets m ON m.id=$7
		LEFT JOIN media_assets sm ON sm.id=$8::uuid
		WHERE j.id=$4 AND j.owner_id=$2 AND j.project_id=$3 AND j.kind=$17
		  AND j.state='running' AND j.lease_token=$18 AND j.lease_until>now()
		  AND j.cancel_requested_at IS NULL
		  AND j.payload->>'snapshot_digest'=$5::text AND j.payload->>'profile_id'=$6
		  AND m.owner_id=$2 AND m.project_id=$3 AND m.deletion_requested_at IS NULL
		  AND m.kind='video' AND m.origin='system'
		  AND m.byte_size=$9 AND m.sha256=$10::text AND m.mime_type=$11
		  AND (
			$8::uuid IS NULL OR (
				sm.owner_id=$2 AND sm.project_id=$3 AND sm.deletion_requested_at IS NULL
				AND sm.kind='document' AND sm.origin='system' AND sm.mime_type='text/vtt'
				AND sm.metadata->>'source'=$17 AND sm.metadata->>'render_job_id'=$4::text
				AND sm.metadata->>'snapshot_digest'=$5::text AND sm.metadata->>'profile_id'=$6
				AND sm.metadata->>'output_role'='subtitle'
				AND j.payload->>'subtitle_mode'='webvtt'
			)
		  )
		RETURNING %s
	`, renderArtifactFields)
	created, err := scanRenderArtifact(r.pool.QueryRow(ctx, query,
		artifact.ID, artifact.OwnerID, artifact.ProjectID, artifact.JobID,
		artifact.SnapshotDigest, artifact.ProfileID, artifact.MediaAssetID, artifact.SubtitleMediaAssetID,
		artifact.ByteSize, artifact.SHA256, artifact.MimeType, artifact.DurationMS,
		artifact.Width, artifact.Height, artifact.ToolchainVersion, artifact.CreatedAt,
		renderexport.JobKind, leaseToken,
	))
	if err == nil {
		return created, nil
	}
	if errors.Is(err, renderexport.ErrArtifactNotFound) {
		cancelRequested, cancelErr := r.renderCancelRequested(ctx, artifact.JobID)
		if cancelErr != nil {
			return renderexport.RenderArtifact{}, cancelErr
		}
		if cancelRequested {
			return renderexport.RenderArtifact{}, renderexport.ErrRenderCancelFenced
		}
		active, leaseErr := r.renderLeaseActive(ctx, artifact.OwnerID, artifact.ProjectID, artifact.JobID, leaseToken)
		if leaseErr != nil {
			return renderexport.RenderArtifact{}, leaseErr
		}
		if !active {
			return renderexport.RenderArtifact{}, renderexport.ErrStaleRenderLease
		}
		return renderexport.RenderArtifact{}, renderexport.ErrArtifactNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return renderexport.RenderArtifact{}, renderexport.ErrArtifactConflict
	}
	return renderexport.RenderArtifact{}, fmt.Errorf("create render artifact: %w", err)
}

func (r *RenderArtifactRepository) renderLeaseActive(ctx context.Context, ownerID, projectID, jobID, leaseToken uuid.UUID) (bool, error) {
	var active bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM jobs
			WHERE id=$1 AND owner_id=$2 AND project_id=$3 AND kind=$4
			  AND state='running' AND lease_token=$5 AND lease_until>now()
			  AND cancel_requested_at IS NULL
		)
	`, jobID, ownerID, projectID, renderexport.JobKind, leaseToken).Scan(&active); err != nil {
		return false, fmt.Errorf("check render artifact lease: %w", err)
	}
	return active, nil
}

func (r *RenderArtifactRepository) renderCancelRequested(ctx context.Context, jobID uuid.UUID) (bool, error) {
	var requested bool
	if err := r.pool.QueryRow(ctx, `
		SELECT cancel_requested_at IS NOT NULL
		FROM jobs
		WHERE id=$1;
	`, jobID).Scan(&requested); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, renderexport.ErrArtifactNotFound
		}
		return false, fmt.Errorf("check render cancel request: %w", err)
	}
	return requested, nil
}

func (r *RenderArtifactRepository) Get(ctx context.Context, ownerID, projectID, artifactID uuid.UUID) (renderexport.RenderArtifact, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil || artifactID == uuid.Nil {
		return renderexport.RenderArtifact{}, renderexport.ErrInvalidRequest
	}
	query := fmt.Sprintf(`SELECT %s FROM render_artifacts
		WHERE owner_id=$1 AND project_id=$2 AND id=$3`, renderArtifactFields)
	return scanRenderArtifact(r.pool.QueryRow(ctx, query, ownerID, projectID, artifactID))
}

func (r *RenderArtifactRepository) GetByJob(ctx context.Context, ownerID, projectID, jobID uuid.UUID) (renderexport.RenderArtifact, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil || jobID == uuid.Nil {
		return renderexport.RenderArtifact{}, renderexport.ErrInvalidRequest
	}
	query := fmt.Sprintf(`SELECT %s FROM render_artifacts
		WHERE owner_id=$1 AND project_id=$2 AND job_id=$3`, renderArtifactFields)
	return scanRenderArtifact(r.pool.QueryRow(ctx, query, ownerID, projectID, jobID))
}

type renderArtifactRow interface{ Scan(...any) error }

func scanRenderArtifact(row renderArtifactRow) (renderexport.RenderArtifact, error) {
	var artifact renderexport.RenderArtifact
	if err := row.Scan(
		&artifact.ID, &artifact.OwnerID, &artifact.ProjectID, &artifact.JobID,
		&artifact.SnapshotDigest, &artifact.ProfileID, &artifact.MediaAssetID, &artifact.SubtitleMediaAssetID,
		&artifact.ByteSize, &artifact.SHA256, &artifact.MimeType, &artifact.DurationMS,
		&artifact.Width, &artifact.Height, &artifact.ToolchainVersion, &artifact.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return renderexport.RenderArtifact{}, renderexport.ErrArtifactNotFound
		}
		return renderexport.RenderArtifact{}, fmt.Errorf("scan render artifact: %w", err)
	}
	return artifact, nil
}
