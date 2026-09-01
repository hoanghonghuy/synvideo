# Media Library + Scene Assignment Workspace V1 Contract

Status: FROZEN for planned `TASK-024`.

This contract defines the creator-facing Stage 8 media library and manual scene-visual assignment workspace over accepted/planned Media Asset + Scene Media Binding APIs.

## Product boundary
The workspace lets creators:
- upload reusable project media;
- browse and preview project assets;
- inspect safe provenance/technical metadata;
- delete unreferenced creator assets with explicit confirmation;
- see approved Scene Plan scenes and whether each has a primary visual;
- assign/replace an image/video asset for a scene;
- inspect replacement history without losing previous choices.

It does not call AI image/video providers, search stock, synthesize audio, edit crop/fit/timing, render or publish.

## Routes and navigation
Primary project route:
`/projects/:id/media`

The Scene Plan workspace may deep-link to media selection for a specific scene using safe route/query state, but asset assignment state is always loaded from server APIs rather than trusted from query params.

## Initial load
Load:
- Project;
- media asset list;
- approved Scene Plan versions/current selected approved plan context;
- current primary visual bindings for the selected approved Scene Plan.

If no approved Scene Plan exists, the media library remains usable for upload/browse but scene assignment is disabled with guidance to complete/approve Stage 7.

Media load failure is not a false empty library. Scene binding failure must not hide already-loaded assets.

## Asset library
Display safe asset metadata only:
- thumbnail/preview where supported;
- original filename;
- kind/origin;
- byte size;
- creation time;
- safe provenance metadata when present.

Never display object key/bucket/storage endpoint/credentials.

V1 may provide client-side filters for kind/origin/recent assets. Do not invent server search semantics beyond the accepted API.

## Upload UX
Creator can upload one asset at a time in V1 with:
- file picker and drag/drop;
- explicit supported type/size guidance;
- upload progress where browser transport can report it;
- cancel support;
- validation vs network/storage error states;
- successful upload inserted/refreshed into the library without full-page reset.

Do not persist file bytes or sensitive browser file handles into local/session storage.

If API rejects oversize/unsupported type, keep the library stable and show localized actionable error.

## Preview
Images render through authorized Media Asset content endpoint.
Videos use browser playback against authorized content endpoint and must work with server byte ranges.

Audio may be previewable in the asset library if accepted upload types include audio, but audio cannot be assigned as V1 primary visual.

Preview failure for one asset must not fail the whole library.

## Delete UX
Delete requires explicit confirmation showing safe filename/type context.

On `MEDIA_ASSET_IN_USE`:
- do not remove asset locally;
- explain that active/history scene references preserve it;
- where possible show that it is referenced, without inventing destructive “force delete”.

Successful delete removes asset from current list/selection only after server confirmation.

## Scene assignment panel
Operate only on an approved Scene Plan version.

For every scene in Scene Plan order show:
- scene key / sequence;
- narration excerpt read-only;
- visual instruction;
- planned source type;
- current primary visual preview or explicit Unbound state.

Creator can choose any same-project image/video asset regardless of planned source type; actual origin/provenance remains visible.

Assignment/replacement:
- selecting already-active asset is harmless/idempotent;
- replacement does not delete prior Media Asset or binding history;
- after success refresh exact current binding/history for that scene;
- error preserves current selected visual.

## Replacement history
For one scene, creator can inspect prior primary-visual bindings newest-first with:
- binding version/status/time;
- safe asset preview/metadata/provenance.

History is read-only in V1. “Restore previous” may be implemented only as a normal new assignment of that historical asset, creating a new binding version rather than mutating history.

## Scene Plan version behavior
Bindings belong to an exact approved Scene Plan version.

When switching approved Scene Plan versions:
- reload current bindings for that exact version;
- do not copy bindings automatically between versions;
- clearly show when the newer plan has unbound scenes.

Stale/older approved Scene Plans remain inspectable with their historical bindings.

## State and concurrency
- assignment buttons disable per-scene while that scene request is in flight, not globally when unnecessary;
- concurrent external replacement may cause returned server state to differ; refresh server current/history and present latest state rather than pretending local selection won;
- route changes with pending upload/assignment should not corrupt another Project/Scene Plan state;
- no optimistic destructive deletion.

## i18n/accessibility/responsiveness
- Vietnamese localized creator-facing strings through existing resources;
- upload/preview/assignment/error status not color-only;
- keyboard accessible file/select/history controls;
- image alt/asset labels use safe filename/type/scene context;
- responsive long-form scene list should virtualize or avoid pathological rendering if hundreds of scenes/assets exist; implementation may choose progressive rendering/pagination but must remain correct.

## Frontend regression gates
1. true empty vs media-load failure;
2. upload supported file success and list refresh;
3. oversize/unsupported error preserves library;
4. upload cancel/transport failure;
5. image/video preview uses authorized content URLs; one preview failure isolated;
6. delete confirm; in-use conflict preserved/actionable;
7. no approved Scene Plan => library usable, assignment disabled;
8. current bindings preserve Scene Plan order/unbound entries;
9. assign first visual;
10. replace visual preserves old preview until success then refreshes;
11. same-asset assignment harmless;
12. history shown deterministically;
13. restore historical asset creates normal new assignment behavior;
14. switch approved Scene Plan versions reloads exact bindings/no auto-copy;
15. cross-route/project stale async responses cannot overwrite current workspace;
16. locale/typecheck/tests/build green.

## Scheduling gate
Contract is frozen now. TASK-024 remains BACKLOG/BLOCKED until TASK-022 releases frontend router/locale/navigation surfaces and TASK-023 Media/Binding API is accepted.