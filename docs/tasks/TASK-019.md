# TASK-019 — Script creator workspace

Status: READY
Milestone: F1 Creative Workflow
Wave: WAVE-F1-H
Branch: `feature/TASK-019-script-workspace`
Base: `develop`
Depends on: TASK-011 accepted; frozen `SCRIPT_WORKSPACE_V1` and `SCRIPT_JOB_V1`. May implement in parallel with TASK-018 against the frozen backend contract.

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

TASK-019 may consume frozen TASK-018 endpoints before TASK-018 merges. Mock/unit tests must stay deterministic; do not invent a different API because the backend PR is not yet merged.

## Required capability
Implement:
- canonical `/projects/:id/script` workspace;
- version history and status badges;
- active-draft editing of ordered sections, estimated duration and notes;
- optimistic revision save and stale-revision preservation;
- approval with dirty guard;
- stale-source warning when a newer approved Proposal exists;
- owner-scoped provider/model selection;
- Generate/Regenerate durable job lifecycle;
- active-job recovery across refresh/navigation;
- exact succeeded Script version loading;
- retryable load/poll failure handling without accidental regeneration.

## Critical invariants
1. Approved/superseded Script versions are read-only.
2. Unsaved edits are never silently discarded by history switch, approval, regeneration or refresh logic.
3. Dirty state blocks approval and Generate/Regenerate until explicit save/discard.
4. No approved Proposal -> generation disabled with clear next-step guidance.
5. No enabled provider/model -> generation disabled with link/guidance to AI Provider settings.
6. Every explicit Generate/Regenerate/terminal Retry gets a fresh request ID.
7. Transient polling error retries the same job ID and never POSTs a replacement job.
8. Existing Script remains mounted while generation is pending or fails.
9. Succeeded job load failure retries the exact returned Script version and is not presented as Regenerate.
10. No provider credential/secret is stored or handled by the workspace.

## TDD plan
Truthful RED → GREEN → REFACTOR must cover at least:
- empty Script state;
- Proposal-not-approved readiness state;
- provider-not-configured state;
- history selection/read-only states;
- dirty guards;
- save success/stale/network behavior;
- approval success/stale/error;
- stale source Proposal warning;
- unique request ID per explicit generation action;
- queued/running/failed preservation;
- refresh/navigation job recovery;
- transient poll same-job retry;
- succeeded exact-version load;
- succeeded-version read failure without duplicate generation;
- list/load error not becoming false empty state;
- no secret client persistence;
- lint/typecheck/tests/build.

## Acceptance criteria
- [ ] `SCRIPT_WORKSPACE_V1` implemented without contract drift.
- [ ] Creator can inspect, edit, save and approve Script versions safely.
- [ ] Creator can Generate/Regenerate using owner-scoped provider choices.
- [ ] Durable generation recovers after refresh and preserves current Script on failure.
- [ ] Source Proposal staleness is visible but never silently mutates Script history.
- [ ] No backend or TASK-020 write-surface leakage.
- [ ] Frontend verification and full CI are green.

## Worktree / claim
Before work, confirm remote `feature/TASK-019-script-workspace` does not exist. Atomically create that remote ref from latest `origin/develop`, then use a dedicated TASK-019 worktree. Shared/control checkout remains on `develop`.

Do not self-merge or self-mark DONE.