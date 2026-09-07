CREATE UNIQUE INDEX media_assets_render_job_output_uq
    ON media_assets (owner_id, project_id, (metadata->>'render_job_id'))
    WHERE kind = 'video'
      AND origin = 'system'
      AND deletion_requested_at IS NULL
      AND metadata->>'source' = 'render_export_v1'
      AND metadata ? 'render_job_id';
