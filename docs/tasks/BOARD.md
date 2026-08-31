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
| TASK-008 | AI Proposal frontend workspace | CHANGES_REQUESTED | PR #20. Recoverable version/reload GET failures currently clear dirty state too early and can leave blank/incoherent selection UI; fix under TDD on the same branch/worktree. |
| TASK-009 | AI Proposal generation job integration | BLOCKED | Issue #15. Requires TASK-008 + TASK-010 accepted; consumes durable jobs rather than implementing another queue. |
| TASK-010 | Durable job execution foundation | CHANGES_REQUESTED | PR #21. Needs executor lease heartbeat, exhausted-final-lease terminalization, repository max-attempt enforcement and JSON-object envelope validation. |
| TASK-011 | Script domain, persistence and approval API | READY | Issue #19. Starts Stage 5–6 Script persistence from approved Proposal + migration `0005`; branch `feature/TASK-011-script-persistence`. |

## Active parallel wave — WAVE-F1-C
Frozen contracts:
- `docs/contracts/AI_PROPOSAL_V1.md`
- `docs/contracts/JOB_EXECUTION_V1.md`
- `docs/contracts/SCRIPT_V1.md`

Current three implementation slots:
- Dev A — TASK-008: `CHANGES_REQUESTED` on PR #20; preserve dirty/recoverable state across failed version/reload GET and surface coherent visible errors.
- Dev B — TASK-010: `CHANGES_REQUESTED` on PR #21; fix long-handler lease heartbeat, final-attempt crash terminalization, durable max-attempt enforcement and JSON-object envelope semantics.
- Dev C — TASK-011: `READY`, Script persistence/approval foundation.

Isolation / merge rules:
- **Each implementation slot must run in its own dedicated Git worktree. The shared/control checkout remains on `develop`; agents must not switch that folder among task branches.**
- TASK-008 owns Proposal frontend only; no backend edits.
- TASK-010 owns `jobs/**`, job repository and migration `0004`; no Proposal/Script semantics or public generic job API.
- TASK-011 owns Script backend + migration `0005`; no jobs/frontend/provider generation work.
- TASK-008, TASK-010 and TASK-011 are merge-order independent by primary write surface.
- TASK-009 becomes eligible only after TASK-008 and TASK-010 are accepted and PM freezes its feature-specific generation-job request/payload/result contract.
- TASK-011 consumes already accepted Proposal approval semantics; it does not depend on live AI Proposal generation.
- ADR 0005 forbids long provider calls inside blocking HTTP requests; TASK-009 must use TASK-010 durable jobs.
- AI Proposal is not production-complete until a live provider/BYOK capability is accepted; deterministic fakes remain test-only.

See `docs/tasks/WAVE_F1_C.md` for migration ownership and integration gates.

## Product progress checkpoint
Accepted durable product capabilities currently cover:
- runnable technical foundation and CI/local infrastructure;
- Project persistence and owner boundary;
- creator-facing Creative Brief persistence/workspace;
- provider-neutral text-generation capability boundary;
- AI Proposal persistence/versioning/approval backend;
- AI Proposal provider-neutral generation engine.

Still incomplete before the Proposal stage is creator-usable end to end:
- TASK-008 Proposal frontend;
- TASK-010 durable async execution;
- TASK-009 generation-job integration;
- live provider/BYOK capability.

Downstream Creative Workflow stages after Proposal remain substantial: Script generation/frontend, Scene Plan, media/audio acquisition/generation, Scene Editor, render/export and publishing/channel management.

## Allowed statuses
`BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.

## Operating rules
- PM owns priority, dependency graph, wave composition and READY/BLOCKED/DONE transitions.
- AI Developers may start only tasks explicitly marked `READY` whose dependencies are satisfied.
- **One implementation task = one dedicated Git worktree.** The shared/control checkout should stay on `develop` while concurrent agents are active; never use `git switch` there to move among task branches.
- Parallel agents inspect remote branches, PRs, local task branches and `git worktree list --porcelain` before claiming. Existing canonical branch/PR/task worktree means the task is claimed or ambiguous and must not be silently taken over.
- A new remote task branch must be claimed by atomically **creating the previously absent GitHub ref** (create-ref/create-branch fail-if-exists) at the selected `origin/develop` SHA. A plain same-base `git push` is not sufficient as an exclusive lock.
- Only after the remote claim succeeds does the agent create/attach its dedicated local worktree and begin implementation there.
- If a claim race is lost, the losing agent never overwrites/deletes the winning remote branch; it re-fetches and selects another eligible READY task.
- Review fixes stay on the original branch/PR and its dedicated worktree.
- Every implementation task follows `docs/engineering/TDD_PROTOCOL.md`; RED -> GREEN -> REFACTOR evidence must be truthful and coverage-only additions must not be misrepresented as failing RED behavior.
- A task normally moves `READY -> IN_PROGRESS -> REVIEW -> DONE`.
- Team Lead may move `REVIEW -> CHANGES_REQUESTED` until acceptance criteria are satisfied.
- Do not create parallelism by splitting tightly coupled work after implementation has already begun. Prefer contract-first tasks with isolated write paths and isolated worktrees.
