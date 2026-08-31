CREATE TABLE scripts (
	project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	version integer NOT NULL,
	revision integer NOT NULL DEFAULT 1,
	status text NOT NULL DEFAULT 'draft',
	source_proposal_version integer NOT NULL,
	content_locale text NOT NULL,
	sections jsonb NOT NULL,
	estimated_duration_seconds integer,
	notes text NOT NULL DEFAULT '',
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	approved_at timestamptz,
	PRIMARY KEY (project_id, version),
	FOREIGN KEY (project_id, source_proposal_version) REFERENCES creative_proposals(project_id, version) ON DELETE RESTRICT,
	CONSTRAINT scripts_version_positive CHECK (version >= 1),
	CONSTRAINT scripts_revision_positive CHECK (revision >= 1),
	CONSTRAINT scripts_status_allowed CHECK (status IN ('draft', 'approved', 'superseded')),
	CONSTRAINT scripts_source_proposal_version_positive CHECK (source_proposal_version >= 1),
	CONSTRAINT scripts_content_locale_allowed CHECK (content_locale IN ('vi', 'en', 'es', 'zh', 'ja')),
	CONSTRAINT scripts_estimated_duration_seconds_range CHECK (
		estimated_duration_seconds IS NULL
		OR estimated_duration_seconds BETWEEN 1 AND 43200
	),
	CONSTRAINT scripts_notes_length CHECK (char_length(notes) <= 10000)
);

CREATE UNIQUE INDEX scripts_one_active_draft_idx ON scripts (project_id) WHERE (status = 'draft');
CREATE INDEX scripts_project_version_idx ON scripts (project_id, version DESC);
CREATE INDEX scripts_updated_at_idx ON scripts (updated_at DESC);
