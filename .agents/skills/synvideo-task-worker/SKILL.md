---
name: synvideo-task-worker
description: Implement exactly one SynVideo READY task on a dedicated branch, using minimal context, required verification, and a PR to develop.
---

# SynVideo Task Worker

## Use when
Implementing a PM-approved SynVideo task.

## Workflow
1. Read root `AGENTS.md`.
2. Read `docs/tasks/BOARD.md`.
3. Confirm the task is `READY` and identify its task spec.
4. Update local `develop`, then create the task branch.
5. Read only the task spec and the exact docs it references.
6. If the task builds a substantial subsystem, invoke the open-source research workflow first.
7. Implement only the stated scope and acceptance criteria.
8. Add/update tests required by the task and affected risk surface.
9. Run required checks from the task plus relevant repository checks.
10. Open/update a PR to `develop` with: scope, implementation summary, tests, risks, assumptions and reuse/license notes if any.

## Stop conditions
Do not silently broaden scope. If a requirement is materially ambiguous or conflicts with an accepted ADR/spec, record the conflict for PM/Team Lead rather than inventing behavior.

## Context discipline
Do not recursively read `docs/**`. Follow links from the current task only.
