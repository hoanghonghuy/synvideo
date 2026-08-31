---
name: AI coding task
about: Implementation task executed by an AI coding agent
---

## Task
`TASK-XXX`

## Status
`READY`

## Git workflow
- Base branch: `develop`
- Branch: `feature/TASK-XXX-<slug>`
- Never commit implementation directly to `main` or `develop`.
- Start from the latest `origin/develop`.
- Open a PR back to `develop` when verification passes.
- Do not merge your own PR and do not mark the task `DONE`.

## Read first
1. `AGENTS.md`
2. `docs/tasks/BOARD.md`
3. `docs/tasks/TASK-XXX.md`
4. Only the additional documents referenced by the task spec.

## Scope
See `docs/tasks/TASK-XXX.md`.

## Delivery
In the PR description include:
- implemented scope;
- tests/verification run;
- migrations/config changes;
- known limitations or follow-up findings;
- open-source code reused, if any, with source and license.
