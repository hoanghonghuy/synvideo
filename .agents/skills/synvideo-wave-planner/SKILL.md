---
name: synvideo-wave-planner
description: Plan a small batch of independent SynVideo implementation tasks for multiple developers with frozen contracts, path ownership, TDD gates and safe merge order.
---

# SynVideo Wave Planner

Use this skill when PM wants multiple developers to work concurrently or asks to prepare the next batch/wave of tasks.

## Goal
Increase throughput without creating avoidable merge conflicts, duplicated work, architectural drift or speculative dependencies.

## Default wave size
Prefer up to 3 concurrent implementation tasks. Use fewer when dependencies/write surfaces are coupled; use more only with a concrete reason and equally clear isolation.

## Planning workflow
1. Read `docs/tasks/BOARD.md` and identify the currently accepted foundation and active work.
2. Do not retroactively split an implementation task that is already materially in progress unless PM explicitly decides to stop/re-scope it.
3. Identify the next product outcome and dependency graph.
4. Find seams that can be implemented independently (for example backend contract provider, frontend contract consumer, isolated provider/platform package).
5. Freeze any shared API/domain/event contract on `develop` **before** opening consumer/provider tasks in parallel.
6. For every task define:
   - canonical branch;
   - dependencies/wave gate;
   - primary write paths;
   - allowed shared integration files;
   - reserved/do-not-touch paths;
   - TDD plan and verification;
   - merge/integration order.
7. Create full task specs and GitHub issues.
8. Keep tasks BLOCKED until their shared gate is accepted. When safe, move the whole independent set to READY together.
9. Coding agents claim READY work via the branch-as-lock rule in `docs/engineering/PARALLEL_WORK_PROTOCOL.md`.

## Isolation test
Do not place two tasks in the same wave if both require substantial edits to the same core files/schema/domain package and neither can be separated by a frozen contract.

Small explicit shared hotspots (router registration, locale registration, composition roots) are acceptable when listed in both task specs and easy to rebase/resolve.

## TDD
All new wave tasks must reference `docs/engineering/TDD_PROTOCOL.md` and define their first meaningful RED behaviors before implementation starts.

## Merge planning
- Independent tasks can merge when accepted.
- Contract provider merges before a consumer's final real integration check when runtime integration is required.
- Consumer may develop against deterministic mocks before the provider merges if the contract is frozen.
- After an upstream wave PR merges, rebase only the affected dependent branch and rerun its integration verification.

## PM output
A planned wave is complete only when:
- BOARD shows wave/dependencies/status;
- each task has a complete `TASK-xxx.md`;
- shared contracts are committed on `develop`;
- GitHub issues exist with correct BLOCKED/READY state;
- path ownership and merge order are explicit;
- no task requires another developer to guess product behavior.
