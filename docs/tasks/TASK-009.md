# TASK-009 — AI Proposal generation job integration

Status: CHANGES_REQUESTED
Milestone: F1 Creative Workflow
Depends on: TASK-006, TASK-007, TASK-008 and TASK-010 accepted
Wave: WAVE-F1-F
Branch: `feature/TASK-009-ai-proposal-integration`
Base: `develop`
Active PR: #28
Current reviewed head: `6ca7ca4d0549ffcc2f42d4e8ef018521a5962e13`

## Current review gate
CI #154 is green. Team Lead review #5073871824 accepts the prior four fixes but found three remaining blockers on the same branch/worktree:

1. Durable job payload decoding must be strict per frozen `AI_PROPOSAL_JOB_V1`. `Handler.Handle` still uses plain `json.Unmarshal`, which accepts unknown fields. Use `DisallowUnknownFields` or an equivalent strict decoder and add a regression proving unknown payload fields terminalize as `GENERATION_INVALID_PAYLOAD` before provider or persistence work.
2. The new duplicate-race service test does not actually execute `Enqueue -> ErrDuplicateJob -> re-read winner`, because it inserts the winner into the mock before the initial lookup. Make the first lookup return not-found, make enqueue return `ErrDuplicateJob`, then expose the winner on the second lookup; prove both matching replay and conflicting provider/model reuse.
3. A transient status GET failure while a job is `queued`/`running` currently stops polling permanently. `activeJob` remains non-terminal, Generate stays disabled, and the generic retry-generation action is hidden. Retry/status-poll the same durable job ID safely (or expose a dedicated status-retry action) and add a regression where one status GET fails and the same job later succeeds without a new POST/request ID.

Previous review fixes now accepted:
- `source_generation_job_id` is hidden from public Proposal JSON with HTTP regression coverage;
- replay/conflict validation is centralized and existing jobs are checked before current Brief/provider state;
- the three accepted TASK-008 frontend regressions are restored;
- succeeded-job Proposal-load recovery keeps the succeeded job/result and retries loading instead of creating another generation job.

Required verification after fixes:
- focused handler strict-payload regression;
- service test that genuinely executes the duplicate-enqueue race branch;
- frontend transient status-poll recovery regression proving no new generation POST;
- existing PostgreSQL exactly-once tests;
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
- recover transient status-poll failures against the same job ID without creating a new generation request;
- on success refresh history and open returned Proposal version;
- terminal retry is explicit and creates a new request/job;
- never render deterministic fake output as production success.

## TDD plan
Truthful RED -> GREEN -> REFACTOR must cover at least:
1. current owner-scoped Project/Brief snapshot and exact source Brief revision;
2. same `request_id` HTTP retry returns same job;
3. conflicting request ID reuse -> `GENERATION_REQUEST_CONFLICT`, including a genuinely exercised duplicate-enqueue race;
4. missing Brief -> `CREATIVE_BRIEF_REQUIRED` without provider call for a genuinely new request;
5. no credentials in request/job payload/result/presentation errors;
6. provider/model catalog contains only text-capable registered models;
7. handler strictly rejects malformed/unknown-field payloads before provider/persistence work;
8. handler persists nothing on provider failure/invalid output;
9. retryable provider failure enters generic retry lifecycle;
10. invalid output is terminal for that job;
11. real PostgreSQL crash/reclaim simulation proves one job creates at most one Proposal draft;
12. distinct regenerate jobs preserve approved history + one-active-draft invariant;
13. owner/project status non-disclosure;
14. dirty frontend blocks generation;
15. pending/failure leaves currently-reviewed Proposal intact;
16. refresh resumes a nonterminal job;
17. transient status GET failure recovers the same nonterminal job without a new POST/request ID;
18. success opens exact returned Proposal version and remains recoverable if follow-up list/version loading transiently fails;
19. production composition has no fake provider registration;
20. full local smoke through Proposal approval;
21. previously accepted TASK-008 load/mutation/dirty-state regressions remain covered.

## Acceptance criteria
- [ ] Frozen `AI_PROPOSAL_JOB_V1` is implemented without contract drift.
- [ ] Provider execution never blocks the initiating HTTP request.
- [ ] Generic TASK-010 jobs/executor is reused and wired into runtime; no second job engine/table.
- [ ] Same HTTP `request_id` is idempotent and conflicting reuse is deterministic under a proven duplicate-enqueue race.
- [ ] Durable job payload decoding is strict and rejects unknown fields safely.
- [ ] One successful durable job creates at most one Proposal version even across commit->crash->reclaim.
- [ ] `source_generation_job_id` remains internal and never appears in public Proposal JSON.
- [ ] Approved Proposal history remains immutable.
- [ ] Safe catalog/status APIs expose no secret/job internals.
- [ ] Production composition never exposes fake provider as live AI.
- [ ] UI preserves dirty/current Proposal state, nonterminal polling recovery and durable succeeded-job recovery correctly.
- [ ] Previously accepted TASK-008 regressions remain protected.
- [ ] Real PostgreSQL integration, frontend tests, full CI and local smoke are green.
- [ ] TDD evidence is truthful.

## Live-provider boundary
TASK-013 live OpenAI-compatible adapter foundation is now accepted, but secure BYOK/runtime registration remains a separate follow-on capability. TASK-009 may still merge with an empty production text-provider catalog; that is an explicit safe state, not product completion.

## Review-fix rule
Continue only in the existing TASK-009 dedicated worktree and PR #28. Do not create a replacement branch/PR. Do not self-merge or self-mark DONE.
