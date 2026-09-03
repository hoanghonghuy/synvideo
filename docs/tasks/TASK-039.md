# TASK-039 — Production readiness & HTTP request observability

Status: BACKLOG
Priority: P2
Base branch: `develop`
Canonical branch: `feature/TASK-039-readiness-observability`
Issue: #80

## Goal
Provide truthful application readiness and a minimal production-grade HTTP request observability baseline so deployment health checks stop routing traffic to instances whose critical dependencies are unavailable, while request failures/latency remain safely diagnosable without vendor lock-in.

## Problem / evidence
Current `apps/api/internal/httpserver/server.go` exposes `/api/v1/healthz` and `/api/v1/readyz`, but readiness currently succeeds unconditionally rather than probing dependencies required for safe request serving. Existing request logging records method/path before dispatch but does not capture response status, duration, request/correlation ID, or correlated panic/error outcome.

This creates two production risks:
- an orchestrator/load balancer may consider an instance ready while a required dependency is unavailable;
- incident/debug logs lack enough request-level context to correlate failures or latency safely.

Dedupe was performed against current open/closed issues, PRs and repository docs/code before issue #80 was created; no existing task owns this outcome.

## Scope
### Liveness / readiness
- Keep `healthz` as a cheap process-liveness signal.
- Define explicit readiness semantics for dependencies required to safely serve configured runtime capabilities.
- At minimum, database readiness must be checked when the database-backed runtime is configured.
- Object storage must affect readiness only when current configuration/runtime actually requires it for safe serving; optional/unconfigured capabilities must not make the whole API unready.
- Dependency probes must be bounded by context timeout and return deterministic non-2xx readiness failure without leaking internal details.

### Request observability
- Propagate a valid incoming request ID or generate one when absent; expose it in a documented response header.
- Record a structured request-completion log with method, safe route/path representation, response status, duration and request ID.
- Correlate safe error/panic logging and response handling with the same request ID.
- Never log credentials, raw authorization headers, signed URLs, secrets or sensitive payloads.
- Avoid accidental high-cardinality or sensitive route/path logging where route templates are unavailable.

### Verification / operational docs
- Add focused regression coverage for readiness success/failure/recovery and bounded timeout behavior.
- Add middleware tests covering request-ID propagation/generation, response status and completion logging behavior.
- Document liveness/readiness operational semantics and request-ID response behavior for deployment and future metrics/tracing work.

## Non-scope
- Paid observability/APM vendor selection.
- Full distributed tracing rollout.
- Large metrics/dashboard program.
- Rewriting domain logging across every package.
- Changing product-feature behavior unrelated to readiness/request instrumentation.

## Acceptance criteria
1. Liveness and readiness semantics are documented and independently testable.
2. `readyz` succeeds only when configured dependencies required for safe serving are usable; required dependency failure returns deterministic non-2xx without leaking internals.
3. Readiness dependency checks are bounded and cannot hang indefinitely.
4. Optional or unconfigured capabilities do not incorrectly make the entire API unready.
5. API requests propagate or receive a safe request ID and the documented response header returns it.
6. Structured request completion logs include method, safe route/path policy, response status, duration and request ID.
7. Panic/error handling produces safe correlated logs/responses without exposing secrets.
8. Regression tests prove dependency failure/recovery, timeout behavior, request-ID behavior and response-status logging.
9. Existing required `Frontend`, `Backend` and `Local Infrastructure` checks remain green.

## Quality gate
- Follow `docs/engineering/TDD_PROTOCOL.md` for behavior changes.
- No required check may be weakened or skipped to make the task pass.
- Review exact current `develop` and existing middleware/server composition before implementation so readiness probes remain small, bounded and testable.
- Prefer existing standard-library `slog` / `net/http` conventions; do not introduce an observability vendor dependency solely for this task.

## Dependencies
- Technical foundation / HTTP server baseline: DONE.
- Database and media-storage foundations: DONE.
- No dependency on TASK-030/TASK-031 behavior; WIP capacity is the only current activation constraint.

## Activation gate
Remain BACKLOG until PM/TL reconciles current implementation WIP capacity or deployment-readiness becomes the immediate release gate. Promote to READY only after a fresh duplicate/branch check confirms no implementation owner already exists.

## Delivery
Developer owns implementation on `feature/TASK-039-readiness-observability` and opens a PR into `develop`. Developer must not self-merge or self-mark DONE.
