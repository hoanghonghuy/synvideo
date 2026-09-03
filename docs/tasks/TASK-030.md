# TASK-030 — Per-scene AI Image Generation Workspace

Status: READY
Milestone: F1 Creative Workflow
Owner role: AI Developer
PR target: develop
Canonical branch when activated: `feature/TASK-030-image-generation-workspace`
Depends on: TASK-029 accepted on `develop`

## Problem / Goal
TASK-029 now provides durable generated-image acquisition, MediaAsset ingestion and optional scene assignment. Deliver the complete creator-facing per-scene image-generation workflow instead of scattering it across micro-tasks.

## Frozen contract
`docs/contracts/GENERATED_IMAGE_WORKSPACE_V1.md`

## Scope
Integrate TASK-029 APIs; scene prompt editing; safe enabled provider/model selection; stable request-id submit/retry; queued/running polling and refresh recovery; exact generated MediaAsset preview; keep-as-alternative vs assign/replace; truthful generation/assignment recovery; regeneration history; i18n/accessibility/responsive tests.

## Required behavior
No cross-owner/project/scene generation; ambiguous submit reuses request ID; polling/refresh resumes same job; exact success result loads; asset success and assignment success stay distinct; deliberate regeneration uses a new request ID; stale responses cannot overwrite current scene state; no secret/external provider data leaks.

## Acceptance criteria
- [x] TASK-029 accepted on `develop`.
- [x] `GENERATED_IMAGE_WORKSPACE_V1` is on `develop`.
- [ ] creator can generate/observe/resume/preview.
- [ ] duplicate paid generation is avoided across ambiguous submit/recovery.
- [ ] creator can keep alternative or assign/replace.
- [ ] assignment failure preserves successful asset.
- [ ] regeneration preserves older alternatives/history.
- [ ] truthful unavailable/failure/stale states.
- [ ] exact-head CI green.

## Primary write surface
`apps/web/src/features/media/**` and/or dedicated generated-image feature, feature API client, focused route/project integration, locale/tests. Shared router/ProjectDetail/locales hotspots must stay minimal. Do not redesign TASK-029 backend semantics unless a proven integration defect requires a separately recorded amendment.

## TDD
Follow `docs/engineering/TDD_PROTOCOL.md`; first RED coverage includes request-id reuse, same-job polling recovery, exact result load, asset-success/assignment-failure and stale-response protection.

## Activation gate
READY — TASK-029 is accepted, task/contract are on protected `develop`, duplicate/canonical-branch checks are clean, and issue #65 is the authoritative live authorization once updated to READY. Claim only by atomically creating `feature/TASK-030-image-generation-workspace` from current `origin/develop`.
