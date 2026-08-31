# SynVideo Task Board

Current milestone: `F1 — CREATIVE WORKFLOW`

PM plans ahead. AI Developers may start only tasks explicitly marked `READY`, and parallel tasks must follow `docs/engineering/PARALLEL_WORK_PROTOCOL.md`.

| ID | Task | Status | Spec / Notes |
|---|---|---|---|
| BOOT-001 | Initialize repository and establish `develop` | DONE | `main` contains the initial repository commit; PM/product scaffold lives on `develop`. |
| AUDIT-001 | Audit pre-existing codebase | CANCELLED | Not applicable: there is no pre-existing application code. |
| PLAN-001 | Define Foundation milestone and first implementation task set | DONE | Initial architecture baseline and first dependency chain are defined. |
| TASK-001 | Technical foundation and runnable project skeleton | DONE | Accepted and squash-merged via PR #3 after Team Lead review and green CI. |
| TASK-002 | Project domain and persistence foundation | DONE | Accepted and squash-merged via PR #8 after Team Lead review, green CI and truthful TDD evidence. |
| TASK-003 | Creative Brief backend and persistence | DONE | Accepted and squash-merged via PR #9 after Team Lead review, real PostgreSQL owner/concurrency tests and green CI. |
| TASK-004 | Creative Brief frontend workspace | CHANGES_REQUESTED | PR #10 reviewed by Team Lead. Fix dirty/saved-state correctness, real ProjectDetail route regression coverage, run real TASK-003 backend smoke, and obtain green CI on corrected `develop` base. Keep fixes on `feature/TASK-004-creative-brief-web`. |
| TASK-005 | AI provider capability and text-generation contracts | CHANGES_REQUESTED | PR #11 reviewed by Team Lead. Fix registry metadata aliasing/thread-safety, make provider errors safe-by-default against secret-bearing raw messages, deep-snapshot fake requests, and obtain green CI on corrected `develop` base. Keep fixes on `feature/TASK-005-ai-provider-contracts`. |

## Active parallel wave — WAVE-F1-A
TASK-003 is complete. Current remaining work:
- Dev B — TASK-004: `CHANGES_REQUESTED`, fix PR #10 on the same branch under TDD and rerun real backend integration.
- Dev C — TASK-005: `CHANGES_REQUESTED`, fix PR #11 on the same branch under TDD, rerun `-race` verification and green PR CI.

Do not invent a third task merely to fill capacity. PM opens additional parallel work only when dependencies and write surfaces are genuinely independent.

## Allowed statuses
`BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.

## Operating rules
- PM owns priority, dependency graph, wave composition and READY/BLOCKED/DONE transitions.
- AI Developers may start only `READY` tasks whose dependencies are satisfied.
- Parallel agents use the canonical remote task branch as a claim/lock: fetch first, skip tasks whose remote branch/PR already exists, create the exact branch from latest `origin/develop`, and push it immediately before implementation.
- Every implementation task follows `docs/engineering/TDD_PROTOCOL.md`; RED -> GREEN -> REFACTOR evidence must be truthful and coverage-only additions must not be misrepresented as failing RED behavior.
- A task normally moves `READY -> IN_PROGRESS -> REVIEW -> DONE`.
- Team Lead may move `REVIEW -> CHANGES_REQUESTED` until acceptance criteria are satisfied.
- Do not create parallelism by splitting tightly coupled work after implementation has already begun. Prefer contract-first tasks with isolated write paths.
