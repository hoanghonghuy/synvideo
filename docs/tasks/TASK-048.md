# TASK-048 — Production Database Connection Budgeting & Pool Resilience

Status: BACKLOG
Priority: P1
Milestone: Production Readiness
Issue: #104
Canonical branch when activated: `feature/TASK-048-db-connection-budgeting`

## Product outcome
SynVideo can run multiple production API instances without exhausting PostgreSQL connections or hanging indefinitely during initial database establishment, while operators can reason about and observe the connection budget safely.

## Evidence
Protected `develop` currently constructs PostgreSQL with `pgxpool.New(ctx, cfg.DatabaseURL)` and immediately calls `pool.Ping(ctx)` using the process-lifetime signal context. No explicit application-level pool sizing/lifetime/idle policy or dedicated startup database timeout is configured in the repository.

This leaves production behavior coupled to library/environment defaults and makes aggregate connection use difficult to budget against the selected database service's connection ceiling as API replicas and operational workloads grow.

## Scope
- Define explicit application configuration for production PostgreSQL pool sizing and lifecycle policy.
- Bound initial connection establishment / startup ping with a dedicated timeout.
- Define an aggregate connection-budget rule accounting for expected API replicas plus migration/admin headroom.
- Validate invalid/unsafe configuration before serving traffic.
- Preserve normal request/job context cancellation and avoid reconnect loops that amplify database outages.
- Expose privacy-safe pool saturation/acquire-failure diagnostics suitable for TASK-039 observability.
- Add regression/config tests for pool construction and bounded startup failure.

## Required behavior
1. Production does not rely solely on implicit pgxpool defaults for maximum connection usage.
2. Initial database connectivity verification has an explicit finite timeout independent from process lifetime.
3. Pool configuration rejects nonsensical values and documents safe production defaults.
4. The initial production policy leaves explicit connection headroom for migrations/operations and expected horizontal replicas.
5. Database unavailability does not create an unbounded process-local reconnect storm.
6. Pool pressure/acquire failures can be diagnosed without leaking credentials, DSNs or private request payloads.
7. Development/local defaults remain usable without requiring production infrastructure.

## Acceptance criteria
- Explicit documented pool configuration is implemented and validated.
- Startup database connect/ping is bounded by a dedicated timeout and covered by regression tests.
- Production connection-budget guidance explains the relationship between per-instance pool maximum, expected replica count and reserved operational headroom.
- Invalid pool values fail configuration validation deterministically.
- Relevant pool saturation/acquire diagnostics are available for production operations without secret leakage.
- Existing required `Frontend`, `Backend` and `Local Infrastructure` CI remains green.

## Non-scope
- PostgreSQL backup/PITR/restore, owned by TASK-044.
- HTTP connection/body bounds, owned by TASK-043.
- General query/index optimization without evidence.
- Selecting a particular managed PostgreSQL vendor.
- Implementing an external database proxy unless READY-time topology specifically requires one.

## Dependencies / relations
- TASK-041 production topology supplies the expected database service connection ceiling and initial API replica assumptions.
- TASK-039 may surface pool health/saturation diagnostics.
- TASK-044 remains responsible for data protection/recovery.

## Activation gate
Remain BACKLOG after planning freeze. Before READY, PM/TL must re-check exact `develop`, dedupe newer database-pool/config work, confirm the selected production database connection ceiling and expected API replica count, freeze initial max/min/lifetime/idle/startup-timeout defaults with operational headroom, and reconcile implementation WIP capacity. Developer owns implementation only after READY activation.

## TDD focus
Invalid configuration, bounded startup timeout, explicit pool construction, replica/headroom budgeting documentation, and non-secret saturation/acquire diagnostics.
