# TASK-020 — Scene media binding foundation

Status: READY
Milestone: F1 Creative Workflow
Wave: WAVE-F1-H
Branch: `feature/TASK-020-scene-media-binding`
Base: `develop`
Depends on: TASK-015 and TASK-016 accepted; frozen `SCENE_MEDIA_BINDING_V1`.

## Goal
Create the durable Stage 8 relationship between approved Scene Plan scenes and their currently selected primary visual Media Assets, with owner/project isolation, atomic replacement, and preserved history for the later Scene Editor.

## Authoritative contract
`docs/contracts/SCENE_MEDIA_BINDING_V1.md`.

Read first:
- `AGENTS.md`;
- `docs/engineering/TDD_PROTOCOL.md`;
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`;
- `docs/contracts/SCENE_PLAN_V1.md`;
- `docs/contracts/MEDIA_ASSET_STORAGE_V1.md`;
- `docs/contracts/SCENE_MEDIA_BINDING_V1.md`;
- accepted TASK-015 Scene Plan persistence and TASK-016 Media Asset persistence/storage.

## Primary ownership
- new `apps/api/internal/scenemedia/**` or cohesive equivalent;
- `apps/api/internal/postgres/scene_media_binding_repository.go` and focused integration tests;
- migration exactly `0011_create_scene_media_bindings.sql`;
- minimal repository-facing read adapters/interfaces for accepted Scene Plan and Media Asset data;
- domain/PostgreSQL/race tests required by the frozen contract.

## Mandatory isolation
Do not modify:
- `apps/api/cmd/api/main.go`;
- `apps/api/internal/httpserver/**`;
- `apps/web/**`;
- generic jobs;
- Script/TASK-018 paths;
- provider settings/runtime;
- Scene Plan generation engine;
- Media Asset S3/storage adapter behavior;
- render/publish paths.

## Required capability
Implement durable primary-visual binding semantics for an approved Scene Plan:
- validate approved plan + exact scene key;
- validate same-owner/same-project image/video asset;
- first assignment version 1;
- same-active-asset idempotency;
- atomic replacement with prior row superseded and next monotonic binding version;
- one active primary visual per scene;
- deterministic active-plan view preserving Scene Plan scene order and unbound scenes;
- deterministic replacement history;
- restrictive asset reference integrity.

## Critical invariants
1. Bindings never edit approved Scene Plan JSON.
2. Only approved Scene Plan versions accept new bindings.
3. Same-owner/same-project boundaries hold at service/repository and DB levels where practical.
4. Image/video are valid primary visuals; audio/document/other are not.
5. Planned source type is intent, not a hard lock on actual asset origin.
6. Replacing media preserves history instead of overwriting old assignment rows.
7. Concurrent replacements leave exactly one active row with unique monotonic binding versions.
8. Referenced assets cannot be silently deleted into dangling binding history.

## TDD plan
Truthful RED → GREEN → REFACTOR must cover at least:
- approved-plan assignment success;
- draft/superseded/nonexistent plan rejection;
- unknown scene key;
- cross-owner/cross-project asset rejection;
- image/video accepted and nonvisual kinds rejected;
- planned-source override allowed;
- first assignment version 1;
- same-asset idempotency;
- replacement supersede/version increment;
- deterministic history;
- plan-level ordered active/unbound view;
- concurrent replacement serialization;
- DB cross-project/one-active constraints;
- referenced asset restrictive deletion behavior;
- real PostgreSQL and `go test -race ./...`.

## Acceptance criteria
- [ ] `SCENE_MEDIA_BINDING_V1` implemented without drift.
- [ ] Migration is exactly `0011_create_scene_media_bindings.sql`.
- [ ] Scene/media owner/project integrity is enforced and tested.
- [ ] Replacement is atomic, versioned and history-preserving.
- [ ] No HTTP/frontend/jobs/TASK-018/TASK-019 write-surface leakage.
- [ ] PostgreSQL integration, race tests and full CI are green.

## Worktree / claim
Before work, confirm remote `feature/TASK-020-scene-media-binding` does not exist. Atomically create that remote ref from latest `origin/develop`, then use a dedicated TASK-020 worktree. Shared/control checkout remains on `develop`.

Do not self-merge or self-mark DONE.