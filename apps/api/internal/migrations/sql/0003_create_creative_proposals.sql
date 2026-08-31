CREATE TABLE creative_proposals (
	project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	version integer NOT NULL,
	revision integer NOT NULL DEFAULT 1,
	status text NOT NULL DEFAULT 'draft',
	source_brief_revision integer NOT NULL,
	title_options text[] NOT NULL,
	hook_options text[] NOT NULL,
	audience_summary text NOT NULL,
	objective_summary text NOT NULL,
	narrative_angle text NOT NULL,
	estimated_duration_seconds integer,
	format_rationale text NOT NULL DEFAULT '',
	structure jsonb NOT NULL,
	visual_direction text NOT NULL DEFAULT '',
	voice_direction text NOT NULL DEFAULT '',
	music_direction text NOT NULL DEFAULT '',
	caption_direction text NOT NULL DEFAULT '',
	call_to_action text NOT NULL DEFAULT '',
	research_gaps text[] NOT NULL DEFAULT '{}',
	warnings text[] NOT NULL DEFAULT '{}',
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	approved_at timestamptz,
	PRIMARY KEY (project_id, version),
	CONSTRAINT creative_proposals_version_positive CHECK (version >= 1),
	CONSTRAINT creative_proposals_revision_positive CHECK (revision >= 1),
	CONSTRAINT creative_proposals_status_allowed CHECK (status IN ('draft', 'approved', 'superseded')),
	CONSTRAINT creative_proposals_source_brief_revision_positive CHECK (source_brief_revision >= 1),
	CONSTRAINT creative_proposals_title_options_count CHECK (
		cardinality(title_options) BETWEEN 1 AND 5
		AND synvideo_text_array_items_between(title_options, 1, 300)
	),
	CONSTRAINT creative_proposals_hook_options_count CHECK (
		cardinality(hook_options) BETWEEN 1 AND 5
		AND synvideo_text_array_items_between(hook_options, 1, 1000)
	),
	CONSTRAINT creative_proposals_audience_summary_length CHECK (char_length(btrim(audience_summary)) BETWEEN 1 AND 2000),
	CONSTRAINT creative_proposals_objective_summary_length CHECK (char_length(btrim(objective_summary)) BETWEEN 1 AND 2000),
	CONSTRAINT creative_proposals_narrative_angle_length CHECK (char_length(btrim(narrative_angle)) BETWEEN 1 AND 4000),
	CONSTRAINT creative_proposals_estimated_duration_seconds_range CHECK (
		estimated_duration_seconds IS NULL
		OR estimated_duration_seconds BETWEEN 1 AND 43200
	),
	CONSTRAINT creative_proposals_format_rationale_length CHECK (char_length(format_rationale) <= 2000),
	CONSTRAINT creative_proposals_visual_direction_length CHECK (char_length(visual_direction) <= 5000),
	CONSTRAINT creative_proposals_voice_direction_length CHECK (char_length(voice_direction) <= 3000),
	CONSTRAINT creative_proposals_music_direction_length CHECK (char_length(music_direction) <= 3000),
	CONSTRAINT creative_proposals_caption_direction_length CHECK (char_length(caption_direction) <= 3000),
	CONSTRAINT creative_proposals_call_to_action_length CHECK (char_length(call_to_action) <= 2000),
	CONSTRAINT creative_proposals_research_gaps_items CHECK (
		cardinality(research_gaps) <= 20
		AND synvideo_text_array_items_between(research_gaps, 1, 1000)
	),
	CONSTRAINT creative_proposals_warnings_items CHECK (
		cardinality(warnings) <= 20
		AND synvideo_text_array_items_between(warnings, 1, 1000)
	)
);

CREATE UNIQUE INDEX creative_proposals_one_active_draft_idx ON creative_proposals (project_id) WHERE (status = 'draft');
CREATE INDEX creative_proposals_project_version_idx ON creative_proposals (project_id, version DESC);
CREATE INDEX creative_proposals_updated_at_idx ON creative_proposals (updated_at DESC);
