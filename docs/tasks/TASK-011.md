# TASK-011 — Script domain, persistence and approval API

Status: READY
Milestone: F1 Creative Workflow
Depends on: TASK-006 accepted
Wave: WAVE-F1-C
Branch: `feature/TASK-011-script-persistence`
Base: `develop`

## Goal
Start Creative Workflow Stage 5–6 by implementing the durable Script resource from an approved Proposal: versioned editable drafts, optimistic revision concurrency, immutable approval, PostgreSQL persistence and owner-isolated read/edit/approve APIs.

This task does not generate Script text with AI and does not create Scene Plans.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/contracts/SCRIPT_V1.md`
- `docs/contracts/AI_PROPOSAL_V1.md`
- `docs/product/CREATIVE_WORKFLOW.md`
- accepted Project/Proposal backend conventions.

## Frozen contract
`docs/contracts/SCRIPT_V1.md` is authoritative. Do not change it from this branch.

## Primary write paths
- `apps/api/internal/script/**`
- `apps/api/internal/postgres/script_repository*.go`
- `apps/api/internal/migrations/sql/0005_create_scripts.sql`
- Script-specific `apps/api/internal/httpserver/scripts*.go`.

## Allowed shared hotspots
- minimal Script service interface/route registration in `apps/api/internal/httpserver/server.go`;
- minimal service composition in `apps/api/cmd/api/main.go`.

## Reserved / do not touch
- `apps/api/internal/jobs/**` and migration `0004` — TASK-010.
- `apps/web/**` — TASK-008.
- Proposal generation integration/job routes — TASK-009.
- provider registry/adapters.
- accepted Proposal schema/contract.

## Scope
- Script domain types/validation matching `SCRIPT_V1`.
- Project-scoped monotonically increasing Script versions.
- Internal `CreateDraft` operation requiring an owner-visible **approved** Proposal source version.
- Persist `source_proposal_version` and Project `content_locale` snapshot.
- At most one active Script draft; newer draft supersedes only the previous unapproved draft atomically.
- Draft optimistic `revision` update.
- `draft|approved|superseded` statuses.
- Atomic approval with expected revision and durable `approved_at`.
- Newest-first version list and full version get.
- Owner isolation/non-disclosure through Project principal boundary.
- HTTP GET list/version, PUT draft replacement, POST approve exactly as frozen contract.
- Real PostgreSQL integration/concurrency coverage.

## Important invariants
- Script cannot be created from draft/superseded/non-visible Proposal.
- Approved Script content cannot be mutated.
- New draft creation never rewrites approved Script history.
- Concurrent draft creation cannot allocate duplicate versions or leave multiple active drafts.
- stale edit/approval cannot silently win.
- long-form Script sections are supported; do not introduce short-only assumptions.

## TDD plan
Start RED for at least:
1. approved Proposal -> first Script draft v1/revision1/draft;
2. non-approved Proposal -> rejected without Script creation;
3. second draft -> monotonic version + prior active draft superseded;
4. approved Script remains immutable and preserved;
5. current revision update increments exactly once;
6. stale update/approval -> conflict;
7. concurrent CreateDraft -> unique versions + one active draft;
8. owner A cannot list/get/update/approve/create from owner B resources;
9. version list newest first;
10. frozen section/length/cardinality validation, including long-form-friendly cases.

## Acceptance criteria
- [ ] Migration `0005_create_scripts.sql` works through current runner.
- [ ] Script source must be an approved owner-visible Proposal.
- [ ] Version/draft/approval invariants match `SCRIPT_V1`.
- [ ] Owner isolation and concurrency are proven against real PostgreSQL.
- [ ] Stable HTTP conflicts include `STALE_REVISION` and `SCRIPT_IMMUTABLE`.
- [ ] No AI generation/provider call/Scene Plan/frontend implementation is introduced.
- [ ] TDD evidence truthful and full CI green.

## Verification
At minimum targeted domain/service/API tests, real PostgreSQL integration/concurrency tests, gofmt, vet, `go test ./...`, backend build, full repository verification and `git diff --check`.

## Next dependencies
A later Script generation-engine task may build independently against `SCRIPT_V1`; later integration will persist generated candidates through this task's internal `CreateDraft`. Scene Plan work starts only from an approved Script version.

Do not self-merge or self-mark DONE.
