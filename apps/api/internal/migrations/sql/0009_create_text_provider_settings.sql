CREATE TABLE text_provider_settings (
	owner_id uuid NOT NULL,
	provider_id text NOT NULL,
	revision integer NOT NULL,
	enabled boolean NOT NULL DEFAULT true,
	enabled_model_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
	api_key_ciphertext bytea NOT NULL,
	api_key_nonce bytea NOT NULL,
	key_version text NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (owner_id, provider_id),
	CONSTRAINT text_provider_settings_provider_id_length CHECK (char_length(btrim(provider_id)) BETWEEN 1 AND 100),
	CONSTRAINT text_provider_settings_revision_positive CHECK (revision > 0),
	CONSTRAINT text_provider_settings_key_version_nonempty CHECK (char_length(btrim(key_version)) > 0)
);

CREATE INDEX text_provider_settings_owner_id_idx ON text_provider_settings (owner_id);
