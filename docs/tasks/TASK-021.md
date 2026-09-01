# TASK-021 — Scene Plan durable generation + API integration

Status: READY
Milestone: F1 Creative Workflow
Wave: WAVE-F1-I
Branch: `feature/TASK-021-scene-plan-generation-integration`
Base: `develop`
Issue: #39
Depends on: TASK-014, TASK-015, TASK-017, TASK-018 accepted; frozen `SCENE_PLAN_JOB_V1`.

## Goal
Make Stage 7 Scene Plan a real durable backend workflow by integrating accepted Scene Plan generation/persistence with generic jobs, owner-scoped live text runtime, resource APIs, DB exactly-once persistence and runtime composition.

## Frozen contract
`docs/contracts/SCENE_PLAN_JOB_V1.md`.

Read first:
- `SCENE_PLAN_V1.md`;
- `SCENE_PLAN_GENERATION_V1.md`;
- `JOB_EXECUTION_V1.md`;
- `BYOK_TEXT_PROVIDER_RUNTIME_V1.md`;
- accepted TASK-018 Script durable generation integration as the behavioral reference.

## Primary ownership
- new `apps/api/internal/sceneplangenerationjob/**` or cohesive equivalent;
- minimal internal generation-idempotency extension to Scene Plan;
- focused `postgres/scene_plan_repository.go` integration tests;
- migration exactly `0012_add_scene_plan_generation_idempotency.sql`;
- Scene Plan resource + generation HTTP handler/tests;
- minimal `httpserver/server.go` and `cmd/api/main.go` composition;
- executor registration/tests.

## Mandatory isolation
Do not modify:
- Scene Plan frontend workspace (TASK-022);
- Media Library/Scene Media Binding APIs (TASK-023);
- visual/TTS provider packages (TASK-025/026/027);
- media storage semantics;
- render/publish.

## Required capability
- resource list/get/PUT/approve routes from frozen contract;
- `POST /scene-plan-generations` + feature job GET;
- highest approved Script + exact matching approved Proposal request-time snapshot;
- owner runtime validation/worker credential resolution;
- job kind `scene_plan_generation_v1`;
- DB-idempotent Scene Plan draft persistence by generation job ID;
- safe status/result with exact returned Scene Plan version;
- Proposal + Script + Scene Plan job kinds on one generic executor.

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

## Why READY now
TASK-018 is accepted and TASK-019 has merged, so shared runtime/jobs/httpserver composition is free. No active task currently owns that backend hotspot. TASK-025 remains isolated to `providers/**`. The TASK-021 remote branch was absent when PM/TL promoted this task.

## TDD
Implement every deterministic/PostgreSQL gate in `SCENE_PLAN_JOB_V1`, including duplicate enqueue race, crash-window retry, strict payload decode, source relationship errors, owner isolation, request-time source snapshots, real same-job concurrency and exact-head race/full verification.

## Worktree / claim
Before work, confirm remote `feature/TASK-021-scene-plan-generation-integration` is still absent. Atomically create that remote ref from latest `origin/develop`, then use a dedicated TASK-021 worktree. Shared/control checkout remains on `develop`.

Do not self-mark DONE or self-merge.