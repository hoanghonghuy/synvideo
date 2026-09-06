CREATE UNIQUE INDEX media_assets_stock_identity_unique
	ON media_assets (
		owner_id,
		project_id,
		(metadata ->> 'stock_provider'),
		(metadata ->> 'stock_result_id'),
		kind
	)
	WHERE origin = 'stock'
	  AND deletion_requested_at IS NULL
	  AND coalesce(metadata ->> 'stock_provider', '') <> ''
	  AND coalesce(metadata ->> 'stock_result_id', '') <> '';
