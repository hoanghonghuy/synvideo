// Command e2e-render-fixture seeds only the approved upstream lineage needed by
// the isolated render acceptance test. It is intentionally gated by the E2E
// run marker and is never wired into the production HTTP/runtime path.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if os.Getenv("SYNVIDEO_E2E_RUN_ID") == "" {
		fatalf("SYNVIDEO_E2E_RUN_ID is required")
	}
	if len(os.Args) != 2 {
		fatalf("usage: e2e-render-fixture <project-id>")
	}
	projectID, err := uuid.Parse(os.Args[1])
	if err != nil || projectID == uuid.Nil {
		fatalf("invalid project id")
	}
	databaseURL := os.Getenv("SYNVIDEO_DATABASE_URL")
	if databaseURL == "" {
		fatalf("SYNVIDEO_DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fatalf("database pool: %v", err)
	}
	defer pool.Close()

	var ownerID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT owner_id FROM projects WHERE id = $1`, projectID).Scan(&ownerID); err != nil {
		fatalf("project lookup: %v", err)
	}
	if expected := os.Getenv("SYNVIDEO_LOCAL_ACTOR_ID"); expected != "" && ownerID.String() != expected {
		fatalf("project owner does not match isolated E2E actor")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		fatalf("begin fixture transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	structure, _ := json.Marshal([]map[string]string{{"key": "intro", "purpose": "render acceptance"}})
	sections, _ := json.Marshal([]map[string]string{{"key": "intro", "body": "SynVideo renders this accepted scene."}})
	scenes, _ := json.Marshal([]map[string]any{{
		"key": "intro", "script_section_key": "intro", "narration": "SynVideo renders this accepted scene.",
		"visual_instruction": "Use the uploaded acceptance frame.", "planned_source_type": "upload",
		"expected_duration_seconds": 2,
	}})

	if _, err := tx.Exec(ctx, `
		INSERT INTO creative_proposals (
			project_id, version, revision, status, source_brief_revision,
			title_options, hook_options, audience_summary, objective_summary,
			narrative_angle, structure, approved_at
		) VALUES ($1, 1, 1, 'approved', 1, ARRAY['Render acceptance'], ARRAY['Render acceptance'],
			'Local acceptance audience', 'Verify real local MP4 export', 'Single accepted scene', $2::jsonb, now())`, projectID, structure); err != nil {
		fatalf("seed approved proposal: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO scripts (
			project_id, version, revision, status, source_proposal_version,
			content_locale, sections, estimated_duration_seconds, approved_at
		) VALUES ($1, 1, 1, 'approved', 1, 'en', $2::jsonb, 2, now())`, projectID, sections); err != nil {
		fatalf("seed approved script: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO scene_plans (
			project_id, version, revision, status, source_script_version,
			source_proposal_version, content_locale, scenes, approved_at
		) VALUES ($1, 1, 1, 'approved', 1, 1, 'en', $2::jsonb, now())`, projectID, scenes); err != nil {
		fatalf("seed approved scene plan: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		fatalf("commit fixture: %v", err)
	}
	fmt.Println(projectID.String())
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
