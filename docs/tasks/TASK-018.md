# TASK-018 — Script durable generation integration

Status: READY
Milestone: F1 Creative Workflow
Wave: WAVE-F1-H
Branch: `feature/TASK-018-script-generation-integration`
Base: `develop`
Depends on: TASK-010, TASK-011, TASK-012, TASK-017 accepted; frozen `SCRIPT_JOB_V1`.

## Goal
Make Script generation a real asynchronous backend capability by integrating the accepted Script generation engine with durable jobs, owner-scoped BYOK runtime, crash-safe idempotent Script persistence, feature-specific HTTP endpoints, and runtime composition.

## Authoritative contract
`docs/contracts/SCRIPT_JOB_V1.md`.

Read first:
- `AGENTS.md`;
- `docs/engineering/TDD_PROTOCOL.md`;
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`;
- `docs/contracts/SCRIPT_V1.md`;
- `docs/contracts/SCRIPT_GENERATION_V1.md`;
- `docs/contracts/JOB_EXECUTION_V1.md`;
- `docs/contracts/BYOK_TEXT_PROVIDER_RUNTIME_V1.md`;
- `docs/contracts/SCRIPT_JOB_V1.md`;
- accepted Proposal job implementation as a behavioral reference, not a copy-paste mandate.

## Primary ownership
- new `apps/api/internal/scriptgenerationjob/**` or cohesive equivalent;
- minimal internal Script generation-idempotency extension;
- `apps/api/internal/postgres/script_repository.go` + focused tests;
- migration exactly `0010_add_script_generation_idempotency.sql`;
- Script-generation HTTP handler/tests;
- minimal `apps/api/internal/httpserver/server.go` composition;
- minimal `apps/api/cmd/api/main.go` jobs registry/service wiring;
- backend/PostgreSQL/security/race tests required by the frozen contract.

## Mandatory isolation
Do not modify:
- `apps/web/**` — owned by TASK-019;
- `apps/api/internal/scenemedia/**` or migration `0011` — owned by TASK-020;
- Scene Plan generation/domain behavior;
- Media Asset storage semantics;
- provider-settings credential lifecycle;
- render/publish paths.

## Required capability
Implement the frozen:
- `POST /api/v1/projects/{id}/script-generations`;
- `GET /api/v1/projects/{id}/script-generations/{job_id}`;
- job kind `script_generation_v1`;
- request-time highest approved Proposal + Project snapshot;
- owner runtime validation and worker-side credential resolution;
- exactly-once Script draft persistence by generation job ID;
- safe feature-specific status/result shape;
- runtime registration in the existing generic executor.

## Critical invariants
1. HTTP POST never calls a provider.
2. Same request ID replay returns the original durable job before current Proposal/provider/credential checks.
3. Durable payload/result are credential-free.
4. Worker uses the snapshotted approved Proposal, not “latest” at execution time.
5. `source_generation_job_id` is internal and DB-unique, never public JSON.
6. Crash after Script draft commit but before `MarkSuccess` cannot create another Script version.
7. Approved Script history is immutable; only active unapproved draft supersede behavior follows accepted Script rules.
8. TASK-017 owner runtime is reused; no second provider/secret path.

## TDD plan
Truthful RED → GREEN → REFACTOR must cover at least:
- no-approved-Proposal rejection;
- exact highest-approved-Proposal snapshot;
- owner/project isolation;
- same-request replay after Proposal/provider/credential changes;
- conflicting request reuse;
- duplicate-enqueue race same/conflicting selection;
- strict durable payload decoding;
- provider unavailable/failed/invalid output mapping;
- local `httptest` owner credential execution;
- secret-free durable bytes/errors;
- PostgreSQL generation-job uniqueness;
- crash-window retry exactly-once Script persistence;
- public JSON omission of internal job ID;
- executor registration/runtime smoke;
- `go test -race ./...` and full `make verify`.

## Acceptance criteria
- [ ] `SCRIPT_JOB_V1` implemented without drift.
- [ ] Migration is exactly `0010_add_script_generation_idempotency.sql`.
- [ ] Script generation is durable, retryable and owner-scoped.
- [ ] Request-id replay semantics match the frozen contract under races and configuration changes.
- [ ] Script draft persistence is DB-idempotent across worker crash/reclaim.
- [ ] No secret/provider raw response leaks into jobs, HTTP or errors.
- [ ] Generic jobs executor runs Proposal and Script job kinds together without regression.
- [ ] No TASK-019/TASK-020 write-surface leakage.
- [ ] Targeted tests, race tests and full CI are green.

## Worktree / claim
Before work, confirm remote `feature/TASK-018-script-generation-integration` does not exist. Atomically create that remote ref from latest `origin/develop`, then use a dedicated TASK-018 worktree. Shared/control checkout remains on `develop`.

Do not self-merge or self-mark DONE.