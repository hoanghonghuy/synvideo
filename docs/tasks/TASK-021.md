# TASK-021 — Scene Plan durable generation + API integration

Status: DONE
Milestone: F1 Creative Workflow
Wave: WAVE-F1-I
Branch: `feature/TASK-021-scene-plan-generation-integration`
Base: `develop`
PR: #50
Accepted head: `3a78f24a1c38b32b3c109f616d651ef1c004bbcb`
Logical TL APPROVE: `5084672798`
CI: #242 green on accepted head
Squash merge: `9d2b5306df7755fbcbe487bcd8bd382e5340fdec`
Issue: #39 completed
Depends on: TASK-014, TASK-015, TASK-017, TASK-018 accepted; frozen `SCENE_PLAN_JOB_V1`.

## Goal
Make Stage 7 Scene Plan a real durable backend workflow by integrating accepted Scene Plan generation/persistence with generic jobs, owner-scoped live text runtime, resource APIs, DB exactly-once persistence and runtime composition.

## Delivered
- Scene Plan list/get/PUT/approve resource API;
- durable `scene_plan_generation_v1` create/status API;
- request-time highest approved Script + exact matching approved Proposal snapshot;
- owner-scoped provider/model validation at request time and current owner credential resolution at worker execution;
- strict unknown/trailing JSON and full bounded Project/Script/Proposal snapshot validation before provider resolution;
- narration preservation through the accepted Scene Plan generation engine;
- internal generation job ID hidden from public resources;
- feature status endpoint hides foreign job kinds;
- DB-level generation-job idempotency and crash-window replay behavior;
- real PostgreSQL same-job concurrency proof;
- request-time locale preservation;
- Proposal + Script + Scene Plan job kinds registered on the single generic executor.

## Frozen contract
`docs/contracts/SCENE_PLAN_JOB_V1.md`.

TASK-021 is accepted. Follow-on creator workspace work belongs to TASK-022.