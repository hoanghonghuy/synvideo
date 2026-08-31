# SynVideo Task Board

Current milestone: `F0 — TECHNICAL FOUNDATION` transitioning to `F1 — CREATIVE WORKFLOW`

PM plans ahead. AI Developers may start only tasks explicitly marked `READY`, and parallel tasks must follow `docs/engineering/PARALLEL_WORK_PROTOCOL.md`.

| ID | Task | Status | Spec / Notes |
|---|---|---|---|
| BOOT-001 | Initialize repository and establish `develop` | DONE | `main` contains the initial repository commit; PM/product scaffold lives on `develop`. |
| AUDIT-001 | Audit pre-existing codebase | CANCELLED | Not applicable: there is no pre-existing application code. |
| PLAN-001 | Define Foundation milestone and first implementation task set | DONE | Initial architecture baseline and first dependency chain are defined. |
| TASK-001 | Technical foundation and runnable project skeleton | DONE | Accepted and squash-merged via PR #3 after Team Lead review and green CI. |
| TASK-002 | Project domain and persistence foundation | CHANGES_REQUESTED | PR #8 reviewed by Team Lead. Keep fixes on `feature/TASK-002-project-persistence`; remaining fixes are test-first under the adopted TDD protocol. |
| TASK-003 | Creative Brief backend and persistence | BLOCKED | Issue #5. WAVE-F1-A. Opens after TASK-002 acceptance. Owns backend Creative Brief domain/API/persistence. |
| TASK-004 | Creative Brief frontend workspace | BLOCKED | Issue #6. WAVE-F1-A. Opens after TASK-002 acceptance; consumes frozen Creative Brief V1 contract; final integration gate TASK-003. |
| TASK-005 | AI provider capability and text-generation contracts | BLOCKED | Issue #7. WAVE-F1-A. Isolated provider package; opens with the wave after TASK-002 acceptance. |

## Next parallel wave — WAVE-F1-A
When TASK-002 is accepted, PM may move TASK-003, TASK-004 and TASK-005 to `READY` **together**.

The wave is deliberately split by write surface:
- Dev A — TASK-003: backend Creative Brief API/persistence.
- Dev B — TASK-004: frontend Creative Brief workspace against frozen `docs/contracts/CREATIVE_BRIEF_V1.md`.
- Dev C — TASK-005: isolated AI provider capability/text boundary.

TASK-003 and TASK-004 share a contract, not implementation files. TASK-005 is merge-order independent from both. TASK-004 may develop/review in parallel but final acceptance includes smoke/integration after TASK-003 merges.

## Allowed statuses
`BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.

## Operating rules
- PM owns priority, dependency graph, wave composition and READY/BLOCKED/DONE transitions.
- AI Developers may start only `READY` tasks whose dependencies are satisfied.
- Parallel agents use the canonical remote task branch as a claim/lock: fetch first, skip tasks whose remote branch/PR already exists, create the exact branch from latest `origin/develop`, and push it immediately before implementation.
- Every implementation task follows `docs/engineering/TDD_PROTOCOL.md` unless a justified exception is documented. TASK-002 is the one grandfathered in-progress task; no fake historical TDD evidence.
- A task normally moves `READY -> IN_PROGRESS -> REVIEW -> DONE`.
- Team Lead may move `REVIEW -> CHANGES_REQUESTED` until acceptance criteria are satisfied.
- Do not create parallelism by splitting tightly coupled work after implementation has already begun. Prefer contract-first tasks with isolated write paths.
