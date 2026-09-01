ALTER TABLE scripts
	ADD COLUMN source_generation_job_id uuid REFERENCES jobs(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX scripts_project_source_job_uniq
	ON scripts (project_id, source_generation_job_id)
	WHERE source_generation_job_id IS NOT NULL;
