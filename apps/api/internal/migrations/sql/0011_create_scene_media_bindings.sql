ALTER TABLE projects
	ADD CONSTRAINT projects_owner_id_id_key UNIQUE (owner_id, id);

ALTER TABLE media_assets
	ADD CONSTRAINT media_assets_owner_project_id_key UNIQUE (owner_id, project_id, id);

CREATE TABLE scene_media_bindings (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	owner_id uuid NOT NULL,
	project_id uuid NOT NULL,
	scene_plan_version integer NOT NULL,
	scene_key text NOT NULL,
	role text NOT NULL DEFAULT 'primary_visual',
	binding_version integer NOT NULL,
	asset_id uuid NOT NULL,
	status text NOT NULL DEFAULT 'active',
	created_at timestamptz NOT NULL DEFAULT now(),
	superseded_at timestamptz,
	CONSTRAINT scene_media_bindings_project_fk
		FOREIGN KEY (owner_id, project_id) REFERENCES projects(owner_id, id) ON DELETE CASCADE,
	CONSTRAINT scene_media_bindings_plan_fk
		FOREIGN KEY (project_id, scene_plan_version) REFERENCES scene_plans(project_id, version) ON DELETE CASCADE,
	CONSTRAINT scene_media_bindings_asset_fk
		FOREIGN KEY (owner_id, project_id, asset_id) REFERENCES media_assets(owner_id, project_id, id) ON DELETE RESTRICT,
	CONSTRAINT scene_media_bindings_plan_version_positive CHECK (scene_plan_version >= 1),
	CONSTRAINT scene_media_bindings_scene_key_valid CHECK (char_length(btrim(scene_key)) BETWEEN 1 AND 64),
	CONSTRAINT scene_media_bindings_role_allowed CHECK (role = 'primary_visual'),
	CONSTRAINT scene_media_bindings_version_positive CHECK (binding_version >= 1),
	CONSTRAINT scene_media_bindings_status_allowed CHECK (status IN ('active', 'superseded')),
	CONSTRAINT scene_media_bindings_superseded_timestamp CHECK (
		(status = 'active' AND superseded_at IS NULL)
		OR (status = 'superseded' AND superseded_at IS NOT NULL)
	)
);

CREATE UNIQUE INDEX scene_media_bindings_identity_version_idx
	ON scene_media_bindings (owner_id, project_id, scene_plan_version, scene_key, role, binding_version);

CREATE UNIQUE INDEX scene_media_bindings_one_active_idx
	ON scene_media_bindings (owner_id, project_id, scene_plan_version, scene_key, role)
	WHERE status = 'active';

CREATE INDEX scene_media_bindings_scene_history_idx
	ON scene_media_bindings (owner_id, project_id, scene_plan_version, scene_key, role, binding_version DESC);
