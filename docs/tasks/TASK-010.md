# TASK-010 — Durable job execution foundation

Status: CHANGES_REQUESTED
Milestone: F1 Creative Workflow / shared execution foundation
Depends on: TASK-002 accepted
Wave: WAVE-F1-C
Branch: `feature/TASK-010-durable-job-foundation`
Base: `develop`
Active PR: #21

## Current review gate
Team Lead review on PR #21 found lifecycle/contract blockers on the current head:
- executor claims a lease but does not renew it while a long-running handler is still alive, so another worker can reclaim and execute the same job concurrently after `LeaseDuration` expires;
- an expired lease on the final allowed attempt (`attempt == max_attempts`) is neither reclaimable nor terminalized, so process loss/cancellation can strand a job in `running` forever;
- `MarkRetryableFailure` can requeue a final-attempt job and queued claiming does not cap `attempt < max_attempts`, allowing execution beyond the configured maximum if the durable repository is called directly;
- frozen V1 requires `payload` and successful `result` to be JSON objects, while current validation accepts arbitrary valid JSON values.

Required TDD fixes on the same branch/worktree:
1. healthy handler longer than initial lease -> executor renews/heartbeats -> second worker cannot reclaim;
2. renewal/stale-lease loss prevents the old handler from committing success as current owner;
3. `max_attempts=1` process loss -> expired final lease becomes terminal and never stays `running` forever;
4. final-attempt retry cannot become queued/claimable for an extra attempt;
5. payload/result reject non-object JSON values;
6. rerun real PostgreSQL concurrency tests, race-relevant tests and full CI.

## Goal
Implement the PostgreSQL-backed durable asynchronous job execution foundation defined by `docs/contracts/JOB_EXECUTION_V1.md`, without Proposal/Script/provider-specific business logic.

This task exists so long-running AI/media/render work is restart-safe and does not block HTTP requests.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/decisions/0005-technical-baseline.md`
- `docs/contracts/JOB_EXECUTION_V1.md`

## Frozen contract
`docs/contracts/JOB_EXECUTION_V1.md` is authoritative. Do not change it from this branch.

## Primary write paths
- `apps/api/internal/jobs/**`
- `apps/api/internal/postgres/job_repository*.go`
- `apps/api/internal/migrations/sql/0004_create_jobs.sql`
- job-specific tests.

## Reserved / do not touch
- `apps/web/**` — TASK-008 currently owns Proposal frontend.
- `apps/api/internal/creativeproposal/**` and `proposalgeneration/**` — accepted Proposal work.
- Script paths / migration `0005` — TASK-011.
- Proposal-generation HTTP/routes — TASK-009.
- provider registry/adapters.

## Scope
- Job domain/envelope and safe validation.
- PostgreSQL migration `0004_create_jobs.sql`.
- Repository/service enqueue and owner/project-scoped get.
- Atomic next-job claiming for registered kinds.
- Lease token/deadline semantics protecting attempt ownership.
- Attempt/max-attempt lifecycle.
- Retry/requeue with `available_at`.
- Expired lease reclaim when attempts remain.
- Terminal success/failure.
- Optional dedupe key enforcement as frozen contract specifies.
- Handler registry/executor boundary with context propagation.
- Graceful executor cancellation behavior sufficient for reclaim after lease expiry.
- No generic public HTTP `/jobs` route.

## Important invariants
- Two workers cannot own the same execution attempt.
- An old/stale lease token cannot complete a newer attempt.
- Process loss cannot permanently strand eligible work.
- Successful jobs are terminal and cannot be executed again.
- Retry accounting increments exactly once per claimed attempt.
- Job payload/result/error presentation fields must not contain secrets by design.
- Generic job infrastructure must not know Proposal/Script/media/provider schemas.

## TDD plan
Start RED for at least:
1. enqueue persists `queued`, attempt 0;
2. two concurrent claimers yield only one owner for a job attempt;
3. claim increments attempt and sets a lease atomically;
4. success is durable and terminal;
5. retryable failure requeues only when attempts remain;
6. terminal failure after max attempts;
7. expired lease is reclaimable after simulated worker loss;
8. stale lease token cannot mark success/failure after reclaim;
9. owner/project get is non-disclosing;
10. dedupe key prevents duplicate accepted work where supplied;
11. handler context cancellation propagates without corrupting job state;
12. long-running healthy handler retains ownership through lease renewal;
13. exhausted expired lease terminalizes rather than stranding;
14. durable retry boundary cannot exceed `max_attempts`;
15. payload/result JSON object shape is enforced.

Use real PostgreSQL integration/concurrency tests for claim/lease/retry semantics.

## Acceptance criteria
- [ ] Migration is compatible with the current migration runner and follows `0004` ownership.
- [ ] Contract state/lease/retry invariants are enforced under concurrency.
- [ ] Long-running healthy handlers retain lease ownership without duplicate reclaim.
- [ ] Final-attempt process loss cannot permanently strand a running job.
- [ ] Retry cannot exceed `max_attempts` at the durable repository boundary.
- [ ] Payload/result satisfy frozen JSON-object semantics.
- [ ] Real PostgreSQL tests prove multi-worker claim and crash/reclaim behavior.
- [ ] No feature-specific HTTP or Proposal/Script/provider behavior is introduced.
- [ ] No external queue vendor dependency is introduced.
- [ ] `gofmt`, vet, race-relevant tests, backend build and full CI green.
- [ ] TDD evidence is truthful.

## Integration
TASK-009 will consume this foundation for Proposal generation jobs after TASK-008 is accepted. Future generation/media/render work may reuse it.

Do not self-merge or self-mark DONE.
