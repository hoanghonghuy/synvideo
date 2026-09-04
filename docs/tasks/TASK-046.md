# TASK-046 — Critical Creator-Flow End-to-End Acceptance Harness

Status: READY
Priority: P1
Milestone: Production Readiness
Issue: #100
Canonical branch when activated: `feature/TASK-046-e2e-acceptance-harness`
Contract: `docs/contracts/E2E_ACCEPTANCE_V1.md`

## Product outcome
SynVideo has a deterministic browser-level acceptance harness proving a small set of critical creator flows across the real web/API/database/storage stack without live paid-provider dependencies.

## Scope
- Browser runner and project structure for the Vue/Vite application.
- Isolated local/CI bootstrap covering web, API, PostgreSQL and S3-compatible storage.
- Deterministic provider fakes for AI/stock/publishing dependencies.
- Stable test identity/auth bootstrap that does not require production credentials.
- Initial smoke flow crossing browser → API → persistence → browser refresh/reload.
- Deterministic cleanup/isolation between runs.
- Failure diagnostics such as trace/screenshot/log capture with privacy-safe redaction.
- CI integration with explicit runtime/flakiness policy while preserving existing required checks.

## Required behavior
1. A clean checkout can start the isolated E2E environment and execute the browser suite with documented commands.
2. Ordinary E2E runs never call live paid/external providers.
3. At least one smoke flow crosses browser → API → PostgreSQL/storage and proves persisted state survives refresh/reload.
4. Test data is isolated per run and cleanup is deterministic even after failed runs.
5. Auth/test identity setup follows application boundaries rather than bypassing authorization in production code.
6. Failure artifacts are actionable but do not expose secrets, authorization headers or unnecessary private payloads.
7. Browser scenarios remain sparse and critical-path oriented; validation matrices stay at lower test layers.
8. Feature-specific scenarios are added only after their owning feature contract/implementation is accepted.
9. CI integration has bounded runtime, timeout and retry rules and does not weaken existing `Frontend`, `Backend`, or `Local Infrastructure` gates.

## Acceptance criteria
- `E2E_ACCEPTANCE_V1` is implemented and documented.
- Chosen browser runner is committed with deterministic install/runtime configuration.
- Clean-checkout local/CI bootstrap brings up the real application stack used by the smoke scenario.
- Initial creator smoke scenario performs a persisted mutation and validates truthful state after browser refresh.
- External AI/stock/publishing calls are replaced by deterministic test doubles at approved adapter boundaries.
- Parallel/retry-safe isolation and cleanup are regression-tested where practical.
- Failure artifacts are retained with secret/private-data redaction rules.
- Developer documentation explains how future TASK-030/032/033/034/035/036/037 critical-path scenarios are added without duplicating unit/component/API coverage.
- RED → GREEN → REFACTOR evidence follows `docs/engineering/TDD_PROTOCOL.md`.
- Existing required CI remains green; any new required E2E gate is promoted only after runtime/flakiness evidence is acceptable.

## Non-scope
- Exhaustive UI validation coverage.
- Live paid-provider tests in ordinary CI.
- Production synthetic monitoring.
- Replacing unit, component, API or integration tests.
- Freezing exact UI selectors for feature work that is not yet accepted.

## Dependencies / relations
- Complements existing required CI and local-infrastructure validation.
- TASK-039 may consume E2E diagnostics conventions but is not required for the initial harness.
- Feature tasks contribute browser scenarios only when their contracts/implementations are accepted.
- The harness must respect current auth, project isolation and MediaAsset persistence boundaries.

## Activation evidence — 2026-09-05
- protected `develop` rechecked at `efdcb88a67b04a749e1fc1bb3fa94b5b5d4ceb4b`;
- no open implementation PR owns this outcome and no canonical `feature/TASK-046-e2e-acceptance-harness` implementation branch is active;
- repository search still finds no Playwright/Cypress/E2E harness competing with this task;
- current web stack remains Vue 3 + Vite + TypeScript/Vitest, compatible with the contract's Playwright default candidate; exact implementation version remains Developer-owned and must be pinned in the implementation PR;
- initial smoke path is frozen to already-accepted behavior only: supported test identity → browser application boot → real project/scene persisted mutation through UI/API → full reload → same persisted state recovered truthfully; no unfinished feature is required;
- ordinary E2E must use isolated PostgreSQL/S3-compatible infrastructure and deterministic external-provider fakes; no live paid calls;
- E2E CI may start non-required while runtime/flakiness evidence is gathered; existing required checks must remain intact.

TASK-046 is therefore READY for Developer claim on `feature/TASK-046-e2e-acceptance-harness` from latest protected `develop`. PM/TL does not implement the harness.

## TDD focus
Bootstrap failure/success, test identity boundary, persistence across reload, isolation/cleanup, deterministic external-provider fakes, failure-artifact redaction and CI invocation.
