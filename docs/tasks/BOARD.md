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
| TASK-008 | AI Proposal frontend workspace | CHANGES_REQUESTED | PR #20 head `41a6b7f`. Earlier state/retry blockers fixed; remaining list-load failure is falsely rendered as empty Proposal history instead of error/retry. |
| TASK-009 | AI Proposal generation job integration | BLOCKED | Issue #15. TASK-010 is now accepted; remaining dependency is TASK-008 acceptance plus PM final integration-contract/live-provider gate. |
| TASK-010 | Durable job execution foundation | DONE | Accepted/squash-merged PR #21 as `f731f4b9...`; lease heartbeat/loss, exhausted attempts, retry cap, JSON-object boundaries and real PostgreSQL lifecycle coverage accepted; CI #109 green. |
| TASK-011 | Script domain, persistence and approval API | CHANGES_REQUESTED | PR #22 head `6fb2910` is code-approved/CI #110 green; branch still has an old base SHA and must sync latest `develop` + rerun CI before final merge. |
| TASK-012 | Script generation engine | READY | Issue #23. Provider-neutral `script_v1` candidate generation against frozen `SCRIPT_GENERATION_V1`; owns only `scriptgeneration/**`. |

## Active parallel wave — WAVE-F1-D transition
Frozen contracts:
- `docs/contracts/AI_PROPOSAL_V1.md`
- `docs/contracts/JOB_EXECUTION_V1.md`
- `docs/contracts/SCRIPT_V1.md`
- `docs/contracts/SCRIPT_GENERATION_V1.md`

Current three implementation slots:
- Dev A — TASK-008: `CHANGES_REQUESTED` on PR #20; fix Project-success/Proposal-list-failure so it renders recoverable error/retry rather than false empty history.
- Dev B — TASK-012: `READY`; claim atomically and implement Script generation engine in a new dedicated worktree.
- Dev C — TASK-011: `CHANGES_REQUESTED` on PR #22; code is accepted, sync latest `develop`, preserve TASK-010 + TASK-011 behavior if any shared resolution is needed, rerun CI, then final merge check.

TASK-010's implementation worktree should be cleaned up after confirming the merged commit, before that developer claims TASK-012.

## Isolation / merge rules
- **Each implementation slot must run in its own dedicated Git worktree. The shared/control checkout remains on `develop`; agents must not switch that folder among task branches.**
- TASK-008 owns Proposal frontend only; no backend edits.
- TASK-011 owns Script persistence/API + migration `0005`; it must preserve accepted TASK-010 jobs behavior during latest-base sync.
- TASK-012 owns `apps/api/internal/scriptgeneration/**` only; no persistence/jobs/HTTP/frontend/provider-SDK changes.
- TASK-009 becomes eligible only after TASK-008 is accepted and PM freezes its feature-specific generation-job request/payload/result contract against accepted TASK-010.
- TASK-012 is independent from TASK-011 implementation because both build against frozen Script contracts and have disjoint primary write surfaces.
- ADR 0005 forbids long provider calls inside blocking HTTP requests; Proposal/Script generation integration must use durable jobs.
- AI Proposal is not production-complete until a live provider/BYOK capability is accepted; deterministic fakes remain test-only.

## Product progress checkpoint
Accepted durable product capabilities currently cover:
- runnable technical foundation and CI/local infrastructure;
- Project persistence and owner boundary;
- creator-facing Creative Brief persistence/workspace;
- provider-neutral text-generation capability boundary;
- AI Proposal persistence/versioning/approval backend;
- AI Proposal provider-neutral generation engine;
- generic durable PostgreSQL job/lease/retry execution foundation.

Still incomplete before the Proposal stage is creator-usable end to end:
- TASK-008 Proposal frontend final acceptance;
- TASK-009 Proposal durable generation-job integration;
- live provider/BYOK capability.

Script Stage 5–6 is now advancing in parallel:
- TASK-011 persistence/API is code-approved but awaiting latest-`develop` sync + re-CI;
- TASK-012 generation engine is READY independently.

Downstream work remains substantial: Script frontend/integration, Scene Plan, media/audio acquisition/generation, Scene Editor, render/export and publishing/channel management.

## Allowed statuses
`BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.

## Operating rules
- PM owns priority, dependency graph, wave composition and READY/BLOCKED/DONE transitions.
- AI Developers may start only tasks explicitly marked `READY` whose dependencies are satisfied.
- **One implementation task = one dedicated Git worktree.** The shared/control checkout stays on `develop` while concurrent agents are active; never use `git switch` there to move among task branches.
- Maximum concurrent implementation worktrees normally equals the configured AI developer slots (currently 3). Do not create speculative spare worktrees.
- When a task is merged/DONE, clean up its worktree before that dev claims the next task unless the worktree is temporarily retained for explicit recovery data.
- Parallel agents inspect remote branches, PRs, local task branches and `git worktree list --porcelain` before claiming.
- A new remote task branch must be claimed by atomically creating the previously absent GitHub ref at the selected latest `origin/develop` SHA. A plain same-base `git push` is not an exclusive lock.
- Only after the remote claim succeeds does the agent create/attach its dedicated local worktree and begin implementation there.
- If a claim race is lost, the losing agent never overwrites/deletes the winning branch; it re-fetches and selects another eligible READY task.
- Review fixes stay on the original branch/PR and its dedicated worktree.
- Every implementation task follows `docs/engineering/TDD_PROTOCOL.md`; RED -> GREEN -> REFACTOR evidence must be truthful.
- A task normally moves `READY -> IN_PROGRESS -> REVIEW -> DONE`; Team Lead may move `REVIEW -> CHANGES_REQUESTED` until acceptance criteria are satisfied.
- Do not create parallelism by splitting tightly coupled work after implementation begins. Prefer contract-first tasks with isolated write paths and isolated worktrees.
