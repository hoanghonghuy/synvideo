CREATE TABLE jobs (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	owner_id uuid NOT NULL,
	project_id uuid REFERENCES projects(id) ON DELETE CASCADE,
	kind text NOT NULL,
	dedupe_key text,
	state text NOT NULL DEFAULT 'queued',
	attempt integer NOT NULL DEFAULT 0,
	max_attempts integer NOT NULL DEFAULT 3,
	available_at timestamptz NOT NULL DEFAULT now(),
	lease_token uuid,
	lease_until timestamptz,
	payload jsonb NOT NULL DEFAULT '{}',
	result jsonb,
	error_code text,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	started_at timestamptz,
	finished_at timestamptz,
	CONSTRAINT jobs_kind_length CHECK (char_length(btrim(kind)) BETWEEN 1 AND 100),
	CONSTRAINT jobs_dedupe_key_length CHECK (dedupe_key IS NULL OR (char_length(btrim(dedupe_key)) BETWEEN 1 AND 200)),
	CONSTRAINT jobs_state_allowed CHECK (state IN ('queued', 'running', 'succeeded', 'failed')),
	CONSTRAINT jobs_attempt_nonnegative CHECK (attempt >= 0),
	CONSTRAINT jobs_max_attempts_positive CHECK (max_attempts >= 1)
);

CREATE UNIQUE INDEX jobs_owner_kind_dedupe_key_idx ON jobs (owner_id, kind, dedupe_key) WHERE (dedupe_key IS NOT NULL);
CREATE INDEX jobs_claim_queued_idx ON jobs (kind, available_at ASC, created_at ASC) WHERE (state = 'queued');
CREATE INDEX jobs_claim_running_idx ON jobs (kind, lease_until ASC, created_at ASC) WHERE (state = 'running');
CREATE INDEX jobs_owner_id_created_at_idx ON jobs (owner_id, created_at DESC);
CREATE INDEX jobs_project_id_created_at_idx ON jobs (project_id, created_at DESC) WHERE (project_id IS NOT NULL);
