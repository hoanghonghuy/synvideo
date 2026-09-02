ALTER TABLE media_assets
	ADD COLUMN deletion_requested_at timestamptz;

CREATE INDEX media_assets_pending_deletion_idx
	ON media_assets (deletion_requested_at)
	WHERE deletion_requested_at IS NOT NULL;
