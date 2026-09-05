CREATE TABLE caption_documents (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    scene_plan_version integer NOT NULL CHECK (scene_plan_version > 0),
    scene_key text NOT NULL,
    revision integer NOT NULL CHECK (revision > 0),
    source_binding_id uuid NOT NULL REFERENCES scene_narration_bindings(id),
    source_asset_id uuid NOT NULL REFERENCES media_assets(id),
    source_duration_ms bigint NOT NULL CHECK (source_duration_ms > 0),
    segments jsonb NOT NULL,
    style jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT caption_documents_scene_revision_unique UNIQUE (owner_id, project_id, scene_plan_version, scene_key, revision),
    CONSTRAINT caption_documents_segments_array CHECK (jsonb_typeof(segments) = 'array'),
    CONSTRAINT caption_documents_style_object CHECK (jsonb_typeof(style) = 'object')
);

CREATE INDEX caption_documents_latest_idx
    ON caption_documents (owner_id, project_id, scene_plan_version, scene_key, revision DESC);
