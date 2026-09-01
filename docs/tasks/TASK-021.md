# TASK-021 — Scene Plan durable generation + API integration

Status: CHANGES_REQUESTED
Milestone: F1 Creative Workflow
Wave: WAVE-F1-I
Branch: `feature/TASK-021-scene-plan-generation-integration`
Base: `develop`
PR: #50
Review head: `3acbd7d470d9ba6971489d15e6725c4ab1f91ccb`
Logical TL review: `5080837369`
CI: #236 green on reviewed head
Issue: #39
Depends on: TASK-014, TASK-015, TASK-017, TASK-018 accepted; frozen `SCENE_PLAN_JOB_V1`.

## Goal
Make Stage 7 Scene Plan a real durable backend workflow by integrating accepted Scene Plan generation/persistence with generic jobs, owner-scoped live text runtime, resource APIs, DB exactly-once persistence and runtime composition.

## Frozen contract
`docs/contracts/SCENE_PLAN_JOB_V1.md`.

## Primary ownership
- `apps/api/internal/sceneplangenerationjob/**` or cohesive equivalent;
- minimal Scene Plan generation-idempotency extension;
- focused `postgres/scene_plan_repository.go` integration tests;
- migration exactly `0012_add_scene_plan_generation_idempotency.sql`;
- Scene Plan resource + generation HTTP handlers/tests;
- minimal `httpserver/server.go` and `cmd/api/main.go` composition;
- executor registration/tests.

## Already-correct behavior to preserve
- request-time highest approved Script selection;
- exact matching approved Proposal snapshot;
- replay before current source/provider/credential checks;
- no provider call in HTTP POST;
- owner-scoped runtime and worker-time credential resolution;
- strict unknown/trailing durable JSON decode;
- request-time Project/Script/Proposal snapshots and locale preservation;
- DB generation-job idempotency with real PostgreSQL same-job concurrency;
- approved Scene Plan history/version semantics;
- internal generation job ID omitted from public Scene Plan JSON;
- safe feature-specific status/result shape;
- Proposal + Script + Scene Plan handlers registered in one generic executor loop;
- Scene Plan resource list/get/PUT/approve API;
- complete accepted Script snapshot validation before provider resolution;
- resolver-not-called proof for invalid Script payload;
- Scene Plan generation status endpoint hides foreign job kinds;
- branch synced with accepted TASK-025 and exact-head CI #236 green.

## Current review blocker
Fix only on existing PR #50/worktree.

**Align Proposal snapshot validation exactly with the accepted Creative Proposal domain before provider resolution.** The current validator is too permissive for several fields:
- `visual_direction`: max 5000 runes (current value is correct);
- `voice_direction`: max 3000 runes, not 5000;
- `music_direction`: max 3000 runes, not 5000;
- `caption_direction`: max 3000 runes, not 5000;
- `warnings`: max 20 items, each trimmed/non-empty and max 1000 runes;
- `research_gaps`: max 20 items, each trimmed/non-empty and max 1000 runes.

A corrupted durable snapshot that could never be a valid approved Proposal must terminalize as `GENERATION_INVALID_PAYLOAD` before `ResolveTextGenerator` is called. Add deterministic regressions for 3001-rune voice/music/caption values, 21 warning/gap items, blank item and 1001-rune item; include at least one invalid Proposal case in the resolver-not-called proof.

## Critical gates
1. No provider call in POST.
2. Replay returns existing durable request before current source/provider credential checks.
3. Worker never switches queued request to a newer Script/Proposal.
4. Narration preservation remains authoritative.
5. Internal generation job ID never public.
6. Crash after Scene Plan draft commit before MarkSuccess creates exactly one version.
7. Approved Scene Plan history remains immutable.
8. No second credential path/executor.
9. Strict durable payload decode/validation happens before provider resolution.
10. Real PostgreSQL same-generation-job concurrency proves one durable Scene Plan version.
11. Capability-specific job status endpoint does not disclose another feature's job kind.
12. Proposal snapshot bounds match the accepted Proposal domain exactly.

## Mandatory isolation
Do not modify Scene Plan frontend, Media Library/Scene Media Binding APIs, visual/TTS provider behavior, media storage semantics, render or publish.

## Final merge gate
- fix the single Proposal snapshot-validation blocker above;
- add focused deterministic regressions including resolver-not-called proof;
- preserve current synced base and all accepted behavior;
- full race/verify and fresh exact-head CI green;
- Team Lead delta review before squash merge.

Continue only on the existing TASK-021 branch/PR. Do not create a replacement branch, self-merge, or self-mark DONE.