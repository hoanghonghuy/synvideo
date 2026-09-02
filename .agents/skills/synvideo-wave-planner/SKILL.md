---
name: synvideo-wave-planner
description: Plan a small batch of independent SynVideo implementation tasks using fresh remote state, duplicate prevention, frozen contracts, path/worktree ownership, TDD gates and safe merge order.
---

# SynVideo Wave Planner

Use this skill when PM wants multiple developers to work concurrently or asks to prepare the next batch/wave of tasks.

## Goal
Increase throughput without creating avoidable merge conflicts, duplicated work, stale-state activation, filesystem collisions, architectural drift or speculative dependencies.

Follow `docs/engineering/CONTROL_PLANE_PROTOCOL.md` for remote authority, freshness, transition ordering and duplicate-task prevention.

## Default wave size
Prefer up to 3 concurrent implementation tasks. Use fewer when dependencies/write surfaces are coupled; use more only with a concrete reason and equally clear isolation.

## Remote preflight
Before planning or activating work:
1. inspect current remote `develop` head;
2. inspect live open and recently completed task issues;
3. inspect active/recent PRs and canonical task branches;
4. inspect current BOARD/task/contracts on `develop`;
5. search for the same product outcome/domain/write surface before creating a new task.

Do not infer "not already planned" from a stale local checkout or only from `BOARD.md`.

## Planning workflow
1. Build the accepted foundation and active-work picture from the remote preflight rather than one local source.
2. Repair obvious metadata mirrors when live state is already unambiguous; stop for material contract conflicts.
3. Do not retroactively split an implementation task that is already claimed/materially in progress unless PM explicitly stops/re-scopes it and records the decision.
4. Identify the next product outcome and dependency graph.
5. Run duplicate/overlap detection across issues, task specs, active/recent PRs and canonical branches. Reuse/update/link existing work when appropriate instead of creating duplicate task IDs.
6. Find seams that can be implemented independently.
7. Freeze any shared API/domain/event contract on `develop` **before** authorizing consumer/provider tasks.
8. For every task define canonical branch, dependencies/wave gate, primary write paths, allowed shared integration files, reserved paths, TDD plan/verification and merge/integration order.
9. Verify worktree compatibility: one dedicated task worktree per implementation agent; shared/control checkout remains on `develop`.
10. Create/update full task specs and GitHub issues while tasks are not yet executable.
11. Activate safely: commit/freeze task spec, contracts and PM ordering on `develop`; verify the remote result; then update the authoritative GitHub issue to `READY` **last**. The issue transition is the live authorization signal.
12. Coding agents claim authorized work via the atomic branch-as-lock + dedicated-worktree rule in `PARALLEL_WORK_PROTOCOL.md`.

## Claimed-task changes
Once the canonical remote branch exists, PM must not silently mutate material scope/acceptance/branch/frozen contracts. Record and surface any material re-scope to the active branch/PR owner under `CONTROL_PLANE_PROTOCOL.md`.

## Isolation test
Do not place two tasks in the same wave if both require substantial edits to the same core files/schema/domain package and neither can be separated by a frozen contract.

Small explicit shared hotspots are acceptable when listed in both task specs and easy to rebase/resolve. Filesystem isolation is separate from path ownership: different paths still require different worktrees.

## TDD
All new wave tasks must reference `docs/engineering/TDD_PROTOCOL.md` and define their first meaningful RED behaviors before implementation starts.

## Merge planning
- Independent tasks can merge when accepted.
- Contract provider merges before a consumer's final real integration check when runtime integration is required.
- Consumer may develop against deterministic mocks before the provider merges if the contract is frozen.
- After an upstream wave PR merges, rebase only the affected dependent branch and rerun its integration verification from that task's dedicated worktree.

## PM output
A planned wave is complete only when:
- remote duplicate/overlap checks were performed;
- BOARD shows intended ordering/dependencies/status mirror;
- each task has a complete current `TASK-xxx.md`;
- shared contracts are committed on `develop`;
- GitHub issues have correct live authorization states;
- path ownership and merge order are explicit;
- worktree/atomic-claim policy is inherited from `PARALLEL_WORK_PROTOCOL.md`;
- no claimed task was silently re-scoped;
- no task requires another developer to guess product behavior.
