# TASK-021 — Scene Plan durable generation + API integration

Status: BACKLOG
Milestone: F1 Creative Workflow
Planned wave: WAVE-F1-I candidate
Branch when activated: `feature/TASK-021-scene-plan-generation-integration`
Base: `develop`
Depends on: TASK-014, TASK-015, TASK-017 accepted; TASK-018 must be accepted before activation.

## Goal
Make Stage 7 Scene Plan a real durable backend workflow by integrating accepted Scene Plan generation/persistence with generic jobs, owner-scoped live text runtime, resource APIs, DB exactly-once persistence and runtime composition.

## Frozen contract
`docs/contracts/SCENE_PLAN_JOB_V1.md`.

Read first:
- `SCENE_PLAN_V1.md`;
- `SCENE_PLAN_GENERATION_V1.md`;
- `JOB_EXECUTION_V1.md`;
- `BYOK_TEXT_PROVIDER_RUNTIME_V1.md`;
- accepted TASK-018 Script job integration once merged.

## Primary ownership when activated
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

## TDD
Implement every deterministic/PostgreSQL gate in `SCENE_PLAN_JOB_V1`, including duplicate enqueue race, crash-window retry, strict payload decode, source relationship errors, owner isolation and exact-head race/full verification.

## Activation gate
Do **not** claim this branch yet.

Before READY:
- TASK-018 merged and its shared `main.go/jobs/httpserver` hotspot released;
- PM/TL rechecks Script-job integration pattern and latest `develop`;
- no conflicting active backend task owns runtime composition.

Do not self-mark READY/DONE or self-merge.