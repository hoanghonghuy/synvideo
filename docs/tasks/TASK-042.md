# TASK-042 — Bound durable-job handler cancellation and lease-loss shutdown

Status: BACKLOG
Priority: P1
Base branch: `develop`
Canonical branch: `feature/TASK-042-bounded-job-cancellation`
Issue: #88

## Goal
Make durable-job executor cancellation and lease-loss handling bounded and safe even when a job handler/provider does not return promptly after context cancellation, without allowing stale work to commit after ownership is lost.

## Problem / evidence
A targeted audit of `apps/api/internal/jobs/executor.go` found that handler execution is delegated to a goroutine, but both parent-context cancellation and lease-renewal failure call the handler cancel function and then wait on `<-resultCh>` without a bound.

If a handler or provider call ignores/does not observe cancellation, `RunOnce` can block indefinitely. This can hang graceful shutdown and can pin a worker after lease loss even though another worker may later reclaim the expired job.

Existing cancellation/lease-loss tests use cooperative handlers that return after `ctx.Done()`, so they do not prove safe behavior for a deliberately non-cooperative handler.

## Scope
- Define bounded executor behavior after parent shutdown/cancellation.
- Define bounded executor behavior after lease renewal failure/loss.
- Preserve fencing: once lease ownership is lost, late handler completion must not commit success/failure using stale ownership.
- Continue delivering context cancellation to cooperative handlers so normal providers can stop promptly.
- Allow a non-cooperative handler goroutine to outlive `RunOnce` only when executor ownership/commit paths are safely detached and bounded.
- Define safe logging/observability for abandoned/stuck handler execution without logging payloads, credentials or provider secrets.
- Review normal provider/network boundaries used by durable jobs and ensure context/deadline propagation is compatible with the executor contract.
- Add regression tests with deliberately non-cooperative handlers.

## Non-scope
- Unsafe forced goroutine termination.
- Rewriting every job handler/provider.
- Distributed worker orchestration redesign.
- Changing feature-specific paid-work idempotency contracts unless required to preserve stale-lease safety.

## Acceptance criteria
1. Canceling the executor/`RunOnce` context cannot block indefinitely solely because a claimed handler ignores cancellation; control flow returns within a documented bound.
2. Lease renewal failure/loss cannot block executor control flow indefinitely on a non-cooperative handler.
3. Once lease ownership is lost, late handler completion cannot call `MarkSuccess`, `MarkRetryableFailure`, or `MarkTerminalFailure` using stale ownership.
4. Cooperative handlers still receive context cancellation and exit promptly without regression to existing reclaim semantics.
5. RED-stage regression tests reproduce the current unbounded wait using a non-cooperative handler for both parent cancellation and lease-loss paths.
6. GREEN-stage tests prove bounded return for both paths and prove late handler completion cannot corrupt/re-finalize the job.
7. Any cancellation grace timeout is configurable or explicitly justified with a safe default and must not accidentally shorten the job lease itself.
8. Logs identify cancellation/lease-loss abandonment with safe job identity/context but never payloads, credentials or provider secrets.
9. Existing durable-job lease renewal/reclaim tests remain green.
10. Required `Frontend`, `Backend` and `Local Infrastructure` CI remains green.

## Quality / implementation notes
- Prefer fencing/ownership semantics and bounded waiting over attempts to kill goroutines.
- Keep the generic safety rule in the jobs executor/lifecycle boundary rather than adding unrelated ad-hoc timeouts to each feature.
- Provider/network operations should normally be context-bounded, but executor correctness must not rely on every handler being perfectly cooperative.
- Follow `docs/engineering/TDD_PROTOCOL.md`; this task specifically requires evidence that the non-cooperative regression test fails before the fix.

## Dependencies / relations
- Generic reliability foundation for TASK-032 and later long-running render/publish jobs.
- Independent from any single generation provider.
- Related to production shutdown/recovery expectations in TASK-041, but can be implemented independently.

## Activation gate
Remain BACKLOG while current implementation WIP is bounded. Before READY, rerun overlap checks against any newer executor/recovery work and freeze the exact cancellation-grace semantics/default. Promote before scaling long-running workers or production release if still unresolved.

## Delivery
Developer implements on `feature/TASK-042-bounded-job-cancellation` and opens a PR into `develop`. Developer must not self-merge or self-mark DONE.
