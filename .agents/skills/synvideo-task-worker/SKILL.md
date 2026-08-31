---
name: synvideo-task-worker
description: Implement exactly one SynVideo READY task on a dedicated branch, using minimal context, TDD, required verification, and a PR to develop.
---

# SynVideo Task Worker

## Use when
Implementing a PM-approved SynVideo task.

## Workflow
1. Read root `AGENTS.md`.
2. Read `docs/tasks/BOARD.md`.
3. Confirm the task is `READY`, its dependencies are satisfied, and identify its canonical branch/spec.
4. Fetch latest `origin/develop` and remote branches. Confirm the canonical remote task branch does not already exist and there is no active PR for the task.
5. Create the canonical task branch from latest `origin/develop` and immediately push the new branch to origin as the claim/lock. If the branch already exists or the push loses the race, do not work on that task; choose another eligible READY task.
6. Read only the task spec and the exact docs/contracts it references.
7. If the task builds a substantial subsystem, invoke the open-source research workflow first.
8. Follow `docs/engineering/TDD_PROTOCOL.md` for behavior changes: write the failing test first, make it pass minimally, then refactor with tests green.
9. Respect declared primary write paths, shared integration files and reserved paths. Use `docs/engineering/PARALLEL_WORK_PROTOCOL.md` for parallel-wave tasks.
10. Implement only the stated scope and acceptance criteria.
11. Run required targeted checks plus the task's repository verification.
12. Open/update a PR to `develop` with: scope, implementation summary, TDD evidence, tests, risks, assumptions and reuse/license notes if any.

## Stop conditions
Do not silently broaden scope. If a requirement materially conflicts with an accepted contract/ADR/spec, or a parallel task requires a write outside this task's declared surface, record the conflict for PM/Team Lead rather than inventing behavior.

## Context discipline
Do not recursively read `docs/**`. Follow links from the current task only.
