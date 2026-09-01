CREATE TABLE scene_plans (
	project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	version integer NOT NULL,
	revision integer NOT NULL DEFAULT 1,
	status text NOT NULL DEFAULT 'draft',
	source_script_version integer NOT NULL,
	source_proposal_version integer NOT NULL,
	content_locale text NOT NULL,
	scenes jsonb NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	approved_at timestamptz,
	PRIMARY KEY (project_id, version),
	FOREIGN KEY (project_id, source_script_version) REFERENCES scripts(project_id, version) ON DELETE RESTRICT,
	FOREIGN KEY (project_id, source_proposal_version) REFERENCES creative_proposals(project_id, version) ON DELETE RESTRICT,
	CONSTRAINT scene_plans_version_positive CHECK (version >= 1),
	CONSTRAINT scene_plans_revision_positive CHECK (revision >= 1),
	CONSTRAINT scene_plans_status_allowed CHECK (status IN ('draft', 'approved', 'superseded')),
	CONSTRAINT scene_plans_source_script_version_positive CHECK (source_script_version >= 1),
	CONSTRAINT scene_plans_source_proposal_version_positive CHECK (source_proposal_version >= 1),
	CONSTRAINT scene_plans_content_locale_allowed CHECK (content_locale IN ('vi', 'en')),
	CONSTRAINT scene_plans_scenes_array CHECK (
		jsonb_typeof(scenes) = 'array' AND jsonb_array_length(scenes) BETWEEN 1 AND 500
	)
);

CREATE UNIQUE INDEX scene_plans_one_active_draft_idx ON scene_plans (project_id) WHERE (status = 'draft');
CREATE INDEX scene_plans_project_version_idx ON scene_plans (project_id, version DESC);
CREATE INDEX scene_plans_updated_at_idx ON scene_plans (updated_at DESC);
