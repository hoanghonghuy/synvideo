ALTER TABLE render_artifacts
    ADD COLUMN subtitle_media_asset_id uuid NULL;

ALTER TABLE render_artifacts
    ADD CONSTRAINT render_artifacts_subtitle_media_asset_fk
    FOREIGN KEY (subtitle_media_asset_id) REFERENCES media_assets(id) ON DELETE RESTRICT;

CREATE UNIQUE INDEX render_artifacts_subtitle_media_asset_unique
    ON render_artifacts (subtitle_media_asset_id)
    WHERE subtitle_media_asset_id IS NOT NULL;
