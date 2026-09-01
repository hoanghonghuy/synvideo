CREATE TABLE media_assets (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	owner_id uuid NOT NULL,
	project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	kind text NOT NULL,
	origin text NOT NULL,
	object_key text NOT NULL UNIQUE,
	mime_type text NOT NULL,
	byte_size bigint NOT NULL,
	sha256 text NOT NULL,
	original_filename text NOT NULL DEFAULT '',
	metadata jsonb NOT NULL DEFAULT '{}',
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT media_assets_kind_allowed CHECK (kind IN ('image', 'video', 'audio', 'document', 'other')),
	CONSTRAINT media_assets_origin_allowed CHECK (origin IN ('upload', 'creator_media', 'stock', 'generated_image', 'generated_video', 'generated_audio', 'system')),
	CONSTRAINT media_assets_object_key_safe CHECK (
		object_key LIKE 'projects/%/assets/%'
		AND position('..' IN object_key) = 0
		AND left(object_key, 1) <> '/'
	),
	CONSTRAINT media_assets_mime_type_length CHECK (char_length(btrim(mime_type)) BETWEEN 1 AND 255),
	CONSTRAINT media_assets_byte_size_nonnegative CHECK (byte_size >= 0),
	CONSTRAINT media_assets_sha256_format CHECK (sha256 ~ '^[0-9a-f]{64}$'),
	CONSTRAINT media_assets_original_filename_length CHECK (char_length(original_filename) <= 500),
	CONSTRAINT media_assets_metadata_object CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX media_assets_owner_project_created_idx
	ON media_assets (owner_id, project_id, created_at DESC, id DESC);
CREATE INDEX media_assets_project_created_idx
	ON media_assets (project_id, created_at DESC, id DESC);
