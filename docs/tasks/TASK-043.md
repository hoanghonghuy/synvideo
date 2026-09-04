# TASK-043 — HTTP server resource bounds & request-size hardening

Status: READY
Priority: P1
Base branch: `develop`
Canonical branch: `feature/TASK-043-http-resource-bounds`
Issue: #91

## Goal
Bound hostile or accidental HTTP connection/request resource consumption without breaking legitimate media transfer or long-running asynchronous workflows.

## Problem / evidence
Current protected `develop` constructs `http.Server` with only `Addr` and `Handler`; no explicit read-header/read/write/idle timeout policy is set. `apps/api/cmd/api/main.go` runs that server directly with `ListenAndServe()`.

Media upload already has a dedicated configured size limit, but ordinary JSON handlers decode `r.Body` directly and there is no shared JSON request-size guard at the HTTP boundary. This leaves slow/incomplete connections and oversized JSON bodies insufficiently bounded at the application edge.

TASK-039 owns readiness/request observability, not connection/request resource limits. TASK-041 owns deployment/reverse-proxy baseline and should align with this task rather than absorb its implementation.

## Scope
- Define explicit production-safe HTTP server timeout/resource defaults or validated configuration for header reads, reads/writes and idle connections.
- Document any intentionally unbounded timeout with justification.
- Keep timeout behavior compatible with valid media upload/download/streaming semantics.
- Add a shared finite request-body policy for JSON API endpoints, enforced before JSON decoding.
- Keep JSON limits separate from existing media upload-size configuration.
- Return deterministic 4xx behavior for oversized JSON payloads without leaking internals.
- Add focused regression tests for server construction/config bounds and oversized JSON rejection.
- Document alignment requirements for reverse proxies/deployment owned by TASK-041.

## Non-scope
- General API rate limiting or account quotas.
- CDN/WAF selection.
- Reworking media streaming architecture.
- Implementing TASK-039 observability or TASK-041 deployment manifests.

## Acceptance criteria
1. Production `http.Server` has explicit, tested resource/time bounds or documented justification for any intentionally unbounded field.
2. JSON API request bodies have a shared finite maximum enforced before decoding/service work.
3. Oversized JSON requests receive a stable 4xx response and do not reach provider/domain work.
4. Existing media upload/download size and streaming semantics remain separately governed and are not silently reduced.
5. Regression tests cover at least one oversized JSON request and server timeout/config construction.
6. Timeout/body-limit policy is documented for TASK-041 reverse-proxy/deployment alignment.
7. Existing required `Frontend`, `Backend`, and `Local Infrastructure` checks remain green.

## Quality / implementation notes
- Prefer standard-library `net/http` primitives such as `http.Server` timeout fields and `http.MaxBytesReader` where appropriate.
- Do not choose arbitrary tiny timeouts that break legitimate uploads/downloads.
- Apply body limits as early as practical and keep endpoint-specific exceptions explicit.
- Avoid duplicated body-limit logic across every handler if a safe shared boundary can own it.

## Dependencies / relations
- TASK-039: request observability/readiness is related but independent.
- TASK-041: production proxy/deployment configuration must align with these limits.
- Current feature implementation can continue independently; this task is a production hardening gate.

## Activation evidence — 2026-09-04
- protected `develop` rechecked at `b3d2a9c24148d7d0564d34a9e42c7d551e0d7644`;
- TASK-030 completed, freeing one bounded implementation slot while TASK-042 remains the only active implementation workstream;
- no canonical `feature/TASK-043-http-resource-bounds` branch exists;
- no implementation PR overlaps TASK-043 (only historical planning PR #92);
- current code search still finds no `ReadHeaderTimeout` implementation on `develop`;
- no new issue/PR owns the same HTTP edge hardening outcome.

TASK-043 is therefore READY for Developer claim on the canonical branch. READY authorizes implementation but does not authorize PM/TL to write runtime code or self-open the implementation PR.

## Delivery
Developer implements on `feature/TASK-043-http-resource-bounds` and opens a PR into `develop`. Developer must not self-merge or self-mark DONE.
