# SCENE_EDITOR_COMPOSITION_V1

Status: FROZEN FOR IMPLEMENTATION
Applies to: TASK-036 Scene Editor composition/API core
Downstream consumer: TASK-037 Render & Export V1
Creator-workspace follow-up: TASK-050 / issue #141
Upstream narration/script bridge follow-up: TASK-051 / issue #142

## Purpose

Define the durable, render-engine-neutral Scene Editor composition boundary that turns accepted project resources into one versioned editable composition and an immutable render-input snapshot.

TASK-036 owns the composition/API core only. It must not silently mutate approved upstream history, accept browser-provided dependency identities as authority, or persist renderer-specific commands.

## Delivery boundary

TASK-036 core includes:
- versioned composition persistence with optimistic revision control;
- stable composition-local scene identity;
- reorder, duplicate and remove operations;
- exact visual, narration, caption and audio-mix references;
- bounded render-neutral presentation/timing/transition semantics;
- deterministic `CURRENT | STALE | BROKEN` dependency state;
- explicit exact-`scene_key` reconciliation preview/apply;
- immutable deterministic render-input snapshots;
- authenticated project isolation and authoritative dependency validation;
- API runtime wiring and representative tests.

The following are intentionally not merge blockers for TASK-036 after the task-sizing split:
- TASK-050: completed creator workspace, semantic browser preview, dirty/saving/saved/conflict/recovery UX and accessibility;
- TASK-051: integrated upstream Script/Scene Plan narration-edit bridge.

Those child tasks consume this core contract and may extend creator UX, but they must not weaken the persistence, identity, reconciliation or snapshot rules defined here.

## Upstream authority and non-mutation

A composition records accepted upstream identities; it is never a second source of truth for Script, Scene Plan, media provenance, narration generation, captions or audio-mix history.

Authoritative upstream identities include, where applicable:
- accepted Scene Plan version + exact `scene_key`;
- visual scene-media binding ID + exact MediaAsset ID;
- narration binding ID + exact audio MediaAsset ID + deterministic narration lineage + authoritative duration;
- caption document ID + exact revision + narration lineage + authoritative last segment end;
- audio-mix document ID + exact revision + music MediaAsset ID + narration lineage.

A newer upstream assignment/revision never silently rewrites a saved composition. Historical same-project dependencies may become `STALE`; missing, cross-project, structurally invalid or unverifiable dependencies become `BROKEN`.

## Composition document

The live editable composition contains:
- stable `composition_id`;
- authenticated owner/project boundary;
- positive `revision`;
- exact `scene_plan_version`;
- ordered scene instances;
- optional exact audio-mix reference;
- created/updated timestamps.

There is one current editable composition per project in V1. Historical render snapshots are immutable separate records.

All timing at domain/API boundaries is integer milliseconds.

## Scene instance

Each scene instance records:
- stable composition-local scene ID;
- exact source `scene_key`;
- optional exact visual dependency;
- optional exact narration dependency;
- optional exact caption dependency;
- positive output duration in milliseconds;
- visual treatment;
- transition-out semantics;
- optional creator note.

Composition-local scene IDs remain stable across ordinary edits and reorder. Duplicate creates a new local ID while preserving upstream references. Remove affects only the composition. A valid composition always contains at least one scene.

Split/merge is not a persisted V1 operation; semantic source split/merge belongs to the upstream Scene Plan workflow.

## Authoritative write boundary

Create, Update, reconciliation preview and reconciliation apply must validate the exact dependency identities they are about to use against authoritative same-owner, same-project PostgreSQL records before persistence/apply.

Rules:
1. browser JSON is input, never authority;
2. cross-owner/project or missing exact identities fail safely without disclosing foreign metadata;
3. an exact historical same-project identity may be accepted and represented as `STALE` when it is no longer production-current;
4. `BROKEN` dependencies are rejected at write/reconciliation boundaries;
5. unrelated presentation edits may still be saved against verifiable historical `STALE` dependencies so creator work is preserved;
6. snapshot creation requires the resulting composition to be fully `CURRENT`.

## Visual dependency

A visual reference is `{binding_id, asset_id}` for one exact same-project `primary_visual` scene binding.

The referenced MediaAsset must:
- belong to the same owner/project;
- be `image` or `video`;
- not be deletion-requested.

If the exact historical binding/asset exists but another active binding is current, state is `STALE`. Random/cross-project/missing pairs are `BROKEN`.

## Narration dependency

A narration reference contains:
- exact narration `binding_id`;
- exact audio `asset_id`;
- deterministic per-scene `lineage_id`;
- authoritative `duration_ms`.

The referenced binding and asset must belong to the same owner/project, exact Scene Plan version and exact `scene_key`; the asset must be readable audio.

### Narration lineage V1

For an exact narration binding, Scene Editor V1 derives the per-scene lineage UUID deterministically using UUIDv5/SHA-1 (`uuid.NewSHA1` with the OID namespace) from the UTF-8 string:

`scene-editor-narration-v1|plan:<scene_plan_version>|scene:<scene_key>|binding:<binding_uuid>|asset:<asset_uuid>|duration_ms:<authoritative_duration_ms>`

The browser may transmit this value for round-trip purposes, but the server re-derives and verifies it. It is not trusted merely because it is a valid UUID.

`duration_ms` is derived from authoritative MediaAsset duration metadata and must match exactly. A client-supplied shorter/longer value is invalid and cannot be used to bypass scene-duration rules.

An exact superseded narration binding remains verifiable historical identity and yields `STALE`; an unresolvable identity, duration mismatch or lineage mismatch yields `BROKEN`.

## Caption dependency

A caption reference contains:
- exact caption `document_id`;
- exact positive `revision`;
- narration `lineage_id`;
- authoritative `last_end_ms`.

The caption document/revision must belong to the same owner/project, Scene Plan version and `scene_key`. Its stored source binding/asset must resolve to the same project narration source.

Caption `lineage_id` is the same deterministic per-scene narration lineage derived from the caption document's stored source binding, source asset and source duration.

`last_end_ms` is re-derived from the persisted caption segments as the maximum segment `end_ms` and must match exactly. Client timing summaries are not authoritative.

A superseded source narration binding or non-latest caption revision yields `STALE`. Missing/cross-project source, lineage mismatch or timing mismatch yields `BROKEN`.

## Project audio mix

An audio-mix reference contains:
- exact document ID;
- exact revision;
- exact music asset ID;
- exact narration lineage ID recorded by the audio-mix document.

The server verifies all four fields against the same-project audio-mix revision and verifies that the music asset remains readable audio. A newer mix revision makes an older valid reference `STALE`; identity mismatch/missing data is `BROKEN`.

## Scene duration

Scene duration is explicit positive integer milliseconds.

Without an explicit trim/time-stretch contract:
- scene duration must be at least authoritative narration duration when narration is enabled;
- scene duration must cover authoritative caption `last_end_ms` when captions are enabled;
- invalid client-supplied dependency timing cannot be used to bypass these checks;
- Scene Editor never silently truncates or time-stretches narration/captions.

## Visual treatment bounds

V1 uses the following frozen render-neutral bounds and defaults.

### Fit
- allowed: `contain | cover`;
- creator default: `contain` unless an accepted product-specific default is supplied by the caller;
- unsupported values are validation errors.

### Position
- `position_x`: inclusive `[-1.0, +1.0]`;
- `position_y`: inclusive `[-1.0, +1.0]`;
- default: `0.0` for each axis;
- out-of-range values are rejected; no silent clamping.

### Scale
- inclusive `[0.25, 4.0]`;
- default: `1.0`;
- out-of-range values are rejected; no silent clamping.

### Crop
Crop, when present, is normalized and deterministic. It must remain inside the accepted normalized source rectangle and must not be translated into CSS/renderer-specific filters in the persisted contract.

### Source-video audio
`mute_video` is a render-neutral boolean only. It does not mutate the source MediaAsset.

## Transitions

Allowed V1 transitions:
- `cut`;
- `fade`;
- `crossfade`.

Frozen duration rules:
- `cut`: exactly `0 ms`;
- `fade` / `crossfade`: inclusive `[100, 2000] ms`;
- creator-control default for fade/crossfade: `300 ms`;
- transition duration must also fit both adjacent scene durations;
- effective maximum is therefore `min(2000 ms, adjacent-scene-fit limit)`;
- invalid values are rejected, never silently rewritten.

If a downstream renderer cannot faithfully implement a frozen transition, TASK-037 must fail/narrow activation rather than silently substitute another effect.

## Dependency state

A composition is `CURRENT` only when all recorded dependencies are production-current for the recorded source lineage.

Typical `STALE` reasons:
- a newer accepted Scene Plan supersedes the recorded plan;
- visual or narration binding was replaced;
- caption source/revision was superseded;
- a newer audio-mix revision exists.

Typical `BROKEN` reasons:
- exact identity cannot be resolved within owner/project boundary;
- referenced asset/revision is missing or unavailable;
- media kind is invalid for the selected role;
- authoritative duration/lineage/caption timing does not match the submitted reference.

`STALE` and `BROKEN` state never silently changes stored creator presentation edits.

## Reconciliation

Reconciliation is explicit and revision-checked.

Preview/apply may auto-map only exact stable `scene_key` matches. Removed, renamed, split, merged or duplicate candidate scene keys are ambiguous and require creator choice/new composition.

Applying reconciliation:
- creates one new composition revision;
- changes accepted dependency references only;
- preserves unrelated creator presentation edits/local scene IDs when mapping is unambiguous;
- validates candidate dependencies through the authoritative write boundary before persistence;
- never trusts browser candidate UUIDs merely because they are structurally valid.

## Optimistic concurrency

Persisted writes require `expected_revision`.

Rules:
- exactly one successful logical save increments revision once;
- stale writers receive deterministic conflict;
- stale writers cannot overwrite newer work;
- retries/recovery must re-read durable revision before issuing a new logical mutation when outcome is ambiguous.

TASK-050 owns the complete dirty/saving/saved/conflict/recovery creator UX built on these server semantics.

## Immutable render-input snapshot

TASK-037 consumes an explicit immutable snapshot of one saved composition revision.

The snapshot records at least:
- schema version;
- composition ID + revision;
- project ID;
- source Scene Plan version;
- ordered scene instances;
- exact dependency identities and authoritative timing already persisted by the composition;
- visual treatment and transitions;
- exact audio-mix reference;
- deterministic canonical SHA-256 digest.

Snapshot creation is rejected unless dependency state is `CURRENT`. Later upstream changes/editor saves never mutate an existing snapshot.

Render-engine-specific codec, bitrate, encoder flags, provider payloads and deployment/runtime settings are outside this contract.

## Isolation and privacy

Every read/write/reconcile/snapshot operation is scoped through authenticated owner/project identity.

The API must not expose raw provider payloads, secrets, signed URLs, storage keys or foreign-project metadata in dependency diagnostics.

## API truthfulness

Server responses must truthfully distinguish validation failure, unauthenticated access, not found, optimistic conflict, ambiguous reconciliation, snapshot blocked and persistence failure.

The core API must not claim a dependency is accepted/current solely from browser input.

TASK-050 owns complete creator-facing loading/dirty/saving/conflict/preview/accessibility presentation.

## Core acceptance / regression gates

TASK-036 core requires representative coverage for:
1. create/load exact composition without mutating upstream state;
2. stable local scene identity;
3. reorder round-trip;
4. duplicate creates independent local ID without duplicating upstream/provider work;
5. remove is composition-local and last-scene removal is rejected;
6. visual project/kind/identity validation;
7. frozen position/scale/transition bounds and no silent clamp;
8. authoritative narration duration + lineage verification;
9. authoritative caption source lineage + last-end verification;
10. exact audio-mix identity validation;
11. historical valid replacement => deterministic `STALE` without silent rebind;
12. missing/cross-project/spoofed identity => deterministic failure/`BROKEN`;
13. explicit reconciliation preserves unrelated edits and rejects ambiguous mapping;
14. reconciliation candidates pass authoritative validation before apply;
15. optimistic concurrency rejects stale writers;
16. immutable snapshot digest is deterministic and exact-revision scoped;
17. snapshot rejects unresolved stale/broken dependencies;
18. later mutable changes do not alter existing snapshots;
19. API runtime routes are actually attached;
20. representative PostgreSQL integration and required repository CI remain green.

TASK-050 and TASK-051 add their own acceptance gates without reopening these core merge gates.

## Downstream constraint for TASK-037

TASK-037 may select a concrete renderer only after revalidating runtime/license constraints. Renderer selection must consume this immutable contract rather than forcing renderer-specific state back into Scene Editor persistence.
