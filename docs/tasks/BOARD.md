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
| TASK-009 | AI Proposal generation job integration | READY | Issue #15. Frozen `AI_PROPOSAL_JOB_V1`: async `202`, request/job idempotency, request-time Brief snapshot, runtime generic executor, DB exactly-once Proposal persistence and creator job UI. |
| TASK-010 | Durable job execution foundation | DONE | Accepted/squash-merged PR #21 as `f731f4b9...`; lease heartbeat/loss, exhausted attempts, retry cap, JSON-object boundaries and real PostgreSQL lifecycle coverage accepted; CI #109 green. |
| TASK-011 | Script domain, persistence and approval API | DONE | Accepted/squash-merged PR #22 as `87c9849e...`; Unicode character semantics, latest-develop sync and CI #123 accepted. |
| TASK-012 | Script generation engine | DONE | Accepted/squash-merged PR #24 as `0a3d2fb9...`; strict `script_v1`, long-form/Unicode/context/immutability coverage accepted; CI #128 green. |
| TASK-013 | Live OpenAI-compatible text provider adapter foundation | READY | Issue #25. Isolated `providers/openaicompat/**`; live HTTP adapter, safe credential/error/resource boundaries, registration factory; no runtime/httpserver/web wiring. |
| TASK-014 | Scene Plan generation engine | READY | Issue #26. Isolated `sceneplangeneration/**`; approved Script narration must be segmented without silent rewrite; no persistence/jobs/HTTP/frontend/media work. |

## Active parallel wave — WAVE-F1-E
Frozen contracts:
- `docs/contracts/AI_PROPOSAL_JOB_V1.md`
- `docs/contracts/OPENAI_COMPAT_TEXT_PROVIDER_V1.md`
- `docs/contracts/SCENE_PLAN_GENERATION_V1.md`
- plus previously accepted/frozen Proposal, Jobs and Script contracts.

Current three implementation slots:
- Dev A — TASK-009 `READY`: full-stack durable Proposal generation integration. Owns Proposal-generation integration/migration `0006`/feature HTTP + existing Proposal workspace extensions and minimal application composition.
- Dev B — TASK-013 `READY`: live OpenAI-compatible provider adapter foundation. Owns only `apps/api/internal/providers/openaicompat/**` and local deterministic tests.
- Dev C — TASK-014 `READY`: Scene Plan generation engine. Owns only `apps/api/internal/sceneplangeneration/**` and task tests.

All three branches are claimed only by atomic remote create-ref and each runs in its own dedicated worktree.

## Why these three are safe to parallelize
These are intentionally **not** artificial micro-tasks:
- TASK-009 delivers the missing end-to-end durable Proposal generation workflow and contains real PostgreSQL idempotency/runtime/frontend work.
- TASK-013 delivers a complete live-network provider adapter boundary with security, cancellation, resource limits and deterministic upstream tests.
- TASK-014 delivers the first Scene Plan generation capability with a strong approved-Script preservation invariant.

Primary write surfaces do not overlap.

TASK-013 deliberately stops before runtime/BYOK UI wiring because that wiring would collide with TASK-009's `main/httpserver/frontend` hotspots. TASK-014 deliberately stops before Scene Plan persistence/HTTP for the same reason.

## Isolation / merge rules
- **Each implementation task must run in its own dedicated Git worktree. The shared/control checkout remains on `develop`; agents must not switch that folder among task branches.**
- Maximum concurrent implementation worktrees normally equals the configured AI developer slots (currently 3). Do not create speculative spare worktrees.
- TASK-009 must not modify `providers/openaicompat/**` or `sceneplangeneration/**`.
- TASK-013 must not modify `main.go`, `httpserver/**`, `apps/web/**`, jobs, migrations or feature domains.
- TASK-014 must not modify `main.go`, `httpserver/**`, `apps/web/**`, jobs, persistence/migrations or media generation.
- ADR 0005 forbids long provider calls inside blocking HTTP requests; Proposal/Script/Scene generation integrations must use durable jobs where orchestration is required.
- Deterministic fake providers remain test-only and must never appear as production creator options.

## Planned next wave — do not claim yet
After WAVE-F1-E acceptance, PM should open substantial follow-on tasks rather than tiny wiring patches:
- **Secure BYOK credentials + runtime provider registration/settings**: owner-scoped credential lifecycle, secret-safe storage/use, provider settings/catalog integration and actual registration of TASK-013 adapter. Keep it as one meaningful product capability rather than a 1–2 line `main.go` wiring task.
- **Script durable generation integration**: combine accepted TASK-010 jobs + TASK-012 engine + TASK-011 idempotent Script draft persistence, using the accepted TASK-009 async pattern where appropriate.
- **Script creator workspace**: version history/edit/stale/approval + durable generation status/action UI, contract-first against accepted backend APIs; schedule to avoid shared router/i18n hotspots with other frontend integration work.
- **Scene Plan persistence/integration**: version/edit/stale/source tracking around TASK-014 before asset/media generation begins.

Do not mark these READY merely to fill slots; freeze their contracts and write surfaces first.

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
- Script persistence/versioning/approval API from approved Proposal versions;
- provider-neutral Script generation engine.

WAVE-F1-E now targets:
- creator-usable durable Proposal generation integration;
- first production-capable live text adapter boundary;
- Scene Plan generation foundation.

AI Proposal still is not production-complete until secure live provider/BYOK runtime registration is accepted.

Downstream remains: Script durable integration/workspace, Scene Plan persistence/workspace, media/audio acquisition/generation, Scene Editor, render/export and publishing/channel management.

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
