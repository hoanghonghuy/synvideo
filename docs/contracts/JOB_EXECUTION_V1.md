# Durable Job Execution V1 Contract

Status: FROZEN for the next F1 parallel wave.

This contract provides a provider-neutral, restart-safe execution foundation for long-running SynVideo work such as AI generation, media processing and rendering. It does not contain Proposal, Script, media or vendor-specific semantics.

## Product/architecture boundary
Long-running work must not be hidden behind one blocking HTTP request. A durable job records accepted work before execution and survives normal process restarts without silently losing successful work or duplicating side effects.

V1 is a PostgreSQL-backed application job queue/worker boundary. No external queue vendor is required.

## Job identity and state
Each job has server-controlled fields:
- `id`: UUID.
- `owner_id`: UUID request principal boundary.
- `project_id`: UUID when the job is project-scoped.
- `kind`: stable application-defined string, max 100 chars.
- `dedupe_key`: optional stable string, max 200 chars; unique for the same owner/kind while applicable.
- `state`: `queued|running|succeeded|failed`.
- `attempt`: positive integer, starts at 0 while queued and increments when execution is claimed.
- `max_attempts`: positive integer.
- `available_at`: UTC time after which queued work may be claimed.
- `lease_until`: nullable UTC lease deadline while running.
- `payload`: JSON object owned by the registered job kind; must not contain credentials/secrets.
- `result`: nullable JSON object produced only on success; job-kind owned.
- `error_code`: nullable stable presentation-safe code.
- `created_at`, `updated_at`, `started_at`, `finished_at`: UTC timestamps as applicable.

## State invariants
- New accepted work starts `queued` with `attempt = 0`.
- Only a claimed queued job may enter `running`.
- A worker claim is atomic and lease-based.
- A `running` job may become `succeeded` or `failed` only through the worker holding its current lease/claim token.
- `succeeded` and terminal `failed` jobs are immutable.
- A process crash must not permanently strand a job in `running`; an expired lease may be reclaimed when attempts remain.
- Reclaim increments attempt exactly once for the new execution attempt.
- Retry must never pretend a side effect is idempotent. Job handlers that create durable domain resources must use a stable job id/dedupe key at the domain integration boundary.
- V1 does not expose cancellation. Add cancellation only through a later explicit product/contract decision.

## Claiming and concurrency
PostgreSQL claiming must prevent two workers from concurrently owning the same execution attempt. `FOR UPDATE SKIP LOCKED` or an equivalent atomic pattern is acceptable.

The repository must support:
- enqueue;
- get owner-scoped job;
- claim next available job for registered kinds;
- renew lease where needed;
- mark success;
- mark retryable failure/requeue with next `available_at` when attempts remain;
- mark terminal failure.

## Handler boundary
A job handler is registered by stable `kind` and receives:
- context;
- immutable job identity/owner/project context;
- decoded kind-specific payload.

The executor owns lifecycle state transitions. Handlers do not update generic job state directly.

Context cancellation/deadline must propagate. Worker shutdown should stop taking new work and allow current handlers to observe cancellation; abandoned leased work is recovered after lease expiry.

## Error safety
Raw vendor/provider errors, credentials and secret-bearing payloads must not be stored in presentation fields. Persist only stable safe `error_code` plus optional non-secret diagnostics explicitly classified for internal logs.

## HTTP boundary
This foundation does **not** add a generic public `/jobs` API. Feature tasks (for example Proposal generation) expose capability-specific HTTP routes while using this internal durable job service/repository.

## Verification
Required behavior includes:
1. enqueue persists queued work;
2. concurrent workers cannot claim the same attempt;
3. successful transition is terminal and durable;
4. retry increments attempt and respects max attempts/backoff availability;
5. expired running lease is reclaimable after process-loss simulation;
6. stale/old lease token cannot complete a newer attempt;
7. owner/project reads remain non-disclosing;
8. dedupe behavior prevents duplicate accepted work where a dedupe key is supplied;
9. context cancellation stops handler execution without corrupting durable state;
10. real PostgreSQL tests prove claim/lease/retry concurrency.

## Integration use
TASK-009 consumes this foundation for Proposal generation jobs. Future media/render tasks may reuse it without changing Proposal or Script domain semantics.
