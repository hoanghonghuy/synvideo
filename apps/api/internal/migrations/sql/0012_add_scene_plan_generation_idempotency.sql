ALTER TABLE scene_plans
	ADD COLUMN source_generation_job_id uuid REFERENCES jobs(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX scene_plans_project_source_job_uniq
	ON scene_plans (project_id, source_generation_job_id)
	WHERE source_generation_job_id IS NOT NULL;
