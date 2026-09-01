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
| TASK-009 | AI Proposal generation job integration | BLOCKED | All implementation dependencies are accepted. Remaining PM gate: freeze feature-specific job/API/idempotency contract and live-provider/BYOK boundary before READY. |
| TASK-010 | Durable job execution foundation | DONE | Accepted/squash-merged PR #21 as `f731f4b9...`; lease heartbeat/loss, exhausted attempts, retry cap, JSON-object boundaries and real PostgreSQL lifecycle coverage accepted; CI #109 green. |
| TASK-011 | Script domain, persistence and approval API | DONE | Accepted/squash-merged PR #22 as `87c9849e...`; Unicode character semantics, latest-develop sync and CI #123 accepted. |
| TASK-012 | Script generation engine | IN_PROGRESS | Issue #23. Canonical branch exists and is claimed; no PR yet. Owns only `scriptgeneration/**`. |

## Active parallel wave — WAVE-F1-D transition
Frozen contracts:
- `docs/contracts/AI_PROPOSAL_V1.md`
- `docs/contracts/JOB_EXECUTION_V1.md`
- `docs/contracts/SCRIPT_V1.md`
- `docs/contracts/SCRIPT_GENERATION_V1.md`

Current implementation slots:
- Dev A — available after TASK-008 DONE; do not claim BLOCKED TASK-009 until PM changes it to READY.
- Dev B — TASK-012 `IN_PROGRESS` on canonical branch `feature/TASK-012-script-generation` in its dedicated worktree.
- Dev C — available after TASK-011 DONE.

Merged TASK-008/TASK-011 worktrees should be cleaned up before those developers claim future tasks.

## Isolation / merge rules
- **Each implementation slot must run in its own dedicated Git worktree. The shared/control checkout remains on `develop`; agents must not switch that folder among task branches.**
- Maximum concurrent implementation worktrees normally equals the configured AI developer slots (currently 3). Do not create speculative spare worktrees.
- TASK-012 owns `apps/api/internal/scriptgeneration/**` only; no persistence/jobs/HTTP/frontend/provider-SDK changes.
- TASK-009 remains unclaimable until PM freezes its Proposal generation-job integration contract and marks it READY.
- ADR 0005 forbids long provider calls inside blocking HTTP requests; Proposal/Script generation integration must use durable jobs.
- AI Proposal is not production-complete until a live provider/BYOK capability is accepted; deterministic fakes remain test-only.

## Product progress checkpoint
Accepted durable product capabilities now cover:
- runnable technical foundation and CI/local infrastructure;
- Project persistence and owner boundary;
- creator-facing Creative Brief persistence/workspace;
- provider-neutral text-generation capability boundary;
- AI Proposal persistence/versioning/approval backend;
- AI Proposal provider-neutral generation engine;
- AI Proposal creator-facing history/edit/stale/approval frontend workspace;
- generic durable PostgreSQL job/lease/retry execution foundation;
- Script persistence/versioning/approval API from approved Proposal versions.

Still incomplete before AI Proposal generation is creator-usable end to end:
- TASK-009 durable Proposal generation-job integration;
- live provider/BYOK capability.

Script Stage 5–6 is advancing:
- TASK-011 persistence/API is DONE;
- TASK-012 generation engine is IN_PROGRESS;
- Script frontend + durable generation integration remain future work.

Downstream work remains substantial: Scene Plan, media/audio acquisition/generation, Scene Editor, render/export and publishing/channel management.

## Allowed statuses
`BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.

## Operating rules
- PM owns priority, dependency graph, wave composition and READY/BLOCKED/DONE transitions.
- AI Developers may start only tasks explicitly marked `READY` whose dependencies are satisfied.
- **One implementation task = one dedicated Git worktree.** The shared/control checkout stays on `develop` while concurrent agents are active; never use `git switch` there to move among task branches.
- When a task is merged/DONE, clean up its worktree before that dev claims the next task unless the worktree is temporarily retained for explicit recovery data.
- Parallel agents inspect remote branches, PRs, local task branches and `git worktree list --porcelain` before claiming.
- A new remote task branch must be claimed by atomically creating the previously absent GitHub ref at the selected latest `origin/develop` SHA. A plain same-base `git push` is not an exclusive lock.
- Only after the remote claim succeeds does the agent create/attach its dedicated local worktree and begin implementation there.
- If a claim race is lost, the losing agent never overwrites/deletes the winning branch; it re-fetches and selects another eligible READY task.
- Review fixes stay on the original branch/PR and its dedicated worktree.
- Every implementation task follows `docs/engineering/TDD_PROTOCOL.md`; RED -> GREEN -> REFACTOR evidence must be truthful.
- A task normally moves `READY -> IN_PROGRESS -> REVIEW -> DONE`; Team Lead may move `REVIEW -> CHANGES_REQUESTED` until acceptance criteria are satisfied.
- Do not create parallelism by splitting tightly coupled work after implementation begins. Prefer contract-first tasks with isolated write paths and isolated worktrees.
