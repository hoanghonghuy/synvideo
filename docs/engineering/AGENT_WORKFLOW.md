# PM / AI Developer / Team Lead Workflow

## Roles
### PM
Owns product intent, scope, priorities, user flows, acceptance criteria, roadmap, task readiness and documentation under `docs/`.

PM may update `AGENTS.md` and `docs/**` directly on `develop` because these are control-plane/product changes rather than application implementation.

### AI Developer
Owns implementation for one approved task at a time. Never implements directly on `main` or `develop`.

### Team Lead
Reviews implementation against the task/spec, architecture constraints, regressions, security, tests and real user behavior. Team Lead does not silently rewrite product decisions during review.

## Branch model
- `main`: stable/release-ready.
- `develop`: integration branch + PM source of truth.
- implementation: `feature/TASK-xxx-*`, `fix/TASK-xxx-*` → PR to `develop`.

## Developer loop
When idle:
1. `git fetch origin`.
2. inspect latest `origin/develop` task board.
3. take only a `READY` task.
4. update local `develop` with fast-forward where possible.
5. create a dedicated task branch.
6. implement/test.
7. PR to `develop`.
8. address Team Lead findings on the same task branch.
9. after merge, return to latest `develop` and check the board again.

Do not continuously pull/merge `develop` while implementing an unrelated task. Sync only when upstream changes are required or at an appropriate integration point.

## Status ownership
PM owns board status and priority. Typical state flow:
`BACKLOG → READY → IN_PROGRESS → REVIEW → (CHANGES_REQUESTED ↔ REVIEW) → DONE`.
Additional states: `BLOCKED`, `CANCELLED`.

AI Developer may report progress but does not self-certify `DONE`.
