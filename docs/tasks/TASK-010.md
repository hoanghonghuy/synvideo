# TASK-010 — Durable job execution foundation

Status: DONE
Milestone: F1 Creative Workflow / shared execution foundation
Depends on: TASK-002 accepted
Wave: WAVE-F1-C
Branch: `feature/TASK-010-durable-job-foundation`
Base: `develop`
Accepted via: PR #21
Squash merge: `f731f4b9cb13e5878546e0b2a580bdf27fbe222d`

## Acceptance record
Team Lead accepted reviewed head `f64ba818` after the follow-up TDD fixes:
- executor-managed lease heartbeat keeps healthy long-running handlers from duplicate reclaim;
- lease-renewal loss cancels the old handler and stale lease tokens cannot complete newer attempts;
- expired final attempts terminalize durably instead of remaining `running`;
- retry cannot exceed `max_attempts` at the durable repository boundary;
- payload and successful result satisfy frozen JSON-object semantics at domain/repository/database boundaries;
- real PostgreSQL claim/lease/retry/crash coverage and race-relevant tests pass;
- CI #109 green;
- no Proposal/Script/provider/public generic jobs scope leakage.

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
`docs/contracts/JOB_EXECUTION_V1.md` is authoritative.

## Primary write paths
- `apps/api/internal/jobs/**`
- `apps/api/internal/postgres/job_repository*.go`
- `apps/api/internal/migrations/sql/0004_create_jobs.sql`
- job-specific tests.

## Scope delivered
- Job domain/envelope and safe validation.
- PostgreSQL migration `0004_create_jobs.sql`.
- Repository enqueue and owner/project-scoped get.
- Atomic next-job claiming for registered kinds.
- Lease token/deadline semantics with executor heartbeat and stale-worker fencing.
- Attempt/max-attempt lifecycle.
- Retry/requeue with `available_at`.
- Expired lease reclaim when attempts remain and terminalization when exhausted.
- Terminal success/failure.
- Dedupe key enforcement.
- Handler registry/executor boundary with context propagation.
- No generic public HTTP `/jobs` route.

## Important invariants
- Two workers cannot own the same execution attempt.
- An old/stale lease token cannot complete a newer attempt.
- Process loss cannot permanently strand eligible work.
- Successful jobs are terminal and cannot be executed again.
- Retry accounting increments exactly once per claimed attempt and never exceeds `max_attempts`.
- Job payload/result are JSON objects; presentation-safe error fields do not contain secrets by design.
- Generic job infrastructure does not know Proposal/Script/media/provider schemas.

## TDD coverage
1. enqueue persists `queued`, attempt 0;
2. concurrent claimers yield only one owner for an attempt;
3. claim increments attempt and sets a lease atomically;
4. success is durable and terminal;
5. retry respects attempts/backoff availability;
6. terminal failure/max-attempt behavior;
7. expired lease reclaim;
8. stale lease token fencing;
9. owner/project non-disclosure;
10. dedupe enforcement;
11. handler context cancellation;
12. long-running handler lease heartbeat;
13. exhausted expired lease terminalization;
14. durable retry cap;
15. payload/result JSON-object validation.

## Acceptance criteria
- [x] Migration is compatible with the current migration runner and follows `0004` ownership.
- [x] Contract state/lease/retry invariants are enforced under concurrency.
- [x] Long-running healthy handlers retain lease ownership without duplicate reclaim.
- [x] Final-attempt process loss cannot permanently strand a running job.
- [x] Retry cannot exceed `max_attempts` at the durable repository boundary.
- [x] Payload/result satisfy frozen JSON-object semantics.
- [x] Real PostgreSQL tests prove multi-worker claim and crash/reclaim behavior.
- [x] No feature-specific HTTP or Proposal/Script/provider behavior is introduced.
- [x] No external queue vendor dependency is introduced.
- [x] `gofmt`, vet, race-relevant tests, backend build and full CI green.
- [x] TDD evidence is truthful.

## Integration
TASK-009 consumes this foundation for Proposal generation jobs after TASK-008 is accepted. Future generation/media/render work may reuse it.
