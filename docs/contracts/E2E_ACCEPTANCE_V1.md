# E2E_ACCEPTANCE_V1

## Purpose
Define the browser-level acceptance boundary for SynVideo so critical creator flows are proven against the running application stack without turning E2E into a slow duplicate of lower-level tests.

## Harness boundary
The harness exercises the real web application, HTTP API, PostgreSQL persistence and S3-compatible managed storage used by the selected smoke scenario. External AI, stock-media and publishing providers are replaced with deterministic fakes at provider/adapter boundaries.

The harness must not alter production authorization semantics or introduce production-only bypass code for tests.

## Runner and invocation
Use one browser E2E runner for V1. Playwright is the default candidate because it supports Chromium-class browser automation, traces/screenshots and CI-friendly operation, but READY-time validation may choose another runner if current repository constraints justify it.

A clean checkout must have documented commands to:
1. prepare/start the isolated stack;
2. seed/create isolated test identity and data;
3. run the browser suite;
4. collect diagnostics;
5. clean up deterministically.

## Initial smoke contract
The first accepted scenario is intentionally small and must use behavior already accepted on `develop` at activation time. It must prove:
- browser navigation/application boot;
- authenticated creator access through the supported test identity boundary;
- at least one project/scene mutation through the real UI/API path;
- durable persistence in PostgreSQL and/or managed storage as applicable;
- full browser refresh/reload;
- truthful recovery of the persisted state after reload.

Do not anchor the initial smoke scenario to an unfinished feature merely to increase coverage.

## External dependency isolation
Ordinary E2E CI must never make live paid calls or depend on third-party provider availability.

Provider fakes must be deterministic and preserve the application's adapter contracts closely enough to exercise success and selected critical failure states. Tests must not monkey-patch browser code to skip server-side domain boundaries.

## Identity and authorization
Test identity/auth bootstrap must be explicit and documented. It may use supported local/test-only infrastructure, fixtures or tokens generated for the isolated environment, but must not require production credentials or weaken production authorization checks.

Cross-project/principal behavior should remain testable at lower layers and may have sparse browser coverage only when it protects a critical release path.

## Isolation and cleanup
Every run must use isolated test state sufficient to avoid collisions with parallel/retried runs. Names/IDs may include run-scoped entropy when necessary.

Cleanup is deterministic and safe to retry. A failed run must not poison subsequent runs. Test teardown may remove only state created by the isolated harness and must never target production resources.

## Selectors and UI stability
Prefer semantic selectors based on accessible roles/names or intentionally stable test identifiers. Avoid selectors tied to incidental CSS structure.

E2E contracts assert user-visible outcomes and durable state, not internal implementation details.

## Failure diagnostics and privacy
Failed runs should retain useful diagnostics supported by the chosen runner, such as trace, screenshot, console/network summary and relevant service logs.

Artifacts/logs must redact or omit:
- secrets and credentials;
- authorization headers/cookies/tokens;
- unnecessary private user content;
- full third-party payloads not required to diagnose the failure.

Retention and upload behavior must follow repository/CI privacy policy.

## Runtime, retries and flakiness
The suite must remain intentionally small and bounded. Tests that are flaky because of timing should be fixed rather than hidden behind broad retries.

Any retry policy must be low and explicit, preserve first-failure evidence, and never convert a reproducibly failing acceptance path into green.

A new E2E job may begin non-required while runtime/flakiness is measured. Promotion to a required merge gate requires stable evidence and must not remove or weaken existing `Frontend`, `Backend`, or `Local Infrastructure` checks.

## Coverage ownership
E2E owns only critical cross-layer creator paths. Lower layers continue to own exhaustive validation, state-machine, API, provider-adapter and edge-case matrices.

When TASK-030/032/033/034/035/036/037 are accepted, each may contribute at most the small number of browser scenarios needed to protect its critical integration path. Their acceptance criteria do not automatically imply browser coverage for every case.

## Required regression coverage
- clean-checkout harness startup/invocation documentation remains valid;
- initial browser → API → persistence → refresh smoke path;
- deterministic external-provider fake wiring with no live network call;
- run isolation and retry-safe cleanup;
- test identity/auth bootstrap without production credentials;
- actionable failure artifacts with redaction expectations;
- CI command/job remains bounded and preserves existing required gates.

## READY-time validation
Before implementation activation, PM/TL re-checks current web/API/auth/test infrastructure, runner ecosystem/compatibility, local stack topology and CI runtime constraints. The exact initial smoke path must use behavior already accepted on current `develop`.
