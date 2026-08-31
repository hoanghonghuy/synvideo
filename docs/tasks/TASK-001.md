# TASK-001 — Technical foundation and runnable project skeleton

Status: READY
Milestone: F0 Technical Foundation
Base branch: `develop`
Branch: `feature/TASK-001-technical-foundation`

## Goal
Create the initial runnable SynVideo application skeleton that later product tasks can safely build on without introducing premature product behavior.

## Why
The repository currently contains product/engineering control-plane documentation but no application implementation. Before AI/video features, SynVideo needs a consistent local development environment, project structure, baseline quality gates and explicit frontend/backend boundaries.

## Read
Required:
- `AGENTS.md`
- `docs/engineering/ARCHITECTURE_PRINCIPLES.md`
- `docs/engineering/CONTEXT_POLICY.md`
- `docs/decisions/0004-i18n-from-start.md`
- `docs/decisions/0005-technical-baseline.md`

Do not load unrelated feature documents for this task.

## Scope
### Repository/application structure
- Establish a clear monorepo structure for Vue frontend, Go backend and local infrastructure.
- Add root-level developer commands/documentation sufficient to install, run, test and verify the scaffold.
- Add `.gitignore` and environment example files without real secrets.

### Frontend baseline
- Vue 3 + TypeScript application starts successfully.
- Routing foundation exists.
- i18n is configured from the start with Vietnamese as the initial locale.
- At least one minimal application shell/page proves routing and localization work.
- No product-heavy placeholder UI is required.
- Lint/typecheck/test commands exist and pass.

### Backend baseline
- Go application starts successfully.
- Expose a minimal health/readiness boundary suitable for local/CI verification.
- Establish a modular package layout that does not mix HTTP/provider/domain concerns indiscriminately.
- Configuration is environment-driven and validated sufficiently to fail clearly on invalid required configuration.
- Unit test command exists and passes.

### Local infrastructure
- Docker Compose provisions PostgreSQL and S3-compatible local object storage for future tasks.
- Infrastructure has documented ports/credentials using development-only defaults or `.env.example` values.
- Application startup must not require production cloud credentials.

### Quality gates
- Root documentation explains the development bootstrap path.
- CI runs appropriate frontend and backend static/test checks on PRs.
- No committed secrets or generated dependency/build artifacts.

## Out of scope
- User authentication/product accounts.
- Project CRUD.
- AI provider integration.
- Media upload business flow.
- Background job implementation.
- Rendering.
- Channel publishing.
- Complete UI design system.

Do not invent those features in this task.

## Acceptance criteria
1. A fresh checkout can follow documented commands and start the required local infrastructure and both applications.
2. Frontend opens successfully and demonstrates router + Vietnamese i18n resource usage.
3. Backend exposes a working health endpoint and has passing tests.
4. PostgreSQL and local S3-compatible storage start through Docker Compose.
5. Frontend lint/typecheck/tests pass.
6. Backend format/vet/tests pass.
7. CI contains matching or equivalent verification for PRs.
8. No application secrets are committed.
9. Structure respects ADR 0005 and does not prematurely introduce microservices or provider-specific domain coupling.
10. PR targets `develop` and clearly reports all verification executed.

## Verification
At minimum run the repository's documented equivalents of:
- frontend install + lint + typecheck + tests/build;
- Go format/vet/tests/build;
- Docker Compose configuration validation/startup smoke test where environment permits.

If an expected verification cannot run in the coding environment, state exactly why in the PR rather than silently skipping it.

## Delivery
Open a PR from `feature/TASK-001-technical-foundation` to `develop`.
Do not self-merge and do not mark the task DONE.
