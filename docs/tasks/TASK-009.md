# TASK-009 — AI Proposal generation job integration

Status: CHANGES_REQUESTED
Milestone: F1 Creative Workflow
Depends on: TASK-006, TASK-007, TASK-008 and TASK-010 accepted
Wave: WAVE-F1-E
Branch: `feature/TASK-009-ai-proposal-integration`
Base: `develop`
Active PR: #28
Current reviewed head: `f55917aa089553db5f32366e42fbf549c542ab13`

## Current review gate
CI #141 is green, but Team Lead review #5073433861 found four blockers to fix on the same branch/worktree:

1. `source_generation_job_id` is internal persistence metadata under frozen `AI_PROPOSAL_JOB_V1`, but the current public `CreativeProposal` JSON shape exposes it. Keep it internal (`json:"-"` or an equivalent private persistence mapping) and add HTTP regression coverage proving generated Proposal responses omit it.
2. `request_id` idempotency/conflict handling is not race-safe and is checked too late. Existing durable jobs must be validated/returned before current Brief/provider state can invalidate an idempotent replay. The duplicate-enqueue race must re-run the same kind/provider/model conflict validation; conflicting concurrent reuse must deterministically return `GENERATION_REQUEST_CONFLICT` rather than returning the winner job.
3. Restore the three previously accepted TASK-008 frontend regressions removed during the test-harness refactor: failed mutation after failed version switch; stale recovery reload failure preserving dirty edits; initial Proposal-version GET failure with visible retry.
4. Once a generation job is already `succeeded`, a transient failure refreshing/loading the created Proposal must not turn the UI into a Regenerate retry that launches another generation. Preserve the succeeded job/result and offer a safe load/recovery path until the created Proposal can be opened.

Required verification after fixes:
- focused service tests for normal replay + conflicting reuse + concurrent duplicate race;
- HTTP regression proving internal job metadata is absent;
- restored TASK-008 frontend regressions plus succeeded-job finalization failure recovery;
- real PostgreSQL exactly-once tests;
- full `make verify`, race where applicable and PR CI on the new head.

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
- accepted `apps/api/internal/providers/openaicompat/**` from TASK-013;
- accepted `apps/api/internal/sceneplangeneration/**` from TASK-014;
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
3. conflicting request ID reuse -> `GENERATION_REQUEST_CONFLICT`, including concurrent duplicate races;
4. missing Brief -> `CREATIVE_BRIEF_REQUIRED` without provider call for a genuinely new request;
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
16. success opens exact returned Proposal version and remains recoverable if follow-up list/version loading transiently fails;
17. production composition has no fake provider registration;
18. full local smoke through Proposal approval;
19. previously accepted TASK-008 load/mutation/dirty-state regressions remain covered.

## Acceptance criteria
- [ ] Frozen `AI_PROPOSAL_JOB_V1` is implemented without contract drift.
- [ ] Provider execution never blocks the initiating HTTP request.
- [ ] Generic TASK-010 jobs/executor is reused and wired into runtime; no second job engine/table.
- [ ] Same HTTP `request_id` is idempotent and conflicting reuse is deterministic under races.
- [ ] One successful durable job creates at most one Proposal version even across commit->crash->reclaim.
- [ ] `source_generation_job_id` remains internal and never appears in public Proposal JSON.
- [ ] Approved Proposal history remains immutable.
- [ ] Safe catalog/status APIs expose no secret/job internals.
- [ ] Production composition never exposes fake provider as live AI.
- [ ] UI preserves dirty/current Proposal state and durable succeeded-job recovery correctly.
- [ ] Previously accepted TASK-008 regressions remain protected.
- [ ] Real PostgreSQL integration, frontend tests, full CI and local smoke are green.
- [ ] TDD evidence is truthful.

## Live-provider boundary
TASK-013 live OpenAI-compatible adapter foundation is now accepted, but secure BYOK/runtime registration remains a separate follow-on capability. TASK-009 may still merge with an empty production text-provider catalog; that is an explicit safe state, not product completion.

## Review-fix rule
Continue only in the existing TASK-009 dedicated worktree and PR #28. Do not create a replacement branch/PR. Do not self-merge or self-mark DONE.
