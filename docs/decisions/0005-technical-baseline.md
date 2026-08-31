# ADR 0005 — Initial Technical Baseline

Status: Accepted

## Decision
SynVideo starts as a web application with a separable frontend/backend architecture and explicit infrastructure boundaries.

Initial baseline:
- Frontend: Vue 3 + TypeScript, using a modern Vite-based setup.
- Backend API: Go.
- Primary relational database: PostgreSQL.
- Object/media storage: S3-compatible abstraction; local development may use MinIO or another compatible local service.
- Background/long-running work: explicit job abstraction. Do not bind domain behavior directly to one queue vendor in the first task.
- Local development: Docker Compose for required infrastructure.
- API contracts: stable versioned HTTP API boundary; implementation may add generated/OpenAPI documentation where appropriate.
- Authentication: architecture must support secure server-side authorization; the initial scaffold does not need to implement a complete auth product unless explicitly tasked.
- Frontend i18n: installed/configured from the first frontend scaffold. Vietnamese is the initial required locale; English can be added progressively.

## Repository shape
Prefer a simple monorepo unless a concrete reason requires otherwise, for example:

```text
apps/
  web/
  api/
packages/        # only when shared packages become justified
infra/
docs/
```

Exact folder names may vary if the implementation task documents a better equivalent, but frontend/backend/infrastructure boundaries must remain clear.

## Why
SynVideo will contain UI-heavy editing, durable project/domain state, asynchronous AI/media operations, provider adapters, rendering and publishing integrations. Keeping the web client and backend boundaries explicit makes those concerns easier to test, evolve and deploy independently without prematurely introducing microservices.

## Non-decisions
This ADR does not lock:
- a specific Go HTTP framework;
- ORM/query library;
- queue implementation;
- cloud vendor;
- AI provider;
- rendering provider/engine;
- deployment platform.

Those choices should be introduced only when a task has enough requirements to evaluate them.

## Constraints
- No microservices by default. Start modular and split only when operational evidence justifies it.
- Provider-specific SDK types must not leak into core domain contracts.
- Secrets must remain server-side where required and never be committed.
- Long-running generation/render/publish work must evolve toward explicit jobs rather than blocking HTTP requests.
- User-facing strings must not create hard-coded language debt.
