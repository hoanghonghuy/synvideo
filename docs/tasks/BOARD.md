# SynVideo Task Board

Current milestone: `F0 — TECHNICAL FOUNDATION`

PM plans ahead, but AI Developers may start only tasks explicitly marked `READY`. Dependent tasks remain blocked until the preceding foundation is reviewed and accepted.

| ID | Task | Status | Spec / Notes |
|---|---|---|---|
| BOOT-001 | Initialize repository and establish `develop` | DONE | `main` contains the initial repository commit; PM/product scaffold lives on `develop`. |
| AUDIT-001 | Audit pre-existing codebase | CANCELLED | Not applicable: there is no pre-existing application code. |
| PLAN-001 | Define Foundation milestone and first implementation task set | DONE | Initial architecture baseline and first dependency chain are defined. |
| TASK-001 | Technical foundation and runnable project skeleton | CHANGES_REQUESTED | PR #3 reviewed by Team Lead. Fix the requested foundation/toolchain/config issues on the existing task branch and re-request review. |
| TASK-002 | Project domain and persistence foundation | BLOCKED | Depends on accepted TASK-001. Preview: `docs/tasks/TASK-002.md`. |
| TASK-003 | Creative Brief intake foundation | BLOCKED | Depends on accepted TASK-002. Preview: `docs/tasks/TASK-003.md`. |

## Allowed statuses
`BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.

## Operating rule
- PM owns priority and status changes.
- AI Developers may start only `READY` tasks.
- A task normally moves `READY -> IN_PROGRESS -> REVIEW -> DONE`.
- Team Lead may move `REVIEW -> CHANGES_REQUESTED` until acceptance criteria are satisfied.
- Do not open many dependent tasks as READY merely to keep an agent busy; correctness of the accepted foundation takes priority over speculative parallelism.
