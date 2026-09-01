# Scene Media Binding V1 Contract

Status: FROZEN for `WAVE-F1-H`.

This contract establishes the durable relationship between an approved Scene Plan scene and the concrete visual Media Asset currently selected for that scene. It is the persistence foundation for Stage 8 media acquisition and the later Scene Editor.

## Product boundary
V1 provides:
- one current primary visual assignment per scene of an approved Scene Plan version;
- append-only replacement history;
- owner/project isolation across Scene Plan and Media Asset boundaries;
- deterministic list/history behavior;
- safe replacement semantics that never mutate approved Scene Plan content or Media Asset metadata.

V1 does not expose HTTP/UI, call stock/generation providers, upload files, generate audio, render video, or implement the Scene Editor.

## Source boundary
Bindings attach only to an owner-visible **approved** Scene Plan version.

The Scene Plan is authoritative for:
- `project_id`;
- `scene_plan_version`;
- valid `scene_key` values;
- narration/visual instruction/planned source intent.

A binding never edits Scene Plan JSON. If scene structure changes, the creator must create/approve a new Scene Plan version; bindings remain historical against the old version.

## Media boundary
The selected asset must:
- exist;
- belong to the same owner and project;
- be a visual asset (`image` or `video`) in V1.

The Scene Plan `planned_source_type` is planning intent, not a hard origin lock. A creator may replace a planned stock/generated visual with an upload or other valid visual asset. Actual provenance remains on `media_assets.origin`.

Audio/document/other assets cannot be used as the V1 primary visual binding.

## Durable model
Migration: `0011_create_scene_media_bindings.sql`.

Each binding row contains at least:
- `id uuid`;
- `owner_id uuid`;
- `project_id uuid`;
- `scene_plan_version integer`;
- `scene_key text`;
- `role text` fixed to `primary_visual` in V1;
- positive `binding_version integer` monotonic for the same scene/role;
- `asset_id uuid`;
- `status text` = `active|superseded`;
- `created_at`;
- optional `superseded_at`.

Database requirements:
- FK `(project_id, scene_plan_version)` -> `scene_plans(project_id, version)`;
- same-project asset integrity enforced at DB level where practical, including a supporting unique key on media assets if a composite FK is required;
- unique `(owner_id, project_id, scene_plan_version, scene_key, role, binding_version)`;
- at most one `active` row for `(owner_id, project_id, scene_plan_version, scene_key, role)`;
- positive versions and allowed role/status checks;
- no cascade behavior may accidentally delete a Media Asset when a binding is removed/history is cleaned.

The repository/service must also verify the project owner boundary because current Scene Plan persistence does not carry `owner_id` directly.

## Assignment semantics
`AssignPrimaryVisual(owner, project, scene_plan_version, scene_key, asset_id)` is the cohesive domain operation.

Rules:
1. principal/owner is required;
2. Scene Plan version must exist for the project and be approved;
3. `scene_key` must exist exactly in that approved plan;
4. Media Asset must exist for the same owner/project and be image/video;
5. if no active binding exists, create binding version 1;
6. replacing an active binding atomically marks the prior row `superseded`, sets its `superseded_at`, and inserts the next binding version;
7. assigning the already-active asset is idempotent and returns the existing active binding without creating history noise;
8. concurrent replacements must serialize so exactly one next version becomes active and no duplicate active rows are possible.

Do not overwrite or delete superseded binding rows during normal replacement.

## Read semantics
Required repository/service operations:
- get current primary visual for one scene;
- list current primary visuals for an approved Scene Plan version in Scene Plan scene order;
- list replacement history for one scene newest-first or version-descending deterministically.

Unbound scenes are valid and appear as unbound in a plan-level current-binding result rather than being silently omitted when the caller requests complete scene coverage.

Owner/project mismatches are non-disclosing not-found style failures.

## Asset deletion interaction
V1 binding rows reference Media Assets with restrictive semantics while the asset is actively or historically referenced.

Normal Media Asset deletion must not silently destroy binding history or leave dangling references. If the existing Media Asset service cannot yet surface a friendly “asset in use” error without crossing TASK-020 ownership, DB `RESTRICT` behavior is acceptable and a later integration task may map it at HTTP/service level.

TASK-020 must not weaken Media Asset owner/project validation to make deletion easier.

## Error semantics
Stable domain-level categories include:
- unauthenticated;
- scene plan not found/non-visible;
- scene plan not approved;
- scene key not found;
- media asset not found/non-visible;
- media asset kind invalid for primary visual;
- persistence/concurrency failure.

Raw SQL/storage errors must not leak through presentation-facing errors.

## Deterministic verification
Required coverage includes:
1. assignment to approved plan scene succeeds;
2. draft/superseded/nonexistent plan cannot receive a new active binding;
3. unknown scene key rejected;
4. cross-owner/cross-project asset rejected;
5. image/video accepted; audio/document/other rejected;
6. planned source type does not wrongly block a valid creator override;
7. first assignment creates binding version 1;
8. same active asset assignment is idempotent;
9. replacement supersedes prior row and increments version exactly once;
10. replacement history is preserved and deterministic;
11. plan-level current list preserves Scene Plan order and represents unbound scenes;
12. concurrent replacements produce one active binding and monotonic unique versions;
13. DB constraints prevent cross-project asset linkage and multiple active rows;
14. referenced Media Asset cannot be silently deleted into dangling history;
15. real PostgreSQL integration and `go test -race ./...` are green.

## Ownership boundary
TASK-020 owns primarily:
- new `apps/api/internal/scenemedia/**` or cohesive equivalent;
- `apps/api/internal/postgres/scene_media_binding_repository.go` and focused integration tests;
- migration `0011_create_scene_media_bindings.sql`;
- only the minimum repository-facing adapters/interfaces needed to read accepted Scene Plan and Media Asset data.

TASK-020 must not modify `apps/api/cmd/api/main.go`, `apps/api/internal/httpserver/**`, `apps/web/**`, generic jobs, Script generation paths, provider runtime/settings, Scene Plan generation engine, Media Asset storage adapter semantics, render or publishing.