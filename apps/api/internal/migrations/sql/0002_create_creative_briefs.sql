CREATE FUNCTION synvideo_text_array_unique(items text[])
RETURNS boolean
LANGUAGE sql
IMMUTABLE
AS $$
	SELECT cardinality(items) = COUNT(DISTINCT item)
	FROM unnest(items) AS item
$$;

CREATE FUNCTION synvideo_text_array_items_between(items text[], min_length integer, max_length integer)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
AS $$
	SELECT COALESCE(bool_and(char_length(btrim(item)) BETWEEN min_length AND max_length), true)
	FROM unnest(items) AS item
$$;

CREATE TABLE creative_briefs (
	project_id uuid PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
	revision integer NOT NULL DEFAULT 1,
	source_text text NOT NULL,
	target_audience text NOT NULL DEFAULT '',
	objective text NOT NULL DEFAULT '',
	desired_style text NOT NULL DEFAULT '',
	tone text NOT NULL DEFAULT '',
	distribution_targets text[] NOT NULL DEFAULT '{}',
	call_to_action text NOT NULL DEFAULT '',
	must_include text[] NOT NULL DEFAULT '{}',
	must_avoid text[] NOT NULL DEFAULT '{}',
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT creative_briefs_revision_positive CHECK (revision >= 1),
	CONSTRAINT creative_briefs_source_text_length CHECK (char_length(btrim(source_text)) BETWEEN 1 AND 20000),
	CONSTRAINT creative_briefs_target_audience_length CHECK (char_length(target_audience) <= 2000),
	CONSTRAINT creative_briefs_objective_length CHECK (char_length(objective) <= 2000),
	CONSTRAINT creative_briefs_desired_style_length CHECK (char_length(desired_style) <= 2000),
	CONSTRAINT creative_briefs_tone_length CHECK (char_length(tone) <= 500),
	CONSTRAINT creative_briefs_distribution_targets_allowed CHECK (
		cardinality(distribution_targets) <= 8
		AND distribution_targets <@ ARRAY['youtube', 'tiktok', 'instagram', 'other']::text[]
		AND synvideo_text_array_unique(distribution_targets)
	),
	CONSTRAINT creative_briefs_call_to_action_length CHECK (char_length(call_to_action) <= 2000),
	CONSTRAINT creative_briefs_must_include_items CHECK (
		cardinality(must_include) <= 20
		AND synvideo_text_array_items_between(must_include, 1, 500)
	),
	CONSTRAINT creative_briefs_must_avoid_items CHECK (
		cardinality(must_avoid) <= 20
		AND synvideo_text_array_items_between(must_avoid, 1, 500)
	)
);

CREATE INDEX creative_briefs_updated_at_idx ON creative_briefs (updated_at DESC);
