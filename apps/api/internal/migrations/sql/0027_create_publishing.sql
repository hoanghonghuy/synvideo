CREATE TABLE publishing_channel_connections (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL,
    provider text NOT NULL CHECK (provider = 'youtube'),
    remote_channel_id text NOT NULL,
    display_name text NOT NULL,
    state text NOT NULL CHECK (state IN ('connected','reconnect_required','revoked')),
    can_upload boolean NOT NULL DEFAULT false,
    can_publish boolean NOT NULL DEFAULT false,
    can_schedule boolean NOT NULL DEFAULT false,
    encrypted_refresh_token bytea NOT NULL,
    token_key_id text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (length(btrim(remote_channel_id)) > 0),
    CHECK (length(btrim(display_name)) > 0),
    CHECK (length(btrim(token_key_id)) > 0),
    CHECK (octet_length(encrypted_refresh_token) > 0),
    CHECK (updated_at >= created_at),
    CHECK (state = 'connected' OR (NOT can_upload AND NOT can_publish AND NOT can_schedule)),
    CHECK (NOT can_schedule OR can_publish),
    UNIQUE (owner_id, provider, remote_channel_id)
);

CREATE INDEX publishing_connections_owner_updated_idx
    ON publishing_channel_connections (owner_id, updated_at DESC, id DESC);

CREATE TABLE publishing_attempts (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL,
    project_id uuid NOT NULL,
    connection_id uuid NOT NULL,
    render_artifact_id uuid NOT NULL,
    request_id uuid NOT NULL,
    provider text NOT NULL CHECK (provider = 'youtube'),
    state text NOT NULL CHECK (state IN ('queued','uploading','upload_accepted','processing','private','scheduled','public','reconnect_required','retryable_failure','rejected')),
    remote_video_id text,
    resumable_session_uri text,
    uploaded_bytes bigint NOT NULL DEFAULT 0 CHECK (uploaded_bytes >= 0),
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    scheduled_at timestamptz,
    last_error_code text,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT publishing_attempt_project_fk FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CONSTRAINT publishing_attempt_connection_fk FOREIGN KEY (connection_id) REFERENCES publishing_channel_connections(id) ON DELETE RESTRICT,
    CONSTRAINT publishing_attempt_artifact_fk FOREIGN KEY (render_artifact_id) REFERENCES render_artifacts(id) ON DELETE RESTRICT,
    CHECK (length(btrim(title)) > 0),
    CHECK (updated_at >= created_at),
    CHECK (remote_video_id IS NULL OR length(btrim(remote_video_id)) > 0),
    CHECK (resumable_session_uri IS NULL OR length(btrim(resumable_session_uri)) > 0),
    CHECK (last_error_code IS NULL OR length(btrim(last_error_code)) > 0),
    CHECK (state <> 'scheduled' OR (scheduled_at IS NOT NULL AND remote_video_id IS NOT NULL)),
    UNIQUE (owner_id, project_id, connection_id, render_artifact_id, request_id)
);

CREATE INDEX publishing_attempts_owner_project_updated_idx
    ON publishing_attempts (owner_id, project_id, updated_at DESC, id DESC);
CREATE INDEX publishing_attempts_recovery_idx
    ON publishing_attempts (state, updated_at)
    WHERE state IN ('uploading','upload_accepted','processing','retryable_failure');
