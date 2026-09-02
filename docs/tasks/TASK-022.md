# TASK-022 — Scene Plan creator workspace

Status: READY
Milestone: F1 Creative Workflow
Wave: WAVE-F1-I
Branch: `feature/TASK-022-scene-plan-workspace`
Base: `develop`
Issue: #40
Depends on: TASK-019 accepted; TASK-021 accepted via PR #50 / squash `9d2b5306df7755fbcbe487bcd8bd382e5340fdec`.

## Goal
Deliver creator-usable Stage 7 Scene Plan history/edit/generate/approve workflow while protecting approved Script narration and durable generation recovery.

## Frozen contract
`docs/contracts/SCENE_PLAN_WORKSPACE_V1.md`.

## Primary ownership
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

## Activation evidence
- TASK-019 merged and frontend router/locale/navigation hotspot is released;
- TASK-021 backend/API accepted and revalidated against frozen workspace contract;
- remote branch `feature/TASK-022-scene-plan-workspace` was absent at promotion;
- no other active frontend task owns the same shared route/navigation surfaces.

## Worktree / claim
Atomically create remote `feature/TASK-022-scene-plan-workspace` from latest `origin/develop`, then use a dedicated TASK-022 worktree. Shared/control checkout remains on `develop`.

Do not self-mark DONE or self-merge.