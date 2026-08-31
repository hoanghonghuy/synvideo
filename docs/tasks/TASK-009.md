# TASK-009 — AI Proposal generation job integration

Status: BLOCKED
Milestone: F1 Creative Workflow
Depends on: TASK-006, TASK-007 and TASK-008 accepted
Wave: post-WAVE-F1-B integration
Branch: `feature/TASK-009-ai-proposal-integration`
Base: `develop`

## Goal
Connect the accepted Proposal persistence/API, provider-neutral generation engine and Proposal frontend through an explicit asynchronous generation-job boundary: request generation, track queued/running/succeeded/failed state, persist the successful candidate as a new draft version, then review/edit/approve it.

## Why async
ADR 0005 requires long-running generation to evolve through explicit jobs rather than blocking HTTP requests. This task must not hide provider latency behind one synchronous request.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/decisions/0005-technical-baseline.md`
- `docs/contracts/AI_PROPOSAL_V1.md`
- `docs/contracts/AI_PROPOSAL_GENERATION_V1.md`
- accepted TASK-006/007/008 implementations

## Scope
- Owner-scoped generation application service loads Project + current Creative Brief.
- Select provider/model through accepted provider registry/capability boundary.
- Call accepted Proposal generation engine from a job worker/executor boundary.
- Persist a Proposal only after a validated successful candidate is produced.
- Durable generation-job state sufficient for restart/retry-safe product behavior; do not bind domain semantics to a queue vendor.
- Proposed V1 HTTP shape to finalize/freeze before implementation:
  - `POST /api/v1/projects/{project_id}/creative-proposal-generations` -> `202` generation job;
  - `GET /api/v1/projects/{project_id}/creative-proposal-generations/{job_id}` -> current job status/result;
  - request contains stable `provider_id` and `model_id`, never credentials;
  - successful job exposes the created Proposal `version`/resource reference.
- Job states at minimum `queued|running|succeeded|failed`; cancellation/retry semantics must be explicitly decided before coding if introduced.
- Provider unavailable/failure/invalid output maps to stable presentation-safe job error data.
- Frontend Generate/Regenerate starts a job, shows progress/status, handles failure/retry and opens returned draft after success.
- Regenerate creates a new draft version and never mutates approved history.
- Full flow: Creative Brief -> generation job -> Proposal draft -> edit/save -> regenerate/version history -> approve.

## TDD plan
Start RED around integration/job seams:
1. generation request captures current owner-scoped Creative Brief revision;
2. accepted job transitions legally through states and persists candidate only on success;
3. provider failure/invalid output leaves no partial Proposal version;
4. worker retry/restart behavior cannot create duplicate Proposal versions for one successful job;
5. regenerate never mutates an approved version;
6. no Creative Brief -> stable failed/rejected generation behavior without provider call;
7. concurrent generation/version creation cannot allocate duplicate active versions;
8. frontend starts job and displays queued/running/succeeded/failed accurately;
9. frontend failure preserves the currently reviewed Proposal state;
10. complete local smoke through approval.

## Acceptance criteria
- [ ] HTTP generation is asynchronous (`202` job), not a provider-blocking request.
- [ ] Job state is durable enough that normal process restart does not silently lose or duplicate successful work.
- [ ] Generation failure is transactional: no partial Proposal version.
- [ ] Successful job creates exactly one new Proposal draft version.
- [ ] Approved Proposal history is preserved.
- [ ] No API keys/secrets are sent in generation request payloads or presentation errors.
- [ ] Provider-specific SDK/schema remains outside domain/frontend Proposal semantics.
- [ ] Frontend never displays fake success before a job actually succeeds.
- [ ] TDD evidence, full CI and real integration smoke are green.

## Live-provider gate
TASK-009 must not pretend fake-provider output is production AI. If no live provider/BYOK capability is accepted by implementation time, local deterministic fake may be used only for automated integration tests, while product-visible live execution remains explicitly blocked and PM must open the provider/BYOK task before declaring AI Proposal production-complete.

## Blocked reason
Requires the three independently developed WAVE-F1-B outputs to be accepted first. Before moving READY, PM must also freeze the generation-job HTTP/state contract and confirm the live-provider/BYOK gate.

Do not self-merge or self-mark DONE.
