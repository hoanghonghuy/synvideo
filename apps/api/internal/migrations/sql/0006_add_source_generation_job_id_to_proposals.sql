ALTER TABLE creative_proposals
	ADD COLUMN source_generation_job_id uuid REFERENCES jobs(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX creative_proposals_project_source_job_uniq
	ON creative_proposals (project_id, source_generation_job_id)
	WHERE source_generation_job_id IS NOT NULL;
