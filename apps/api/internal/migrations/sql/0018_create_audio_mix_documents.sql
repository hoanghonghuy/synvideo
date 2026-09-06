CREATE TABLE audio_mix_documents (
    id uuid NOT NULL,
    owner_id uuid NOT NULL,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    revision integer NOT NULL CHECK (revision > 0),
    scene_plan_version integer NOT NULL CHECK (scene_plan_version > 0),
    music_asset_id uuid NOT NULL REFERENCES media_assets(id),
    music_duration_ms bigint NOT NULL CHECK (music_duration_ms > 0),
    narration_lineage_id uuid NOT NULL,
    narration_duration_ms bigint NOT NULL CHECK (narration_duration_ms > 0),
    config jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT audio_mix_documents_primary PRIMARY KEY (id, revision),
    CONSTRAINT audio_mix_documents_project_revision_unique UNIQUE (owner_id, project_id, revision),
    CONSTRAINT audio_mix_documents_config_object CHECK (jsonb_typeof(config) = 'object')
);

CREATE INDEX audio_mix_documents_latest_idx
    ON audio_mix_documents (owner_id, project_id, revision DESC);
