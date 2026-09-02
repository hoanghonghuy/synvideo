-- Add capability/voice support to existing text_provider_settings while preserving text data.
-- This migration is backward-compatible: existing text settings remain usable without re-encryption.

-- Rename table to reflect multi-capability scope
ALTER TABLE text_provider_settings RENAME TO provider_settings;

-- Rename constraints/indexes to match new table name
DO $$
BEGIN
	IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'text_provider_settings_provider_id_length') THEN
		ALTER TABLE provider_settings RENAME CONSTRAINT text_provider_settings_provider_id_length TO provider_settings_provider_id_length;
	END IF;
	IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'text_provider_settings_revision_positive') THEN
		ALTER TABLE provider_settings RENAME CONSTRAINT text_provider_settings_revision_positive TO provider_settings_revision_positive;
	END IF;
	IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'text_provider_settings_key_version_nonempty') THEN
		ALTER TABLE provider_settings RENAME CONSTRAINT text_provider_settings_key_version_nonempty TO provider_settings_key_version_nonempty;
	END IF;
END $$;

ALTER INDEX text_provider_settings_owner_id_idx RENAME TO provider_settings_owner_id_idx;

-- Add new columns for capability-aware configuration
-- Migrate existing enabled_model_ids data to enabled_text_model_ids before adding new column
ALTER TABLE provider_settings
	ADD COLUMN enabled_text_model_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
	ADD COLUMN enabled_image_model_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
	ADD COLUMN enabled_voice_ids jsonb NOT NULL DEFAULT '[]'::jsonb;

-- Copy existing text model selections to the new text column
UPDATE provider_settings
SET enabled_text_model_ids = enabled_model_ids
WHERE enabled_text_model_ids = '[]'::jsonb
  AND jsonb_array_length(enabled_model_ids) > 0;

-- Drop old column after data migration
ALTER TABLE provider_settings DROP COLUMN enabled_model_ids;
