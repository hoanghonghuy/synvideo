# TASK-003 — Creative Brief backend and persistence

Status: DONE
Milestone: F1 Creative Workflow
Depends on: TASK-002 accepted
Branch: `feature/TASK-003-creative-brief-api`
Base: `develop`
Wave: WAVE-F1-A
Accepted via: PR #9

## Goal
Implement the durable owner-scoped Creative Brief V1 backend so creator intent can be created, reopened and safely updated before any AI proposal/generation begins.

## Why
Creative Brief is the first explicit human-in-the-loop creative state. Backend and frontend are implemented against a frozen contract rather than defining behavior ad hoc in either branch.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/contracts/CREATIVE_BRIEF_V1.md`
- `docs/product/CREATIVE_WORKFLOW.md`
- `docs/engineering/ARCHITECTURE_PRINCIPLES.md`
- accepted TASK-002 project/principal/repository conventions

Do not recursively load unrelated docs.

## Integration contract
Provides the backend implementation of frozen `docs/contracts/CREATIVE_BRIEF_V1.md`.

The contract is not editable inside this task branch. Contract changes require PM/Team Lead coordination on `develop`.

## Parallel safety
### Primary write paths
- `apps/api/internal/creativebrief/**`
- Creative-Brief-specific migration files under the migration location established by TASK-002.

### Allowed shared integration files
- Minimal backend route/composition registration required to expose Creative Brief endpoints.
- Existing project/principal interfaces only when a tiny additive seam is required.

### Reserved / do not touch
- `apps/web/**` — owned by TASK-004 in this wave.
- AI/provider packages introduced by TASK-005.
- Project schema/behavior already accepted in TASK-002 except a strictly necessary foreign-key relationship.

## Scope delivered
- Creative Brief V1 domain/value validation.
- PostgreSQL persistence with one brief per project and owner-scoped access via the accepted project/principal boundary.
- Atomic `revision` optimistic concurrency semantics.
- `GET /api/v1/projects/{project_id}/creative-brief`.
- `PUT /api/v1/projects/{project_id}/creative-brief` create/update semantics exactly as frozen contract.
- Standard repository error envelope mapping including validation, not-found and `STALE_REVISION` conflict.
- Migration, repository/service/HTTP boundaries and automated verification.

## Required behavior accepted
- All access is scoped to the resolved owner/principal; another owner's project/brief has not-found semantics.
- First PUT creates revision 1 and returns 201.
- Existing PUT requires current revision, updates atomically and returns 200 with exactly one revision increment.
- Missing/stale revision cannot overwrite newer creator edits.
- Validation matches the frozen field limits and normalization rules.
- DB constraints support persistence-level invariants.
- Request cancellation propagates through service/repository/database calls.
- Client bodies cannot choose owner/project identity beyond route project id or server-controlled fields.

## TDD acceptance
PR #9 supplied RED -> GREEN -> REFACTOR evidence across domain validation, service behavior, HTTP contract and PostgreSQL integration. Real PostgreSQL tests prove owner isolation and concurrent stale-revision behavior.

## Acceptance criteria
- [x] Frozen Creative Brief V1 JSON/status contract is implemented without silent drift.
- [x] Persistence survives process restart and uses PostgreSQL, not in-memory state.
- [x] Owner isolation is tested against real persistence behavior.
- [x] Revision concurrency prevents stale overwrite and is atomic.
- [x] Migrations work on a clean database and CI/integration verification exercises them.
- [x] HTTP/service/repository/domain concerns remain separated.
- [x] No frontend/provider/AI-generation scope leaks into this PR.
- [x] PR contains TDD evidence and repository verification is green.

## Verification accepted
- targeted Creative Brief unit/HTTP tests;
- PostgreSQL integration tests with migrations on clean DB;
- concurrent/stale revision test;
- `gofmt`, `go vet`, `go test ./...`, backend build;
- existing CI/infrastructure checks affected by migration changes;
- `git diff --check`.

## Delivery
Accepted by Team Lead and squash-merged into `develop` via PR #9.
