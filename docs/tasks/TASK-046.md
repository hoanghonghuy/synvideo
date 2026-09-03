# TASK-046 — Critical Creator-Flow End-to-End Acceptance Harness

Status: BACKLOG
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

## Activation gate
Remain BACKLOG after planning freeze. Before READY, PM/TL must re-check exact `develop`, dedupe any new E2E infrastructure/branches/PRs, confirm the chosen runner/bootstrap still fits the current web/API stack, select the initial smoke flow against already-accepted behavior, and reconcile CI runtime/capacity. Developer owns `feature/TASK-046-e2e-acceptance-harness` only after READY activation.

## TDD focus
Bootstrap failure/success, test identity boundary, persistence across reload, isolation/cleanup, deterministic external-provider fakes, failure-artifact redaction and CI invocation.
