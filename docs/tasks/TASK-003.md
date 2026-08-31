# TASK-003 — Creative Brief backend and persistence

Status: BLOCKED
Milestone: F1 Creative Workflow
Depends on: TASK-002 accepted
Branch: `feature/TASK-003-creative-brief-api`
Base: `develop`
Wave: WAVE-F1-A

## Goal
Implement the durable owner-scoped Creative Brief V1 backend so creator intent can be created, reopened and safely updated before any AI proposal/generation begins.

## Why
Creative Brief is the first explicit human-in-the-loop creative state. Backend and frontend will be implemented in parallel against a frozen contract rather than defining behavior ad hoc in either branch.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/contracts/CREATIVE_BRIEF_V1.md`
- `docs/product/CREATIVE_WORKFLOW.md`
- `docs/engineering/ARCHITECTURE_PRINCIPLES.md`
- accepted TASK-002 project/principal/repository conventions after it merges

Do not recursively load unrelated docs.

## Integration contract
Provides the backend implementation of frozen `docs/contracts/CREATIVE_BRIEF_V1.md`.

The contract is not editable inside this task branch. If implementation discovers a contract defect, report it to PM/Team Lead rather than silently changing fields/status semantics.

## Parallel safety
### Primary write paths
- `apps/api/internal/creativebrief/**`
- Creative-Brief-specific migration files under the migration location established by TASK-002.

### Allowed shared integration files
- Minimal backend route/composition registration required to expose Creative Brief endpoints (for example the accepted HTTP server/router wiring from TASK-002).
- Existing project/principal interfaces only when a tiny additive seam is required; material refactoring is out of scope.

### Reserved / do not touch
- `apps/web/**` — owned by TASK-004 in this wave.
- AI/provider packages introduced by TASK-005.
- Project schema/behavior already accepted in TASK-002 except a strictly necessary foreign-key relationship.

TASK-003 can merge before TASK-004. TASK-004 final integration acceptance waits for this backend implementation to be merged.

## Scope
- Creative Brief V1 domain/value validation.
- PostgreSQL persistence with one brief per project and owner-scoped access via the accepted project/principal boundary.
- Atomic `revision` optimistic concurrency semantics.
- `GET /api/v1/projects/{project_id}/creative-brief`.
- `PUT /api/v1/projects/{project_id}/creative-brief` create/update semantics exactly as frozen contract.
- Standard repository error envelope mapping including validation, not-found and `STALE_REVISION` conflict.
- Migration(s), repository/service/HTTP boundaries and automated verification.

## Out of scope
- Any Vue/frontend implementation.
- AI Proposal generation or provider calls.
- Script/scenes/media/assets.
- Approval state.
- Attachments/source file ingestion.
- Changing Project format/locale fields.
- Full authentication product.

## Required behavior
- All access is scoped to the resolved owner/principal; another owner's project/brief has not-found semantics.
- First PUT creates revision 1 and returns 201.
- Existing PUT requires current revision, updates atomically and returns 200 with exactly one revision increment.
- Missing/stale revision on update cannot overwrite newer creator edits.
- Validation matches the frozen field limits and normalization rules.
- DB constraints/indexes support invariants safely enforceable at persistence level.
- Request cancellation propagates through service/repository/database calls.
- No client body may choose owner/project identity beyond route project id or server-controlled fields.

## TDD plan
Start with RED tests before production behavior, including at minimum:
1. domain validation/normalization cases from the frozen contract;
2. repository integration test proving one brief per project and owner isolation against real PostgreSQL;
3. concurrent/stale revision update test proving no lost update;
4. HTTP contract tests for GET, first PUT 201, update 200, validation 400, not-found 404, stale 409;
5. regression proof that accepted project/health behavior remains green.

Follow RED -> GREEN -> REFACTOR per behavior rather than implementing all layers first.

## Acceptance criteria
- [ ] Frozen Creative Brief V1 JSON/status contract is implemented without silent drift.
- [ ] Persistence survives process restart and uses PostgreSQL, not in-memory state.
- [ ] Owner isolation is tested against real persistence behavior.
- [ ] Revision concurrency prevents stale overwrite and is atomic.
- [ ] Migrations work on a clean database and CI/integration verification exercises them.
- [ ] HTTP/service/repository/domain concerns remain separated.
- [ ] No frontend/provider/AI-generation scope leaks into this PR.
- [ ] PR contains TDD evidence and repository verification is green.

## Open-source research
None required; this is product/domain persistence work. Normal maintained dependencies still require standard license/security hygiene.

## Verification
At minimum:
- targeted Creative Brief unit/HTTP tests;
- PostgreSQL integration tests with migrations on clean DB;
- concurrency/stale revision test;
- `gofmt`, `go vet`, `go test ./...`, backend build;
- existing CI/infrastructure checks affected by migration changes;
- `git diff --check` or equivalent.

## Delivery
PR to `develop` from `feature/TASK-003-creative-brief-api` must include:
- schema/migration summary;
- contract implementation notes;
- TDD evidence: RED/GREEN/REFACTOR;
- owner isolation and revision-concurrency verification;
- commands/results;
- any requested contract change as an explicit finding rather than an unreviewed implementation deviation.

Do not self-merge or mark DONE.
