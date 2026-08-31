CREATE TABLE projects (
	id uuid PRIMARY KEY,
	owner_id uuid NOT NULL,
	title text NOT NULL,
	description text NOT NULL DEFAULT '',
	content_format text NOT NULL,
	aspect_ratio text NOT NULL,
	target_duration_seconds integer,
	locale text NOT NULL DEFAULT 'vi',
	status text NOT NULL DEFAULT 'active',
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT projects_title_length CHECK (char_length(btrim(title)) BETWEEN 1 AND 160),
	CONSTRAINT projects_description_length CHECK (char_length(description) <= 5000),
	CONSTRAINT projects_content_format CHECK (content_format IN ('short', 'long', 'flexible')),
	CONSTRAINT projects_aspect_ratio CHECK (aspect_ratio IN ('16:9', '9:16', '1:1', '4:5')),
	CONSTRAINT projects_target_duration_seconds CHECK (target_duration_seconds IS NULL OR target_duration_seconds BETWEEN 1 AND 43200),
	CONSTRAINT projects_locale CHECK (locale IN ('vi', 'en')),
	CONSTRAINT projects_status CHECK (status IN ('active', 'archived'))
);

CREATE INDEX projects_owner_updated_id_idx ON projects (owner_id, updated_at DESC, id DESC);
