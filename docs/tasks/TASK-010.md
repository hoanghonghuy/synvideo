# TASK-010 — Durable job execution foundation

Status: READY
Milestone: F1 Creative Workflow / shared execution foundation
Depends on: TASK-002 accepted
Wave: WAVE-F1-C
Branch: `feature/TASK-010-durable-job-foundation`
Base: `develop`

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
11. handler context cancellation propagates without corrupting job state.

Use real PostgreSQL integration/concurrency tests for claim/lease/retry semantics.

## Acceptance criteria
- [ ] Migration is compatible with the current migration runner and follows `0004` ownership.
- [ ] Contract state/lease/retry invariants are enforced under concurrency.
- [ ] Real PostgreSQL tests prove multi-worker claim and crash/reclaim behavior.
- [ ] No feature-specific HTTP or Proposal/Script/provider behavior is introduced.
- [ ] No external queue vendor dependency is introduced.
- [ ] `gofmt`, vet, race-relevant tests, backend build and full CI green.
- [ ] TDD evidence is truthful.

## Integration
TASK-009 will consume this foundation for Proposal generation jobs after TASK-008 is accepted. Future generation/media/render work may reuse it.

Do not self-merge or self-mark DONE.
