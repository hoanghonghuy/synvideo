# TASK-021 — Scene Plan durable generation + API integration

Status: CHANGES_REQUESTED
Milestone: F1 Creative Workflow
Wave: WAVE-F1-I
Branch: `feature/TASK-021-scene-plan-generation-integration`
Base: `develop`
PR: #50
Review head: `286c583b0594e6d7a0663255281642808798181d`
Logical TL review: `5080593928`
CI: #231 green on reviewed head
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
- Scene Plan resource list/get/PUT/approve API.

## Current review blockers
Fix only on existing PR #50/worktree.

1. **Complete strict durable snapshot validation before provider resolution.** `validatePayload` must reject malformed approved Script snapshot fields/invariants before `ResolveTextGenerator`, not merely IDs/enums/version linkage. At minimum cover blank section body, invalid/duplicate/oversized section keys, heading/body bounds, estimated-duration/notes bounds and equivalent bounded immutable snapshot fields required by the accepted source contracts. Invalid payload must terminalize as `GENERATION_INVALID_PAYLOAD`. Add tests that prove the resolver/provider is not called for malformed snapshots.
2. **Kind-scope the feature status endpoint.** `GetGeneration` must require `job.Kind == scene_plan_generation_v1`. A Proposal/Script job UUID in the same owner/project must be treated as not found/non-disclosed through `/scene-plan-generations/{job_id}`.
3. **Sync latest `develop`.** TASK-025 merged during this review as `1c550f3165efc5a541177deebd40bedbfd2ba16c`; rebase/sync latest `develop`, resolve only genuine conflicts, and rerun exact-head CI.

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

## Mandatory isolation
Do not modify Scene Plan frontend, Media Library/Scene Media Binding APIs, visual/TTS provider behavior, media storage semantics, render or publish.

## Final merge gate
- fix the two code blockers above;
- add focused deterministic regressions;
- sync latest `develop`;
- full race/verify and fresh exact-head CI green;
- Team Lead delta review before squash merge.

Continue only on the existing TASK-021 branch/PR. Do not create a replacement branch, self-merge, or self-mark DONE.