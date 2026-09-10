# TASK-045 — Paid-generation quota, throttling & cost guardrails

Status: ACTIVE / CLAIMED (`AUTO-OWNER: SYNVIDEO-A`)
Priority: P1
Base branch: `develop`
Canonical branch: `feature/TASK-045-paid-generation-guardrails`
Issue: #95

## Goal
Provide provider-neutral guardrails that bound paid AI generation abuse and unexpected spend while preserving legitimate creator workflows and durable-job/idempotency semantics.

## Problem / evidence
Protected `develop` allows authenticated provider-backed generation flows to create paid work after request validation and idempotency checks, but there is no repository-level principal/project quota, paid-work concurrency policy, spend guard or explicit cost-abuse boundary. TASK-043 limits generic HTTP/process resources, and TASK-032 owns exactly-once semantics for a single logical video generation, but neither bounds the number or rate of intentional valid paid operations.

## Frozen first-production policy boundary
- Cost-bearing logical operations are provider-backed image generation, narration/TTS generation, and scene-video generation.
- Enforcement key is verified principal + project + operation kind + logical request ID.
- Shared/durable enforcement uses PostgreSQL for the accepted first-production topology; process-local counters are not authoritative.
- A new logical paid submit must reserve quota/concurrency before provider work. Replay/resume of the same logical request preserves its original quota charge while reacquiring a live concurrency lease when work resumes.
- Production limits are explicit validated environment configuration. Missing or partial production configuration fails closed; the repository does not invent provider prices or implicit unlimited/default business quotas.
- Quota exhaustion and concurrency throttling remain distinct stable errors and carry bounded retry/reset hints. Guard/runtime/provider failure is not mislabeled as a policy denial.

## Scope
- Define the provider-backed operation kinds treated as paid/cost-bearing policy subjects, including image, narration/TTS, video and future generation kinds where applicable.
- Define first-production principal/project request-rate and in-flight concurrency controls.
- Define configurable quota/budget guardrails with explicit hard-limit, throttle and provider-outage semantics.
- Enforce policy before initiating new paid logical work.
- Preserve idempotency: retry/resume of an already-authorized logical generation must not be charged as a new paid operation.
- Define stable API rejection semantics and retry/reset metadata without leaking provider credentials or cost internals.
- Use shared/durable enforcement where multi-instance correctness requires it; do not depend on process-local counters for production correctness.
- Record privacy-safe policy decisions for observability/audit.
- Add regression coverage for burst throttling, concurrent requests, boundary resets, principal/project isolation and idempotent retry/resume.

## Non-scope
- End-user subscription/payment product design.
- Full billing ledger or invoicing unless separately required.
- Provider-specific business coupling in domain code.
- Replacing provider-side rate limits.

## Acceptance criteria
1. Covered paid-generation operation kinds and policy ownership boundaries are explicit.
2. New paid logical submits are subject to principal/project rate, concurrency and quota policy before provider work starts.
3. Idempotent retry/resume of the same logical generation does not consume quota as brand-new paid work.
4. Enforcement is safe across multiple API/worker instances where process-local counters would be insufficient.
5. Quota/throttle rejection has stable API semantics plus appropriate retry/reset metadata without sensitive provider leakage.
6. Limits are configurable by environment with safe production defaults and no committed secrets.
7. Tests cover burst/concurrency boundaries, isolation, idempotent retry/resume and reset behavior.
8. Request/job observability records privacy-safe allow/deny decisions sufficient for spend-abuse diagnosis.
9. Existing required `Frontend`, `Backend` and `Local Infrastructure` CI remains green.

## Quality / implementation notes
- Keep policy/business semantics provider-neutral even when providers expose different rate or cost characteristics.
- Apply guards to new logical paid work, not every transport retry or durable polling action.
- Prefer durable/shared state or atomic primitives where correctness crosses process boundaries.
- Use renewable, fenced leases for in-flight concurrency; bounded cleanup plus lease expiry provides crash/stall recovery.
- Scene-video quota applies to the paid `StartVideo` + durable external-operation checkpoint boundary, not result polling after a durable submit checkpoint exists.
- Fail closed when a policy decision cannot be made safely, but distinguish policy denial from provider/runtime failure.
- Never log prompts, credentials, signed URLs or private generation payloads merely to explain quota decisions.

## Production configuration
The four values below are configured together and must all be positive:

- `SYNVIDEO_PAID_GENERATION_MAX_REQUESTS`
- `SYNVIDEO_PAID_GENERATION_WINDOW`
- `SYNVIDEO_PAID_GENERATION_MAX_IN_FLIGHT`
- `SYNVIDEO_PAID_GENERATION_LEASE_DURATION`

Production startup fails when the policy is absent, partial or invalid. Non-production environments may leave the policy disabled for local/test compatibility unless explicitly configured.

## Dependencies / relations
- TASK-040 production authentication supplies trustworthy principals before public production enforcement and is accepted for activation.
- TASK-041 supplies the accepted first-production topology/distributed-state assumptions and is accepted for activation.
- TASK-032 owns exactly-once semantics for an individual scene-video operation; TASK-045 bounds the number/rate of intentional logical paid operations.
- TASK-043 owns generic HTTP resource bounds, not paid-work quotas.
- TASK-039 observability should surface privacy-safe throttle/quota diagnostics when activated.

## Activation
Activated on 2026-09-10 after TASK-040/TASK-041 acceptance, duplicate/branch reconciliation, and PM/TL policy freeze. Implementation is owned by `SYNVIDEO-A` on the canonical branch above.

## Delivery
Implementation changes go through the canonical task branch and PR into `develop`. Final merge requires exact-current-head CI/QA evidence and independent cross-review; implementation self-review does not satisfy that gate.
