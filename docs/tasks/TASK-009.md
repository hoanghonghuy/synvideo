# TASK-009 — AI Proposal generation job integration

Status: READY
Milestone: F1 Creative Workflow
Depends on: TASK-006, TASK-007, TASK-008 and TASK-010 accepted
Wave: WAVE-F1-E
Branch: `feature/TASK-009-ai-proposal-integration`
Base: `develop`

## Goal
Connect the accepted Proposal persistence/API, Proposal generation engine, durable job foundation and Proposal frontend into a durable creator-facing Generate/Regenerate flow.

The implementation must satisfy `docs/contracts/AI_PROPOSAL_JOB_V1.md` exactly.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/decisions/0005-technical-baseline.md`
- `docs/contracts/JOB_EXECUTION_V1.md`
- `docs/contracts/AI_PROPOSAL_V1.md`
- `docs/contracts/AI_PROPOSAL_GENERATION_V1.md`
- `docs/contracts/AI_PROPOSAL_JOB_V1.md`
- accepted TASK-006/007/008/010 implementations.

## Frozen feature contract
`AI_PROPOSAL_JOB_V1.md` is authoritative. Do not silently change HTTP/job/idempotency semantics from this branch.

Key frozen decisions:
- `GET /api/v1/ai/text-generation-options` exposes only safe registered text models;
- `POST /api/v1/projects/{project_id}/creative-proposal-generations` accepts `request_id`, `provider_id`, `model_id` and returns `202`;
- client `request_id` is the durable job ID and makes ambiguous HTTP retries idempotent;
- job kind is `creative_proposal_generation_v1`;
- job payload snapshots request-time Project + Creative Brief input and never contains credentials;
- status is capability-specific GET backed by generic owner/project job isolation;
- migration `0006` adds internal `source_generation_job_id` Proposal metadata with DB uniqueness;
- persistence keyed by job ID is idempotent across the crash window after Proposal commit but before generic job success;
- generic success result contains only `proposal_version`;
- production composition never registers the deterministic fake provider;
- when no live provider is registered the product shows a disabled/localized configuration state rather than fake AI success.

## Primary ownership
- `apps/api/internal/proposalgenerationjob/**` (or an equivalently cohesive feature package);
- Proposal-generation-specific PostgreSQL integration/idempotent persistence changes;
- `apps/api/internal/migrations/sql/0006_*` only;
- Proposal-generation-specific HTTP handler/tests;
- `apps/web/src/features/creative-proposal/**` generation status/action extensions.

## Allowed shared hotspots
Only minimal composition/registration changes required for this feature:
- `apps/api/cmd/api/main.go`;
- `apps/api/internal/httpserver/server.go`;
- existing central frontend locale/style files if genuinely required.

Do not perform unrelated refactors in shared hotspots.

## Reserved / do not touch
- `apps/api/internal/providers/openaicompat/**` — TASK-013;
- `apps/api/internal/sceneplangeneration/**` — TASK-014;
- Script persistence/generation behavior;
- generic jobs schema/lifecycle unless a real frozen-contract defect is discovered and escalated;
- Scene Plan/media/editor/render/publishing code.

## Backend scope
- owner-scoped Project + current Creative Brief request-time load;
- safe provider/model catalog from accepted registry metadata;
- idempotent enqueue using caller `request_id` as job ID;
- strict versioned snapshot payload;
- Proposal generation job handler registered with the generic executor;
- generic executor lifecycle actually composed into the running API process with graceful shutdown;
- engine error -> retryable/terminal job mapping per frozen contract;
- idempotent Proposal creation keyed by `source_generation_job_id`;
- capability-specific status projection hiding raw payload/lease/provider internals;
- safe behavior when production registry has zero live providers.

## Frontend scope
- safe provider/model selects from catalog endpoint;
- disabled localized state when no live model exists;
- Generate/Regenerate on existing Proposal workspace;
- block generation while current Proposal form is dirty;
- generate fresh `request_id` for each explicit new generation intent;
- show queued/running/succeeded/failed state without unmounting or corrupting current Proposal;
- resume an active non-terminal generation across page refresh/navigation using route/session job identity;
- on success refresh history and open returned Proposal version;
- terminal retry is explicit and creates a new request/job;
- never render deterministic fake output as production success.

## TDD plan
Truthful RED -> GREEN -> REFACTOR must cover at least:
1. current owner-scoped Project/Brief snapshot and exact source Brief revision;
2. same `request_id` HTTP retry returns same job;
3. conflicting request ID reuse -> `GENERATION_REQUEST_CONFLICT`;
4. missing Brief -> `CREATIVE_BRIEF_REQUIRED` without provider call;
5. no credentials in request/job payload/result/presentation errors;
6. provider/model catalog contains only text-capable registered models;
7. handler persists nothing on provider failure/invalid output;
8. retryable provider failure enters generic retry lifecycle;
9. invalid output is terminal for that job;
10. real PostgreSQL crash/reclaim simulation proves one job creates at most one Proposal draft;
11. distinct regenerate jobs preserve approved history + one-active-draft invariant;
12. owner/project status non-disclosure;
13. dirty frontend blocks generation;
14. pending/failure leaves currently-reviewed Proposal intact;
15. refresh resumes a nonterminal job;
16. success opens exact returned Proposal version;
17. production composition has no fake provider registration;
18. full local smoke through Proposal approval.

## Acceptance criteria
- [ ] Frozen `AI_PROPOSAL_JOB_V1` is implemented without contract drift.
- [ ] Provider execution never blocks the initiating HTTP request.
- [ ] Generic TASK-010 jobs/executor is reused and wired into runtime; no second job engine/table.
- [ ] Same HTTP `request_id` is idempotent.
- [ ] One successful durable job creates at most one Proposal version even across commit->crash->reclaim.
- [ ] Approved Proposal history remains immutable.
- [ ] Safe catalog/status APIs expose no secret/job internals.
- [ ] Production composition never exposes fake provider as live AI.
- [ ] UI preserves dirty/current Proposal state and durable job state correctly.
- [ ] Real PostgreSQL integration, frontend tests, full CI and local smoke are green.
- [ ] TDD evidence is truthful.

## Live-provider boundary
This task may merge with an empty production text-provider catalog. That is an explicit safe state, not product completion.

TASK-013 builds the live OpenAI-compatible adapter foundation independently. A later secure BYOK/runtime-registration task must be accepted before the Proposal AI stage is considered production-complete.

## Worktree / claim
Atomically create the previously absent remote branch ref, then create one dedicated TASK-009 worktree. Never implement in the shared `develop` checkout.

Do not self-merge or self-mark DONE.
