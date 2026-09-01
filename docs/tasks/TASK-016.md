# TASK-016 — Media Asset + S3-compatible storage foundation

Status: READY
Milestone: F1 Creative Workflow
Depends on: TASK-002 accepted; frozen `MEDIA_ASSET_STORAGE_V1`
Wave: WAVE-F1-F
Branch: `feature/TASK-016-media-asset-storage`
Base: `develop`

## Goal
Implement the durable Stage 8 media metadata and vendor-neutral object-storage foundation, including an S3-compatible adapter/local development path, before scene-level generation/acquisition orchestration is added.

## Authoritative contract
`docs/contracts/MEDIA_ASSET_STORAGE_V1.md`

Read first:
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/decisions/0005-technical-baseline.md`
- `docs/contracts/MEDIA_ASSET_STORAGE_V1.md`
- accepted Project owner/isolation conventions.

## Primary ownership
- `apps/api/internal/mediaasset/**`;
- object-storage port and S3-compatible adapter under a cohesive media/storage infrastructure package;
- media-asset PostgreSQL repository;
- migration **`0008_create_media_assets.sql`**;
- local S3-compatible development/test infrastructure (MinIO or equivalent) where needed;
- task-specific tests.

## Mandatory isolation
Do not touch:
- `apps/api/cmd/api/main.go`;
- `apps/api/internal/httpserver/**`;
- `apps/web/**`;
- Proposal/Script/Scene Plan feature packages;
- generic jobs;
- AI provider registry/text-generation packages;
- TASK-009 paths;
- TASK-015 `sceneplan/**` or migration `0007`;
- render/publish code.

No public upload/download HTTP endpoints in this task.

## Scope
- Media Asset domain metadata and validation;
- owner/project-scoped PostgreSQL persistence;
- migration `0008_create_media_assets.sql`;
- object-store-neutral put/stat/open/delete interface;
- S3-compatible adapter suitable for local MinIO and future cloud S3-compatible services;
- safe credential/config/error boundary;
- server-generated traversal-safe object keys;
- streaming SHA-256 + byte-size calculation;
- create/store workflow with DB-failure compensating object delete;
- owner-authorized metadata get/list/open/delete;
- local deterministic storage integration without public internet.

## TDD plan
Truthful RED → GREEN → REFACTOR must cover at least:
1. asset domain kind/origin/mime/size/hash validation;
2. deterministic safe object-key generation and traversal rejection;
3. S3-compatible put/stat/open/delete against local deterministic infrastructure;
4. context cancellation/deadline propagation;
5. secret-free storage errors;
6. SHA-256 and byte-size correctness while streaming;
7. object write failure creates no DB metadata;
8. object succeeds + DB fails -> compensation delete attempted;
9. owner/project non-disclosure for metadata/content/delete;
10. list is bounded/newest-first according to repository contract;
11. deletion/error ordering is deterministic and tested;
12. real PostgreSQL constraints/indexes;
13. `go test -race`, integration tests, Docker Compose validation and `make verify`.

## Acceptance criteria
- [ ] Frozen `MEDIA_ASSET_STORAGE_V1` implemented without drift.
- [ ] Migration is exactly `0008_create_media_assets.sql`.
- [ ] Core media domain does not depend on AWS/MinIO SDK types.
- [ ] Credentials/endpoints/bucket internals do not leak into asset resources or presentation-safe errors.
- [ ] S3-compatible local path is deterministic and internet-free in CI/integration testing.
- [ ] Cross-owner/project object access is prevented at the metadata/service boundary.
- [ ] Failure windows and compensation behavior are explicitly tested.
- [ ] No HTTP/frontend/jobs/Scene Plan/provider-generation scope leakage.
- [ ] TDD evidence is truthful.

## Worktree
Atomically create the absent remote branch ref, then create/use a dedicated TASK-016 worktree. The shared control checkout stays on `develop`.

Do not self-merge or self-mark DONE.