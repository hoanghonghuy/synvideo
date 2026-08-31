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
| TASK-004 | Creative Brief frontend workspace | DONE | Accepted and squash-merged via PR #10 after Team Lead review, real backend smoke, dirty/saved/error/stale regression coverage and green CI. |
| TASK-005 | AI provider capability and text-generation contracts | DONE | Accepted and squash-merged via PR #11 after Team Lead review, mutation-isolation fixes, safe-by-default provider errors, deep-snapshot fake requests, race tests and green CI. |
| TASK-006 | AI Proposal domain, persistence and approval API | DONE | Accepted and squash-merged via PR #17 after Team Lead review, real PostgreSQL concurrency/owner-isolation coverage and green CI. |
| TASK-007 | AI Proposal generation engine | DONE | Accepted and squash-merged via PR #16 after fixing in-flight context cancellation/deadline propagation; CI run #74 green on `develop`. |
| TASK-008 | AI Proposal frontend workspace | READY | Issue #14. Owns Proposal web feature + minimal route/i18n/navigation; TASK-006 is now merged, so final real backend smoke can run when implementation is ready. |
| TASK-009 | AI Proposal generation job integration | BLOCKED | Issue #15. Async/durable integration gate after TASK-006/007/008: generation job -> validated candidate -> persist draft -> frontend status -> approve. |

## Active parallel wave — WAVE-F1-B
Frozen contracts:
- `docs/contracts/AI_PROPOSAL_V1.md`
- `docs/contracts/AI_PROPOSAL_GENERATION_V1.md`

Current wave state:
- Dev A — TASK-006: DONE via PR #17.
- Dev B — TASK-007: DONE via PR #16.
- Dev C — TASK-008: READY / independent frontend workspace; TASK-006 backend integration gate is available.

Isolation / merge rules:
- TASK-006 and TASK-007 are accepted and merged.
- TASK-008 develops against the frozen Proposal HTTP/resource contract and may run its final real backend smoke against merged TASK-006.
- TASK-009 is the only task that connects generation engine -> Proposal CreateDraft -> async generation-job HTTP/status -> frontend Generate/Regenerate. It stays BLOCKED until TASK-008 is accepted and PM freezes the job contract.
- ADR 0005 forbids treating provider generation as a long blocking HTTP request; TASK-009 must use an explicit job boundary.
- Completing TASK-009 proves workflow integration. Do not call AI Proposal production-complete unless a live provider/BYOK capability is also accepted; deterministic fakes are for tests only.

See `docs/tasks/WAVE_F1_B.md` for the wave integration plan.

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
