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
| TASK-007 | AI Proposal generation engine | DONE | Accepted and squash-merged via PR #16 after fixing in-flight context cancellation/deadline propagation; CI #74 green. |
| TASK-008 | AI Proposal frontend workspace | DONE | Accepted/squash-merged PR #20 as `36418b8e...`; final list-load recovery regression fixed and CI #122 green. |
| TASK-009 | AI Proposal generation job integration | CHANGES_REQUESTED | PR #28 head `f55917aa`; CI #141 green but internal job metadata leak, request-id race/replay semantics, removed TASK-008 regressions and succeeded-job UI recovery must be fixed. |
| TASK-010 | Durable job execution foundation | DONE | Accepted/squash-merged PR #21 as `f731f4b9...`; lease heartbeat/loss, exhausted attempts, retry cap, JSON-object boundaries and real PostgreSQL lifecycle coverage accepted; CI #109 green. |
| TASK-011 | Script domain, persistence and approval API | DONE | Accepted/squash-merged PR #22 as `87c9849e...`; Unicode character semantics, latest-develop sync and CI #123 accepted. |
| TASK-012 | Script generation engine | DONE | Accepted/squash-merged PR #24 as `0a3d2fb9...`; strict `script_v1`, long-form/Unicode/context/immutability coverage accepted; CI #128 green. |
| TASK-013 | Live OpenAI-compatible text provider adapter foundation | DONE | Accepted/squash-merged PR #29 as `177e78fc...`; secret-safe bounded OpenAI-compatible adapter, deterministic registration and CI #142 accepted. |
| TASK-014 | Scene Plan generation engine | DONE | Accepted/squash-merged PR #27 as `6b5c9d3c...`; approved narration preservation, strict scene validation and CI #140 accepted. |

## Active parallel wave — WAVE-F1-E review/fix
Frozen contracts:
- `docs/contracts/AI_PROPOSAL_JOB_V1.md`
- `docs/contracts/OPENAI_COMPAT_TEXT_PROVIDER_V1.md`
- `docs/contracts/SCENE_PLAN_GENERATION_V1.md`
- plus previously accepted/frozen Proposal, Jobs and Script contracts.

Current implementation slots:
- Dev A — TASK-009 `CHANGES_REQUESTED` on PR #28; continue only in the same dedicated worktree/PR and fix the four recorded review blockers.
- Dev B — available after TASK-013 DONE; clean merged TASK-013 worktree before claiming future work.
- Dev C — available after TASK-014 DONE; clean merged TASK-014 worktree before claiming future work.

Do not create micro-tasks merely to fill Dev B/C. The next wave should be frozen as substantial capabilities with non-overlapping primary write surfaces.

## TASK-009 current review blockers
1. `source_generation_job_id` must remain internal persistence metadata and never appear in public Proposal JSON.
2. `request_id` replay/conflict behavior must remain idempotent even if current Brief/provider state changes and must deterministically reject conflicting concurrent reuse after duplicate enqueue races.
3. Restore the TASK-008 regressions removed during the frontend test-harness refactor.
4. A succeeded durable generation job whose Proposal list/version follow-up load transiently fails must remain recoverable without offering a Regenerate action that starts another AI job.

## Planned next wave — do not claim yet
Freeze contracts/write surfaces before changing these to READY:
- **Secure BYOK credentials + runtime provider registration/settings**: owner-scoped credential lifecycle, secret-safe persistence/use, provider settings/catalog integration and actual TASK-013 adapter registration. This is one meaningful product capability, not a tiny `main.go` wiring patch.
- **Script durable generation integration**: combine TASK-010 jobs + TASK-012 generation engine + TASK-011 idempotent Script draft persistence using the accepted async pattern.
- **Script creator workspace**: history/edit/stale/approval plus durable generation action/status UI; schedule separately from other frontend-heavy integration to avoid router/i18n collisions.
- **Scene Plan persistence/integration**: versioning/editing/source tracking and durable generation around accepted TASK-014 before asset/media generation begins.

## Isolation / merge rules
- **Each implementation task must run in its own dedicated Git worktree. The shared/control checkout remains on `develop`; agents must not switch that folder among task branches.**
- Maximum concurrent implementation worktrees normally equals the configured AI developer slots (currently 3). Do not create speculative spare worktrees.
- Review fixes stay on the original branch/PR and its dedicated worktree.
- Merged task worktrees should be removed before that developer claims a new task unless explicitly retained for recovery data.
- A new remote task branch must be claimed by atomically creating the previously absent GitHub ref at the selected latest `origin/develop` SHA; plain same-base `git push` is not an exclusive lock.
- Every implementation task follows `docs/engineering/TDD_PROTOCOL.md`; RED -> GREEN -> REFACTOR evidence must be truthful.
- Do not create parallelism by splitting tightly coupled work after implementation begins. Prefer contract-first tasks with isolated write surfaces and enough product/engineering value to justify a PR.

## Product progress checkpoint
Accepted durable product capabilities currently cover:
- runnable technical foundation and CI/local infrastructure;
- Project persistence and owner boundary;
- creator-facing Creative Brief persistence/workspace;
- provider-neutral text-generation capability boundary;
- AI Proposal persistence/versioning/approval backend;
- AI Proposal provider-neutral generation engine;
- AI Proposal creator-facing history/edit/stale/approval frontend workspace;
- generic durable PostgreSQL job/lease/retry execution foundation;
- Script persistence/versioning/approval API;
- provider-neutral Script generation engine;
- live OpenAI-compatible provider adapter foundation;
- provider-neutral Scene Plan generation engine with approved narration preservation.

Still incomplete before AI Proposal is creator-usable end to end:
- TASK-009 review fixes and acceptance;
- secure live-provider/BYOK runtime registration.

Downstream remains substantial: Script durable integration/workspace, Scene Plan persistence/workspace, media/audio acquisition/generation, Scene Editor, render/export and publishing/channel management.

## Allowed statuses
`BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.
