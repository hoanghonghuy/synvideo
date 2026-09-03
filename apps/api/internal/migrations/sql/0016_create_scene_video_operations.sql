ALTER TABLE provider_settings
	ADD COLUMN enabled_video_model_ids jsonb NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE scene_video_operations (
	job_id uuid PRIMARY KEY,
	owner_id uuid NOT NULL,
	project_id uuid NOT NULL,
	state text NOT NULL,
	external_operation_id text,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT scene_video_operations_job_fk
		FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE,
	CONSTRAINT scene_video_operations_project_fk
		FOREIGN KEY (owner_id, project_id) REFERENCES projects(owner_id, id) ON DELETE CASCADE,
	CONSTRAINT scene_video_operations_state_allowed
		CHECK (state IN ('submitted', 'ambiguous')),
	CONSTRAINT scene_video_operations_external_id_state
		CHECK (
			(state = 'submitted' AND external_operation_id IS NOT NULL AND char_length(btrim(external_operation_id)) BETWEEN 1 AND 500)
			OR (state = 'ambiguous' AND external_operation_id IS NULL)
		)
);

CREATE INDEX scene_video_operations_owner_project_idx
	ON scene_video_operations (owner_id, project_id, created_at DESC);
