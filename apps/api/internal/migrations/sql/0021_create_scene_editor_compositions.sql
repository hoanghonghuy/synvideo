CREATE TABLE scene_editor_compositions (
    id uuid NOT NULL,
    owner_id uuid NOT NULL,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    revision integer NOT NULL CHECK (revision > 0),
    scene_plan_version integer NOT NULL CHECK (scene_plan_version > 0),
    document jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT scene_editor_compositions_primary PRIMARY KEY (id, revision),
    CONSTRAINT scene_editor_compositions_project_revision_unique UNIQUE (owner_id, project_id, revision),
    CONSTRAINT scene_editor_compositions_document_object CHECK (jsonb_typeof(document) = 'object')
);

CREATE INDEX scene_editor_compositions_latest_idx
    ON scene_editor_compositions (owner_id, project_id, revision DESC);

CREATE TABLE scene_editor_snapshots (
    composition_id uuid NOT NULL,
    revision integer NOT NULL CHECK (revision > 0),
    owner_id uuid NOT NULL,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    digest text NOT NULL,
    snapshot jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT scene_editor_snapshots_primary PRIMARY KEY (composition_id, revision, digest),
    CONSTRAINT scene_editor_snapshots_digest_unique UNIQUE (owner_id, project_id, digest),
    CONSTRAINT scene_editor_snapshots_payload_object CHECK (jsonb_typeof(snapshot) = 'object'),
    CONSTRAINT scene_editor_snapshots_composition_revision_fk
        FOREIGN KEY (composition_id, revision)
        REFERENCES scene_editor_compositions(id, revision)
        ON DELETE RESTRICT
);

CREATE INDEX scene_editor_snapshots_project_idx
    ON scene_editor_snapshots (owner_id, project_id, revision DESC);
