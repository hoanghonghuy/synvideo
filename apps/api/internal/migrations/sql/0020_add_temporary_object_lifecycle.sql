CREATE TABLE temporary_objects (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL,
    project_id uuid NOT NULL,
    job_id uuid NOT NULL,
    object_key text NOT NULL,
    state text NOT NULL DEFAULT 'recoverable' CHECK (state IN ('recoverable', 'cleanup_pending', 'removed')),
    next_cleanup_at timestamptz NOT NULL DEFAULT now(),
    claim_token uuid,
    claim_until timestamptz,
    cleanup_attempts integer NOT NULL DEFAULT 0 CHECK (cleanup_attempts >= 0),
    last_error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    removed_at timestamptz,
    UNIQUE (project_id, job_id, object_key)
);

CREATE INDEX temporary_objects_cleanup_idx
    ON temporary_objects (next_cleanup_at, created_at)
    WHERE removed_at IS NULL;

CREATE INDEX temporary_objects_job_idx
    ON temporary_objects (job_id, project_id)
    WHERE removed_at IS NULL;
