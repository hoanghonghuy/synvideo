---
name: AI coding task
about: Implementation task executed by an AI coding agent
---

## Task
`TASK-XXX`

## Status
`BACKLOG` / `BLOCKED` until PM activation. Change this authoritative issue to `READY` only after the task spec/contracts/ordering are frozen on remote `develop`.

## Git workflow
- Base branch: `develop`
- Branch: `feature/TASK-XXX-<slug>`
- Follow `docs/engineering/CONTROL_PLANE_PROTOCOL.md` and `PARALLEL_WORK_PROTOCOL.md`.
- Before deciding work exists/does not exist, inspect live issue/PR/remote branch state and refresh `origin/develop`; local absence is not remote absence.
- `READY` authorizes execution but does not mean unclaimed. Existing canonical remote branch/active PR means claimed.
- Atomically create the absent canonical remote branch from the selected current `origin/develop` SHA; plain same-SHA push is not a lock.
- Never commit implementation directly to `main` or `develop`.
- Use one dedicated worktree for this implementation branch.
- Open/update a PR back to `develop` when verification passes.
- Do not merge your own PR and do not mark the task `DONE`.

## Read first
1. `AGENTS.md`
2. `docs/engineering/CONTROL_PLANE_PROTOCOL.md`
3. This live GitHub issue + canonical remote branch/PR state
4. refreshed `docs/tasks/BOARD.md` from current `origin/develop`
5. `docs/tasks/TASK-XXX.md` from current `origin/develop`
6. only additional documents referenced by the task spec.

## Scope
See `docs/tasks/TASK-XXX.md`.

If this issue and the task spec disagree materially on scope, branch, acceptance criteria, dependencies or frozen contracts, stop for PM/Team Lead reconciliation. Do not guess. Metadata-only status drift is handled by the authority protocol.

## Delivery
In the PR description include:
- implemented scope;
- exact task/contract version or relevant base SHA when useful;
- TDD evidence and tests/verification run;
- migrations/config changes;
- known limitations or follow-up findings;
- open-source code reused, if any, with source and license.
