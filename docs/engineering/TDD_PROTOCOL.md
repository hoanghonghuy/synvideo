# Test-Driven Development Protocol

TDD is the default implementation discipline for SynVideo behavior changes.

## Core loop
For each observable behavior or defect:

1. **RED** — write or change the smallest meaningful test first and run it. Confirm it fails for the expected product/technical reason.
2. **GREEN** — implement the minimum production change needed to satisfy the test.
3. **REFACTOR** — improve structure/naming/duplication while keeping the targeted and relevant regression suite green.
4. Repeat for the next behavior.

Do not write a large implementation first and add tests afterward merely to satisfy CI.

## Required layers
Choose the lowest useful layer first, then add higher-level coverage for boundaries that carry material risk.

### Go/backend
- Domain rules: unit tests before implementation.
- Application/service behavior: service tests with controlled fakes where appropriate.
- HTTP contracts/error mapping: handler/API tests.
- PostgreSQL behavior, constraints, migrations, isolation and query semantics: integration tests against real PostgreSQL.
- Provider adapters: contract tests around the provider boundary; do not make live paid API calls in ordinary CI.

### Vue/frontend
- User-visible state/interaction: component or feature tests before implementation.
- Client/API mapping: client tests with deterministic transport mocks.
- Validation/error/loading/empty/retry states must be covered where they are acceptance behavior.
- Avoid snapshot-only tests as proof of behavior.
- Cross-stack E2E/smoke coverage is added when a task's integration risk requires it.

### Migrations/config/infrastructure
- For DB changes, define failing repository/integration expectations or migration verification before relying on the new schema.
- For validation/config changes, add the failing validation test first.
- Pure declarative infrastructure that cannot reasonably use RED/GREEN must still have automated validation and a documented verification step.

## TDD evidence in PRs
Every implementation PR must include a short `TDD evidence` section:
- RED: test(s) introduced/changed and what behavior failed before implementation;
- GREEN: targeted command(s) that pass after implementation;
- REFACTOR: notable cleanup, or `none`;
- full verification commands/results.

The PR does not need noisy one-commit-per-cycle history. The evidence is about development discipline and regression protection, not commit choreography.

## Exceptions
Tests may follow implementation only when test-first is genuinely impractical (for example a generated file or a purely declarative change with no executable seam). The PR must state the reason and the alternative verification. "Small change", "time", or "CI passes" are not sufficient exceptions.

## Review gate
Team Lead should request changes when production behavior is added or changed without meaningful automated regression coverage, unless an explicit justified exception applies.

There is no arbitrary repository-wide coverage percentage target. Test the behavior and risk surface, not a vanity number.
