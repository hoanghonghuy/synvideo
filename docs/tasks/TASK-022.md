# TASK-022 — Scene Plan creator workspace

Status: CHANGES_REQUESTED
Milestone: F1 Creative Workflow
Wave: WAVE-F1-J parallel lane
Branch: `feature/TASK-022-scene-plan-workspace`
Base: `develop`
PR: #51
Review head: `1f5d00fc45dc049404dca648b80eefc31ede167b`
Logical TL review: `5085479067`
CI: #252 green on reviewed head
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

## Already-correct behavior to preserve
- true empty state when all required load context succeeds and no Scene Plan exists;
- active draft/newest opening behavior;
- newest/history/version switching with dirty guard;
- approved/superseded read-only behavior;
- stale approved Script source warning;
- planning-field edits only; approved narration is not a freeform editor;
- same-section merge with ordered narration concatenation;
- optimistic revision save and stale local-edit preservation;
- approve current clean draft;
- owner provider/model selection;
- fresh UUID generation requests;
- displayed plan remains visible while generation is queued/running;
- same durable job polling/recovery through sessionStorage;
- succeeded job exact-version load/retry without regeneration;
- terminal Retry creates a fresh request ID.

## Current review blockers
Fix only on existing PR #51/worktree.

1. **Partial-load failure truthfulness.** Generation-options failure must remain a retryable load/error state, not become “no providers configured”. Project/plans/approved-Script context failures must not be hidden merely because a non-empty Scene Plan list was already fetched.
2. **Refresh/resume ordering.** Initial workspace load and resumed job polling must not race such that an initial draft/newest selection overwrites the exact version returned by a succeeded resumed job. The succeeded durable job selection is authoritative.
3. **Narration-safe split at Unicode boundaries and valid keys.** Do not split by raw UTF-16 code-unit indexes that can bisect surrogate pairs. Enforce accepted scene key slug shape, <=64 runes and uniqueness before mutating the local draft.
4. **Accessible validation errors.** Backend field errors must be rendered and associated with the relevant scene controls, with a safe form-level fallback for content-wide errors.
5. **Lineage inspection.** Display immutable `source_proposal_version` alongside Script lineage in metadata/history.

## Required regressions
- provider-options request failure is retryable and distinct from valid empty provider configuration;
- approved-Script/context failure after non-empty plan list remains visibly retryable;
- delayed initial workspace load cannot overwrite exact succeeded resumed job version;
- emoji/non-BMP narration split preserves valid text exactly;
- invalid/duplicate/>64-rune split keys are rejected before mutation;
- backend field validation error is visible and associated with the corresponding control.

## Critical product gate
Scene Plan is not a second Script editor. Any UI operation that can add/omit/paraphrase/reorder approved narration is out of scope and must be rejected by design/server validation.

## Mandatory isolation
Do not modify backend, Media Library/Scene Binding, image/TTS providers, jobs, storage, render or publish.

## Final merge gate
- fix all current review blockers on PR #51;
- preserve already-correct behavior;
- focused regressions plus full frontend verify green;
- fresh exact-head CI green;
- Team Lead delta review before squash merge.

Continue only on existing branch/PR #51. Do not self-mark DONE or self-merge.