# E2E acceptance harness

TASK-046 adds a deliberately small browser-level acceptance harness for SynVideo. It exercises the real Vue application, Go API, PostgreSQL and local S3-compatible infrastructure without production credentials or live paid-provider dependencies.

## Local invocation

Prerequisites: Docker Compose, Go from `apps/api/go.mod`, Node 24, npm 11+, and `curl`.

From a clean checkout:

```bash
npm ci
make e2e
```

The runner creates run-scoped PostgreSQL and SeaweedFS volumes, uses dedicated local ports (`55432` for PostgreSQL and `58333` for S3 by default), runs migrations, starts the API and web app, installs pinned Playwright `1.55.0` without modifying `package-lock.json`, runs Chromium, and tears down the isolated compose project with volumes even after failure.

Override `SYNVIDEO_E2E_RUN_ID`, `SYNVIDEO_E2E_POSTGRES_PORT`, or `SYNVIDEO_E2E_S3_PORT` when running concurrent local jobs. The run ID is incorporated into database/bucket/compose names to avoid state collisions. Fixed API/web ports mean concurrent jobs on the same host must additionally coordinate those two listener ports; CI runs each acceptance job on an isolated runner.

## Identity and external dependencies

The harness uses the existing test/local actor boundary (`SYNVIDEO_LOCAL_ACTOR_ID`) with an isolated non-production UUID. It does not add a browser or API authorization bypass.

The initial smoke scenario only creates and reloads a project, so no AI, stock-media or publishing adapter is invoked. Ordinary acceptance scenarios must remain deterministic and may only introduce provider behavior through approved test/fake adapter boundaries. Live paid-provider credentials must never be supplied to this workflow.

## Initial smoke scenario

`e2e/tests/project-persistence.spec.ts` performs:

1. browser boot at `/projects/new`;
2. project creation through the real form and HTTP API;
3. navigation to the persisted project detail route;
4. a full browser reload;
5. assertion that the same project title is recovered from the backend.

This intentionally protects one critical browser → API → PostgreSQL → reload path rather than duplicating lower-layer validation matrices.

## Diagnostics and privacy

Playwright keeps traces and screenshots only for failures. API/web/infra logs are written under `e2e/artifacts/`; the runner redacts Authorization, Cookie and configured storage-secret patterns before persisted logs are written. The CI workflow uploads this directory for seven days.

Do not add tokens, production credentials, unnecessary private payloads or raw third-party responses to E2E artifacts. New diagnostics must be reviewed for redaction before upload.

## Runtime and retries

The suite uses one worker and a 30-second per-test timeout. CI permits one retry and retains first-failure trace evidence. The separate `E2E Acceptance (non-required)` workflow has a 15-minute job timeout and does not replace or weaken the existing required `Frontend`, `Backend` or `Local Infrastructure` checks.

## Adding future scenarios

Add browser scenarios only for accepted critical integration paths. Prefer accessible roles/names or stable form/test identifiers, assert user-visible durable outcomes, and keep exhaustive edge-case/state-machine/provider matrices in unit/component/API/integration tests.
