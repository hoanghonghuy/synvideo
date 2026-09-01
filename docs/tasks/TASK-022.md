# TASK-022 — Scene Plan creator workspace

Status: BACKLOG
Milestone: F1 Creative Workflow
Planned wave: WAVE-F1-I candidate
Branch when activated: `feature/TASK-022-scene-plan-workspace`
Base: `develop`
Depends on: TASK-019 accepted; TASK-021 backend contract frozen and implementation accepted or stable for exact API verification.

## Goal
Deliver creator-usable Stage 7 Scene Plan history/edit/generate/approve workflow while protecting approved Script narration and durable generation recovery.

## Frozen contract
`docs/contracts/SCENE_PLAN_WORKSPACE_V1.md`.

## Primary ownership when activated
- `apps/web/src/features/scene-plan/**`;
- Scene Plan frontend API/types/tests;
- minimal `/projects/:id/scene-plan` route;
- minimal Project detail workflow navigation;
- Scene Plan locale keys and only unavoidable shared responsive styles.

Frontend-only. Do not modify `apps/api/**`.

## Required UX
- true empty/error/loading states;
- newest/history/version switching with dirty guard;
- stale approved Script source awareness;
- edit planning metadata only;
- narration-safe split/merge, no freeform hidden Script rewrite;
- optimistic revision save and stale conflict preservation;
- approve current clean draft;
- owner provider/model selection;
- durable Generate/Regenerate with refresh/navigation resume;
- pending/failure preserves displayed plan;
- succeeded job opens exact returned version.

## Critical product gate
Scene Plan is not a second Script editor. Any UI operation that can add/omit/paraphrase/reorder approved narration is out of scope and must be rejected by design/server validation.

## TDD
Cover every frontend regression in `SCENE_PLAN_WORKSPACE_V1`, including split/merge preservation, dirty guards, transient polling recovery, exact-version success and long-form responsive behavior.

## Activation gate
Do **not** claim yet.

Before READY:
- TASK-019 merged and frontend router/locale/navigation hotspot released;
- TASK-021 API contract revalidated against accepted backend head;
- PM/TL confirms no other active frontend task owns the same shared route/navigation files.

Do not self-mark READY/DONE or self-merge.