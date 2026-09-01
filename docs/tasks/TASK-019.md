# TASK-019 — Script creator workspace

Status: CHANGES_REQUESTED
Milestone: F1 Creative Workflow
Wave: WAVE-F1-H
Branch: `feature/TASK-019-script-workspace`
Base: `develop`
PR: #47
Review head: `33b0d14aa684c3af0462ab2260491f0369730e23`
Logical TL review: `5079860958`
CI: #206 green on reviewed head
Depends on: TASK-011 accepted; frozen `SCRIPT_WORKSPACE_V1` and `SCRIPT_JOB_V1`. Implements against TASK-018 frozen API contract.

## Goal
Deliver the creator-facing Stage 5 Script workspace: history, editing, optimistic save, approval, source-staleness awareness, owner-scoped Generate/Regenerate, durable job recovery, and safe failure behavior.

## Authoritative contract
`docs/contracts/SCRIPT_WORKSPACE_V1.md`.

Read first:
- `AGENTS.md`;
- `docs/engineering/TDD_PROTOCOL.md`;
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`;
- `docs/contracts/SCRIPT_V1.md`;
- `docs/contracts/SCRIPT_JOB_V1.md`;
- `docs/contracts/SCRIPT_WORKSPACE_V1.md`;
- accepted Creative Proposal workspace as UX/state-machine reference.

## Primary ownership
- `apps/web/src/features/script/**`;
- Script frontend API client and tests;
- minimal `apps/web/src/router/index.ts` route for `/projects/:id/script`;
- minimal Project-detail navigation/link integration;
- Script locale keys in existing locale files;
- only minimal shared styles required by this workspace.

## Mandatory isolation
Do not modify:
- `apps/api/**` — TASK-018 owns Script backend generation integration;
- Scene Plan/media frontend or backend;
- provider-settings behavior;
- render/publish paths;
- unrelated shared layout/design refactors.

## Required capability
Implement:
- canonical `/projects/:id/script` workspace;
- version history and status badges;
- active-draft editing of ordered sections, estimated duration and notes;
- optimistic revision save and stale-revision preservation/reconcile path;
- approval with dirty guard;
- stale-source warning when a newer approved Proposal exists;
- owner-scoped provider/model selection;
- Generate/Regenerate durable job lifecycle;
- active-job recovery across refresh/navigation;
- exact succeeded Script version loading;
- retryable load/poll failure handling without accidental regeneration.

## Critical invariants
1. Approved/superseded Script versions are read-only.
2. Unsaved edits are never silently discarded by history switch, approval, regeneration or recovery logic.
3. Dirty state blocks approval and Generate/Regenerate until explicit save/discard.
4. No approved Proposal -> generation disabled with clear next-step guidance.
5. No enabled provider/model -> generation disabled with link/guidance to AI Provider settings.
6. Every explicit Generate/Regenerate/terminal Retry gets a fresh valid UUID request ID.
7. Transient polling error retries the same job ID and never POSTs a replacement job.
8. Existing Script remains mounted while generation is pending or fails.
9. Succeeded job load failure retries the exact returned Script version and is not presented as Regenerate.
10. No provider credential/secret or Script draft content is persisted as recovery state.

## Current review blockers
Fix only on the existing PR/worktree, preserving already-correct behavior.

1. **Valid UUID on every explicit generation action.** The fallback from `crypto.randomUUID()` currently emits a non-UUID. Use a valid UUIDv4 fallback (for example `crypto.getRandomValues`) or a clear unsupported/error path. Add a test that inspects POST bodies and proves distinct valid UUIDs for first generation and terminal Retry.
2. **Explicit stale-revision reload/reconcile path.** Current stale save preserves local edits but only displays an error. Add an explicit creator action to reload/reconcile server state, with confirmation before discarding local edits; never silently replace them.
3. **Complete frozen high-risk frontend coverage.** Add focused deterministic tests for no-provider/settings guidance, approved/superseded read-only history, optimistic save success, approval success and stale/error preservation, fresh request IDs across Retry, terminal generation failure preserving mounted Script, and non-secret recovery persistence.
4. Sync latest `develop` before final review.

## Already-correct behavior to preserve
- no-Script + Proposal-readiness state;
- dirty version-switch and generation guards;
- newer approved Proposal stale-source warning;
- list/load errors do not become false empty state;
- transient poll failure retries the same job without POSTing another;
- succeeded job opens exact returned Script version;
- failed reload after success retains the succeeded job and retries the read rather than regenerating;
- only non-secret job identity is stored for session recovery.

## TDD plan
Truthful RED → GREEN → REFACTOR must cover at least:
- empty Script state;
- Proposal-not-approved readiness state;
- provider-not-configured state/settings guidance;
- history selection/read-only approved and superseded states;
- dirty guards;
- save success/stale/network behavior plus explicit stale reconcile;
- approval success/stale/error;
- stale source Proposal warning;
- fresh valid UUID per explicit generation/retry action;
- queued/running/failed preservation;
- refresh/navigation job recovery;
- transient poll same-job retry;
- succeeded exact-version load;
- succeeded-version read failure without duplicate generation;
- list/load error not becoming false empty state;
- no secret or Script content client persistence;
- lint/typecheck/tests/build.

## Acceptance criteria
- [ ] `SCRIPT_WORKSPACE_V1` implemented without contract drift.
- [ ] Creator can inspect, edit, save and approve Script versions safely.
- [ ] Stale revision preserves edits until explicit reconcile/reload choice.
- [ ] Creator can Generate/Regenerate using owner-scoped provider choices with valid fresh UUIDs.
- [ ] Durable generation recovers after refresh and preserves current Script on failure.
- [ ] Source Proposal staleness is visible but never silently mutates Script history.
- [ ] No backend or Scene Plan/media write-surface leakage.
- [ ] Required frontend gates, lint, typecheck, tests, build and exact-head CI are green.

## Worktree / review protocol
The branch is already claimed. Continue only in the existing TASK-019 dedicated worktree/PR #47. Review fixes stay on this branch. Do not create a replacement branch, self-merge, or self-mark DONE.