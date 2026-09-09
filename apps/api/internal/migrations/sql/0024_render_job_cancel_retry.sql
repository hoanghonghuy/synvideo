ALTER TABLE jobs DROP CONSTRAINT jobs_state_allowed;
ALTER TABLE jobs ADD CONSTRAINT jobs_state_allowed CHECK (state IN ('queued', 'running', 'succeeded', 'failed', 'cancelled'));

ALTER TABLE jobs ADD COLUMN cancel_requested_at timestamptz;

CREATE INDEX jobs_project_kind_created_id_idx ON jobs (project_id, kind, created_at DESC, id DESC)
	WHERE project_id IS NOT NULL;
