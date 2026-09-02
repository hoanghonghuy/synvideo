# TASK-022 — Scene Plan creator workspace

Status: CHANGES_REQUESTED
Milestone: F1 Creative Workflow
Wave: WAVE-F1-J parallel lane
Branch: `feature/TASK-022-scene-plan-workspace`
Base: `develop`
PR: #51
Review head: `1f5d00fc45dc049404dca648b80eefc31ede167b`
Logical TL reviews: `5085479067`, `5088647732`
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
- approved/superseded read-only behavior;
- stale approved Script source warning;
- planning-field edits only; approved narration is not a freeform editor;
- same-section merge with ordered narration concatenation;
- optimistic revision save and stale local-edit preservation;
- approve current clean draft;
- owner provider/model selection;
- fresh UUID generation requests;
- displayed plan remains visible while generation is queued/running;
- durable job identity persisted through sessionStorage;
- succeeded job exact-version load/retry without regeneration;
- terminal Retry creates a fresh request ID.

## Authoritative consolidated review blockers
Fix only on existing PR #51/worktree.

1. **Partial-load truthfulness.** Generation-options failure must remain a retryable error rather than become “no providers configured”. Project/plans/approved-Script failures must not be hidden after a non-empty Scene Plan list, and initial selected-version load failure must remain visibly retryable.
2. **Refresh/resume ordering.** Initial workspace loading and resumed job polling must not race so that a late initial draft/newest selection overwrites the exact `scene_plan_version` from a succeeded durable job.
3. **Dirty history switch Save / Discard / Cancel.** Preserve the originally requested target version through a successful save and open it afterward; the current global Save clears `pendingVersion` and is not equivalent.
4. **No duplicate POST on transient status error.** A transient poll failure must retry the same durable job/status only. The current generic retry-generation action can call `startGeneration()` while queued/running `activeJob` exists and POST a replacement job. Only terminal generation retry may create a fresh request ID.
5. **Unicode/key-safe split.** Split on Unicode-safe boundaries (at least code points), never raw UTF-16 boundaries that can bisect surrogate pairs. Enforce accepted scene-key `^[a-z0-9_-]+$`, <=64 runes and uniqueness before local mutation.
6. **Accessible backend validation errors.** Render field errors next to/for the relevant scene controls with accessible association and a safe form/content-level fallback.
7. **Lineage inspection.** Display immutable `source_proposal_version` alongside Script lineage in metadata/history.
8. **Final sync.** Sync latest `develop`, preserve scope, rerun focused regressions + full frontend verify + fresh exact-head CI.

## Required regressions
- provider-options failure is retryable and distinct from valid empty provider configuration;
- Script/context failure after a non-empty plan list is visibly retryable;
- initial selected-version fetch failure is visibly retryable;
- delayed initial load cannot overwrite exact succeeded resumed-job version;
- Save-and-switch preserves edits and opens the requested target version;
- clicking retry during transient status failure produces zero generation POSTs and retains the same job ID;
- emoji/non-BMP narration split preserves text exactly;
- invalid/duplicate/>64-rune split keys are rejected before mutation;
- backend field validation error is visible and associated with the corresponding control;
- Proposal lineage is visible.

## Critical product gate
Scene Plan is not a second Script editor. Any UI operation that can add/omit/paraphrase/reorder approved narration is out of scope and must be rejected by design/server validation.

## Mandatory isolation
Do not modify backend, Media Library/Scene Binding, image/TTS providers, jobs, storage, render or publish.

## Final merge gate
- fix all authoritative blockers on PR #51;
- preserve already-correct behavior;
- focused deterministic regressions plus full frontend verify green;
- sync latest `develop` and fresh exact-head CI green;
- Team Lead delta review before squash merge.

Issue #40 is the authoritative consolidated review gate if older PR comments differ in wording. Continue only on existing branch/PR #51. Do not self-mark DONE or self-merge.