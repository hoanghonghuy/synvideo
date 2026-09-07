CREATE TABLE render_artifacts (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL,
    project_id uuid NOT NULL,
    job_id uuid NOT NULL,
    snapshot_digest char(64) NOT NULL,
    profile_id text NOT NULL,
    media_asset_id uuid NOT NULL,
    byte_size bigint NOT NULL CHECK (byte_size > 0),
    sha256 char(64) NOT NULL,
    mime_type text NOT NULL CHECK (mime_type = 'video/mp4'),
    duration_ms bigint NOT NULL CHECK (duration_ms > 0),
    width integer NOT NULL CHECK (width > 0),
    height integer NOT NULL CHECK (height > 0),
    toolchain_version text NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT render_artifacts_project_fk FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CONSTRAINT render_artifacts_job_fk FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE RESTRICT,
    CONSTRAINT render_artifacts_media_asset_fk FOREIGN KEY (media_asset_id) REFERENCES media_assets(id) ON DELETE RESTRICT,
    CONSTRAINT render_artifacts_snapshot_digest_format CHECK (snapshot_digest ~ '^[0-9a-f]{64}$'),
    CONSTRAINT render_artifacts_sha256_format CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT render_artifacts_toolchain_nonempty CHECK (length(btrim(toolchain_version)) > 0),
    CONSTRAINT render_artifacts_profile_nonempty CHECK (length(btrim(profile_id)) > 0),
    UNIQUE (owner_id, project_id, job_id),
    UNIQUE (media_asset_id)
);

CREATE INDEX render_artifacts_owner_project_created_idx
    ON render_artifacts (owner_id, project_id, created_at DESC, id DESC);
