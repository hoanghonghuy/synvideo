# TASK-020 — Scene media binding foundation

Status: DONE
Milestone: F1 Creative Workflow
Wave: WAVE-F1-H
Branch: `feature/TASK-020-scene-media-binding`
Base: `develop`
PR: #46
Accepted head: `3924f0693bedd3efd66e8d64d68ed4e04e53470c`
Logical TL APPROVE: `5079847789`
CI: #205 green on accepted head
Squash merge: `b80b8e7bb4dbf72145441d4794a989cfc0d8ff5f`
Issue: #38 closed completed
Depends on: TASK-015 and TASK-016 accepted; frozen `SCENE_MEDIA_BINDING_V1`.

## Goal
Create the durable Stage 8 relationship between approved Scene Plan scenes and their currently selected primary visual Media Assets, with owner/project isolation, atomic replacement, and preserved history for the later Scene Editor.

## Authoritative contract
`docs/contracts/SCENE_MEDIA_BINDING_V1.md`.

## Delivered
- `apps/api/internal/scenemedia/**` domain/service/repository contract;
- `apps/api/internal/postgres/scene_media_binding_repository.go` and real PostgreSQL coverage;
- migration `0011_create_scene_media_bindings.sql`;
- approved Scene Plan + exact scene-key gating;
- same-owner/same-project visual Media Asset validation;
- first assignment / same-active-asset idempotency;
- atomic replacement with append-only superseded history;
- transaction-scoped serialization and unique monotonic binding versions;
- one active primary visual per Scene Plan scene/role;
- deterministic current view preserving Scene Plan scene order and explicit unbound scenes;
- restrictive Media Asset reference integrity.

## Accepted invariants
1. Bindings never edit approved Scene Plan JSON.
2. Only approved Scene Plan versions accept new bindings.
3. Owner/project boundaries hold at service and DB levels through scoped reads and composite foreign keys.
4. Image/video are valid primary visuals; audio/document/other are rejected.
5. Planned source type remains intent rather than a hard lock on actual asset origin.
6. Replacing media preserves history instead of overwriting old assignment rows.
7. Concurrent replacements leave exactly one active row with unique monotonic versions.
8. Referenced assets cannot be silently deleted into dangling binding history.

## Verification accepted
- [x] approved-plan assignment success;
- [x] draft/nonexistent plan rejection and exact scene-key validation;
- [x] cross-owner/cross-project asset rejection;
- [x] image/video accepted and nonvisual kinds rejected;
- [x] planned-source creator override allowed;
- [x] first assignment version 1;
- [x] same-asset assignment idempotent;
- [x] replacement supersedes prior binding and increments version;
- [x] deterministic replacement history;
- [x] plan-level ordered active/unbound view;
- [x] real PostgreSQL concurrent replacement serialization;
- [x] DB cross-project and one-active constraints;
- [x] referenced asset deletion restricted;
- [x] exact-head CI #205 green.

No HTTP/frontend/jobs/Script/provider/runtime/render/publish scope was added. TASK-020 is complete; future public API/UI integration is owned by planned TASK-023/TASK-024.