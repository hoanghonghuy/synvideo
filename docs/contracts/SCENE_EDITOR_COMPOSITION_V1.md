# SCENE_EDITOR_COMPOSITION_V1

Status: FROZEN FOR PM/TL PLANNING
Applies to: TASK-036 Scene Editor V1
Downstream consumer: TASK-037 Render & Export V1

## Purpose
Define the durable, render-engine-neutral Scene Editor composition contract that turns accepted creative-workflow resources into one versioned editable project composition without mutating approved upstream history.

The Scene Editor is the composition boundary. It does not become a second Script/Scene Plan persistence model, a provider orchestration layer, or a render-engine configuration surface.

## Product outcome
A creator can open a project composition, review and initiate narration/script edits through the authoritative upstream versioning workflow, reorder/duplicate/remove scene instances, choose exact accepted visual/narration/caption/audio inputs, edit bounded presentation/timing/transition settings, preview the same normalized semantics that will be snapshotted for rendering, save/recover work safely, detect stale or broken dependencies, reconcile accepted upstream changes explicitly, and create an immutable render-input snapshot.

## Upstream authority and non-mutation rule
TASK-036 consumes accepted upstream resources and never silently rewrites them.

Authoritative upstream identities include, where applicable:
- approved `ScenePlan` version and exact `scene_key`;
- current or explicitly selected visual `MediaAsset` plus scene-media binding lineage;
- current or explicitly selected narration audio `MediaAsset` plus narration binding/generation lineage;
- caption document ID + exact revision + source narration/audio lineage;
- audio-mix document ID + exact revision + referenced music asset + narration lineage.

An existing composition remains historical against the exact identities it recorded. A newer Scene Plan, visual assignment, narration regeneration, caption rebuild, music replacement, or audio-mix edit never mutates a saved composition in place.

Editing Script text, Scene Plan narration/visual instruction, provider prompts, TTS text, caption source derivation, or MediaAsset provenance belongs to the owning upstream feature. The Scene Editor may initiate or navigate those workflows, but must not perform a hidden upstream rewrite or persist an alternative composition-only source-of-truth for them.

## Integrated narration/script edit bridge
Stage-9 Scene Editor must still provide a coherent narration/script editing experience. In V1 this is an **integrated upstream-versioned bridge**, not a local text override inside the composition.

When the creator initiates a narration/script edit from Scene Editor:
1. the edit is performed through the authoritative Script/Scene Plan versioning path owned by the upstream workflow;
2. approved Script/Scene Plan history is preserved rather than mutated in place;
3. the resulting accepted upstream version/scene identity is explicit before it becomes production-current for the composition;
4. narration audio, captions, generated-media assumptions, or other dependencies affected by the source change become stale/rebuild-required according to their owning contracts instead of being silently rebound;
5. the existing composition remains recoverable against its recorded source identities until the creator explicitly reconciles to the accepted newer upstream version;
6. reconciliation creates a new saved composition revision and preserves unrelated presentation edits where mapping is unambiguous;
7. no local-only narration/script text may appear in preview or render snapshot unless it exists in the exact accepted upstream source lineage recorded by that composition revision.

The UI may keep transient unsaved input while the upstream edit workflow is open, but it must label it truthfully and must not present that text as persisted composition/render state before upstream acceptance and reconciliation succeed.

## Composition document
A project may have versioned composition documents/snapshots. The live editable composition contains at least:
- stable `composition_id`;
- `project_id`;
- positive optimistic-concurrency `revision`;
- exact source `scene_plan_version`;
- lifecycle state sufficient to distinguish `CURRENT`, `STALE`, `BROKEN`, and error/conflict conditions;
- ordered composition scene instances;
- optional exact project audio-mix reference;
- project presentation defaults that are render-engine neutral;
- created/updated metadata.

There is at most one current editable composition document for the same project/source lineage unless a later contract explicitly introduces named variants. Historical immutable render snapshots are separate from the live editor document.

All timing at domain/API boundaries uses integer milliseconds in V1. Floating-point seconds are not persisted as canonical timing identity.

## Composition scene instance
Each scene instance has a stable composition-local ID independent from the upstream `scene_key`. It records at least:
- `composition_scene_id`;
- exact source `scene_key` from the recorded approved Scene Plan;
- deterministic order position;
- exact selected visual dependency or explicit unbound state;
- exact selected narration dependency or explicit silent/unbound state;
- exact selected caption document/revision or explicit captions-off state;
- output duration in milliseconds;
- bounded visual presentation settings;
- bounded caption presentation override only where the caption contract permits semantic override;
- transition-out settings;
- optional creator-facing composition notes that are not render content unless explicitly modeled as such.

A scene instance may not point to a `scene_key` outside the composition's recorded Scene Plan version.

## Reorder, duplicate and remove semantics
V1 supports composition-local reorder, duplicate and remove without changing approved Scene Plan history.

### Reorder
Reorder changes output sequence only. Source Scene Plan order remains unchanged and inspectable.

### Duplicate
Duplicate creates a new `composition_scene_id` and copies the source/dependency/presentation values from the chosen instance at the current saved revision. Later edits to either instance are independent. Duplication never creates a new upstream scene, MediaAsset, narration job, caption document or provider operation.

### Remove
Remove deletes the scene instance from the live composition only. It never deletes the upstream Scene Plan scene or referenced assets/history. A valid V1 composition contains at least one scene instance.

### Split / merge
Interactive scene split and merge are **not persisted operations in V1**. Offering them as active editor controls would create ambiguous narration/caption/media lineage and risk hidden Script/Scene Plan rewrites. The UI must not present them as supported actions.

A creator who needs semantic scene split/merge returns to the Scene Plan workflow, creates/approves a new Scene Plan version, then explicitly reconciles or creates a new composition against that version. A later contract may add composition-local clip splitting when source-range semantics are fully specified.

## Visual dependency and treatment
A selected primary visual is a durable same-project visual `MediaAsset` (`image` or `video`) or an explicit unbound state. Temporary URLs and raw provider/storage identities are never composition identities.

V1 visual treatment is semantic and bounded:
- fit mode: `contain | cover`;
- normalized crop rectangle when applicable;
- normalized position/translation within documented bounds;
- bounded scale/zoom;
- optional mute flag for source-video audio if video assets can contain audio;
- no arbitrary CSS, FFmpeg filters, shader source or renderer command fragments.

Crop/position/scale validation must be deterministic. Preview and render normalization must use the same accepted values.

Changing the project's current scene-media binding after a composition save makes the composition dependency stale when it no longer matches the recorded selected binding/asset. It does not silently replace the composition's asset.

## Narration and scene duration
Narration references the exact accepted narration asset/binding lineage. The Scene Editor does not rewrite narration text or regenerate TTS implicitly; narration/source edits follow the integrated upstream-versioned bridge defined above, and narration regeneration/rebinding remains an explicit owning-workflow action.

V1 scene duration is explicit positive integer milliseconds. Without an explicit trim/time-stretch contract, Scene Editor V1 must not silently truncate or stretch narration/captions to force a shorter scene.

When narration is enabled, persisted duration must be at least the authoritative measured narration duration. When captions are enabled, duration must also cover the last caption segment end. A creator may extend scene duration beyond those sources; any resulting trailing visual/audio behavior must be deterministic and represented by composition settings rather than renderer magic.

If exact upstream duration is unavailable when required for validation, save/snapshot must fail truthfully rather than guess.

## Captions
Caption selection records exact caption document ID, revision, and narration/audio source lineage. Only a production-current caption revision may be selected by default for a new current composition.

A newer caption revision or narration lineage makes the recorded caption dependency stale; the composition does not silently switch revisions.

V1 editor-level caption treatment may only persist semantic presentation fields already supported by the caption/render-neutral contract. It must not persist renderer-specific markup, filters or command fragments.

## Project audio mix
A composition may reference one exact production-current `AUDIO_MIX_V1` document/revision. The reference includes exact narration lineage and music asset identity needed to detect staleness/breakage.

A newer mix revision, changed narration lineage, or missing music asset does not silently change the composition. It produces a truthful stale/broken dependency state until the creator explicitly reconciles it.

V1 does not introduce arbitrary multitrack audio editing beyond the accepted audio-mix contract.

## Transitions
V1 transition semantics are deliberately small and render-engine neutral:
- `cut`;
- `fade`;
- `crossfade`.

Transition duration is integer milliseconds with documented bounds and must fit both adjacent scene durations. `cut` has zero duration. Unsupported engine-specific transition names/easing/filter graphs are not persisted.

If the first chosen render engine cannot faithfully implement a frozen V1 transition, TASK-037 must fail validation or narrow activation before production; it must not silently substitute a different effect.

## Stale and broken dependency semantics
A composition is `CURRENT` only when required recorded dependencies are still production-valid for its exact source lineage.

Examples of `STALE` include:
- a newer approved Scene Plan becomes the creator's active planning source;
- a selected scene visual/narration binding has been replaced;
- caption or audio-mix source lineage/revision has changed;
- a dependency remains readable but is no longer current for production.

Examples of `BROKEN` include:
- a required referenced MediaAsset is missing/unavailable;
- a referenced caption/mix revision cannot be resolved;
- source identity is structurally invalid or no longer readable under the project boundary.

Rules:
1. stale/broken state never deletes creator edits;
2. editing unrelated presentation fields does not magically make stale dependencies current;
3. reconciliation is explicit and shows what dependency will change;
4. accepting a replacement dependency creates a new saved revision;
5. default render-snapshot creation rejects unresolved stale/broken required dependencies;
6. cross-project/owner mismatches remain non-disclosing failures.

## Explicit reconciliation
The editor exposes explicit reconcile operations rather than auto-rebinding.

A reconcile preview identifies, per affected dependency:
- recorded identity;
- current candidate identity;
- stale/broken reason category;
- whether creator presentation edits can be preserved safely.

Applying reconciliation is revision-checked. If source Scene Plan structure changed materially, automatic mapping is permitted only for exact stable `scene_key` matches whose semantics remain valid; ambiguous removed/renamed/split/merged scenes require creator choice or a new composition. No heuristic remap may silently alter output.

An upstream narration/script edit uses this same reconciliation boundary after the new Script/Scene Plan version is accepted. The editor must show that the old composition remains historical/current-to-its-own-recorded-lineage until the creator deliberately accepts the candidate mapping.

## Save, dirty state and concurrency
The browser keeps unsaved editor changes as transient dirty state. Persisted writes require the expected composition revision.

Rules:
- one successful save increments revision exactly once;
- stale writers receive deterministic conflict and never overwrite newer work;
- refresh loads the last persisted revision;
- an ambiguous save response is recovered by re-reading server state/revision before issuing a new logical mutation;
- retries are idempotent where a request identity is required;
- leaving/reloading with dirty edits follows explicit creator-safe UX; the server must not pretend unsaved client state was persisted.

## Immutable render-input snapshot
TASK-037 consumes an immutable composition snapshot created explicitly from one successfully saved composition revision.

The snapshot contains all data required to reproduce composition intent without consulting mutable "current" selections at render time, including:
- composition ID + revision;
- project aspect-ratio/content-locale identity required by composition;
- source Scene Plan version;
- ordered stable composition scene instances;
- exact MediaAsset IDs and relevant immutable media metadata identities;
- exact narration binding/asset lineage;
- exact caption document/revision/source lineage;
- exact audio-mix document/revision/music/narration lineage;
- scene durations, visual treatment and transitions;
- schema version and deterministic canonical digest/hash.

Snapshot creation performs full dependency validation. Once created, the snapshot is immutable. Later editor saves create different snapshots; they never rewrite a snapshot already referenced by a render job/artifact.

Render configuration that is truly output-engine specific (codec, bitrate, encoder flags, deployment/runtime knobs) belongs to TASK-037 and is not smuggled into the composition document.

## Preview equivalence
Creator preview may use browser-specific rendering, but it must consume the same normalized composition semantics used to build the immutable snapshot.

Preview must not:
- read a newer current dependency than the saved editor revision without marking the preview dirty/stale;
- render transient narration/script bridge text as production state before its upstream version is accepted and reconciled;
- silently ignore unsupported persisted controls;
- mutate composition state merely by playing;
- claim frame/codec equivalence when the browser preview is only semantic/visual approximation.

The contract requires semantic equivalence: order, selected assets, accepted source/narration lineage, duration, crop/fit/position, captions on/off/style semantics, audio-mix selection and transitions must correspond to the snapshot consumed by TASK-037.

## Isolation, privacy and deletion
Every composition read/write/reconcile/snapshot operation is scoped through authenticated project/principal ownership.

Referenced durable resources must belong to the same project. Raw provider responses, secrets, signed URLs, storage keys and private prompt/media contents are not copied into composition diagnostics merely for observability.

Deletion of a referenced resource must not leave an undetectable dangling composition. Prefer restrictive deletion where accepted upstream contracts already require it; otherwise surface deterministic `BROKEN` state while preserving historical identity/provenance.

## API / creator UI truthfulness
The API/UI must distinguish at least:
- loading;
- saved/current;
- dirty/unsaved;
- saving;
- save conflict;
- stale dependency;
- broken dependency;
- upstream narration/script edit pending acceptance/reconciliation;
- preview unavailable/error;
- snapshot-ready vs snapshot-blocked.

Unsupported V1 controls (including split/merge and arbitrary timeline effects) must not be displayed as functioning actions.

Narration/script editing initiated from the Scene Editor must make the upstream versioning/reconciliation transition understandable to the creator; it may feel integrated in one workspace, but the UI must never imply a local edit has already changed persisted/renderable composition state when it has not.

Keyboard navigation, focus state, labels, error announcements and responsive layout are part of acceptance for the editor workspace; drag/drop must have an accessible non-pointer alternative for reorder.

## Required TDD / integration gates
1. create/load a composition from an exact approved Scene Plan without mutating it;
2. narration/script edit initiated from Scene Editor persists through the authoritative upstream version workflow, preserves approved history, and cannot produce hidden local render text;
3. accepted upstream narration/script change deterministically stales affected composition/dependencies and explicit reconciliation preserves unrelated creator edits;
4. stable composition scene identity across ordinary edits/reorder;
5. reorder round-trip and deterministic ordering;
6. duplicate creates independent local identity without duplicating upstream/provider work;
7. remove is composition-local and preserves upstream history; last-scene removal rejected;
8. split/merge rejected/not exposed in V1;
9. visual asset/project/kind validation and bounded crop/fit/position round-trip;
10. scene duration rejects truncation of authoritative narration/caption timing;
11. exact narration/caption/audio-mix lineage persisted;
12. visual/narration/caption/mix replacement => deterministic stale state without silent rebind;
13. missing required asset/revision => truthful broken state;
14. explicit reconciliation preserves unrelated creator edits and rejects ambiguous scene mapping;
15. optimistic concurrency permits one writer and rejects stale competitors;
16. ambiguous save recovery does not duplicate logical mutation;
17. refresh recovers persisted state; dirty client state is not misreported as saved;
18. immutable snapshot records exact revision/dependencies and deterministic canonical digest;
19. later upstream/editor changes do not mutate an existing snapshot;
20. preview normalization and snapshot semantics agree for representative image/video, accepted narration source, captions, narration audio and transition cases;
21. cross-owner/project access fails safely;
22. accessibility keyboard reorder/focus/error-state coverage;
23. representative real PostgreSQL/object-storage integration plus required repository CI remains green.

## Reference and reuse checkpoint — 2026-09-04
References are architectural/product-study inputs, not authority over this contract.

- `timoncool/videosos`: repository declares MIT; may be studied for browser editor/product patterns, but SynVideo should prefer its own domain model rather than importing product-specific architecture wholesale.
- `gyoridavid/short-video-maker`: repository declares MIT; useful for deterministic scene assembly/render pipeline study.
- `openvideodev/react-video-editor` / OpenVideo: current project states a dual-license model with a company license required above the free-eligibility threshold. Treat as **study-only by default** unless a later legal/commercial decision explicitly authorizes code reuse.
- `remotion-dev/remotion`: current repository uses the Remotion License with free-use eligibility limits and a company-license path. TASK-037 must revalidate deployment/company eligibility before selecting it as a production render dependency; do not assume generic MIT rights for the full project.
- Cutaway / RemotionUI: current public page presents early-access product behavior and source ownership claims for generated output, but no sufficiently clear public source-code license was established in this checkpoint. Treat as **product/UX study-only**.

License/runtime selection for the actual render engine remains TASK-037. TASK-036 must stay render-engine neutral.

Reference URLs:
- https://github.com/timoncool/videosos
- https://github.com/gyoridavid/short-video-maker
- https://github.com/openvideodev/react-video-editor
- https://github.com/remotion-dev/remotion/blob/main/LICENSE.md
- https://remotionui.com/cutaway

## Activation rule
Merging this contract freezes TASK-036 product/composition semantics but does **not** automatically authorize implementation.

Before issue #71 becomes READY, PM/TL must:
1. confirm this contract is accepted on protected `develop`;
2. rerun duplicate branch/PR/issue checks;
3. verify the concrete prerequisite implementations required by the chosen V1 editor path are available and compatible;
4. reconcile bounded implementation WIP;
5. move the authoritative issue to READY last.

Developer implementation, once activated, belongs on `feature/TASK-036-scene-editor-v1`. PM/TL must not implement runtime code in this planning branch.
