# TASK-009 — AI Proposal generation job integration

Status: BLOCKED
Milestone: F1 Creative Workflow
Depends on: TASK-006, TASK-007, TASK-008 and TASK-010 accepted
Wave: post-WAVE-F1-C integration
Branch: `feature/TASK-009-ai-proposal-integration`
Base: `develop`

## Goal
Connect the accepted Proposal persistence/API, provider-neutral generation engine and Proposal frontend through the accepted durable job foundation: request generation, track queued/running/succeeded/failed state, persist the successful candidate as a new draft version, then review/edit/approve it.

## Why async
ADR 0005 requires long-running generation to evolve through explicit jobs rather than blocking HTTP requests. TASK-010 owns the generic durable queue/lease/retry foundation; this task owns Proposal-generation semantics and capability-specific HTTP/UI integration only.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/decisions/0005-technical-baseline.md`
- `docs/contracts/JOB_EXECUTION_V1.md`
- `docs/contracts/AI_PROPOSAL_V1.md`
- `docs/contracts/AI_PROPOSAL_GENERATION_V1.md`
- accepted TASK-006/007/008/010 implementations

## Scope
- Owner-scoped generation application service loads Project + current Creative Brief.
- Select provider/model through accepted provider registry/capability boundary.
- Enqueue a Proposal-generation job through TASK-010 durable job service.
- Register a Proposal-generation job handler that calls accepted generation engine.
- Persist a Proposal only after a validated successful candidate is produced.
- Use stable job identity/dedupe semantics so retry/restart cannot create duplicate Proposal versions for one successful generation request.
- Feature-specific HTTP shape to finalize/freeze before implementation:
  - `POST /api/v1/projects/{project_id}/creative-proposal-generations` -> `202` generation job;
  - `GET /api/v1/projects/{project_id}/creative-proposal-generations/{job_id}` -> current job status/result;
  - request contains stable `provider_id` and `model_id`, never credentials;
  - successful job exposes the created Proposal `version`/resource reference.
- Reuse generic `queued|running|succeeded|failed` lifecycle from `JOB_EXECUTION_V1`; do not fork a second job engine.
- Provider unavailable/failure/invalid output maps to stable presentation-safe job error data.
- Frontend Generate/Regenerate starts a job, shows progress/status, handles failure/retry and opens returned draft after success.
- Regenerate creates a new draft version and never mutates approved history.
- Full flow: Creative Brief -> generation job -> Proposal draft -> edit/save -> regenerate/version history -> approve.

## TDD plan
Start RED around integration seams:
1. generation request captures current owner-scoped Creative Brief revision;
2. accepted job payload contains no credentials/secrets;
3. generation handler persists candidate only on success;
4. provider failure/invalid output leaves no partial Proposal version;
5. durable retry/reclaim of the same job cannot create duplicate Proposal versions;
6. regenerate never mutates an approved version;
7. no Creative Brief -> stable rejected/failed behavior without provider call;
8. concurrent generation/version creation cannot allocate duplicate active versions;
9. frontend displays queued/running/succeeded/failed accurately;
10. frontend failure preserves the currently reviewed Proposal state;
11. complete local smoke through approval.

## Acceptance criteria
- [ ] HTTP generation is asynchronous (`202` job), not a provider-blocking request.
- [ ] Uses accepted TASK-010 durable job foundation instead of implementing another queue/lease table.
- [ ] Generation failure is transactional: no partial Proposal version.
- [ ] One successful durable job creates exactly one Proposal draft version even across retries/reclaims.
- [ ] Approved Proposal history is preserved.
- [ ] No API keys/secrets are sent in generation request payloads, durable job payloads or presentation errors.
- [ ] Provider-specific SDK/schema remains outside domain/frontend Proposal semantics.
- [ ] Frontend never displays fake success before a job actually succeeds.
- [ ] TDD evidence, full CI and real integration smoke are green.

## Live-provider gate
TASK-009 must not pretend fake-provider output is production AI. If no live provider/BYOK capability is accepted by implementation time, deterministic fake may be used only for automated integration tests while product-visible live execution remains explicitly blocked. PM must open/accept the live provider/BYOK task before declaring AI Proposal production-complete.

## Blocked reason
TASK-006 and TASK-007 are accepted. TASK-008 frontend and TASK-010 durable job foundation must be accepted before this task moves READY. Before implementation PM must freeze the Proposal-generation HTTP/job-payload/result contract and confirm the live-provider/BYOK gate.

Do not self-merge or self-mark DONE.
