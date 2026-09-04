# TASK-036 — Scene Editor V1

Status: BACKLOG / SPEC FROZEN
Milestone: F1 Creative Workflow
Canonical branch when activated: `feature/TASK-036-scene-editor-v1`
Depends on: stable accepted visual/audio/caption/music foundations and accepted TASK-032 scene-video workflow
Authoritative contract: `docs/contracts/SCENE_EDITOR_COMPOSITION_V1.md`

## Product outcome
Deliver the first baseline comprehensive scene editor: compose exact accepted scene/media/audio/caption inputs into a versioned project edit; reorder/duplicate/remove scene instances; edit bounded timing, visual treatment and transition semantics; preview truthfully; save/recover creator work; surface stale/broken dependencies; and produce an immutable composition snapshot suitable for deterministic TASK-037 rendering.

## Scope
- versioned composition document with optimistic concurrency and exact approved Scene Plan lineage;
- stable composition-local scene identity;
- reorder/duplicate/remove as composition-local operations;
- **split/merge explicitly unsupported in V1** and redirected to the upstream Scene Plan workflow;
- exact durable visual MediaAsset/binding references plus bounded `contain|cover`, crop, position/scale and video-audio mute semantics;
- exact narration asset/binding lineage; no hidden narration rewrite or TTS regeneration;
- explicit duration in integer milliseconds without silent narration/caption truncation or time stretch;
- exact caption document/revision/source-lineage selection plus render-neutral style semantics;
- exact project audio-mix document/revision/music/narration lineage;
- render-neutral `cut|fade|crossfade` transition baseline;
- deterministic `CURRENT|STALE|BROKEN` dependency truthfulness and explicit reconciliation;
- dirty/saving/saved/conflict/refresh recovery semantics;
- immutable render-input snapshot with schema version, exact dependency identities and deterministic canonical digest;
- scene list/inspector/preview workspace with keyboard-accessible reorder, focus/error semantics and responsive coverage.

## Required behavior
- Never silently mutate approved Script/Scene Plan, MediaAsset provenance, narration generation, captions or audio-mix history.
- A newer upstream assignment/revision/lineage never silently rewrites a saved composition.
- Stale/broken dependencies preserve creator edits and require explicit reconciliation before default render snapshot creation.
- Reconciliation may auto-map only exact stable `scene_key` matches; ambiguous removed/renamed/split/merged scenes require creator choice/new composition.
- Preview consumes the same normalized composition semantics used for render snapshot creation and must not silently ignore persisted controls.
- Saved writes are revision-checked; stale writers cannot overwrite newer creator changes.
- Ambiguous save recovery re-reads durable state before issuing another logical mutation.
- Render snapshot creation is explicit and immutable; render jobs must never consult newer mutable “current” selections behind the snapshot.
- Cross-owner/project dependencies fail safely and diagnostics do not expose secrets/raw provider payloads/private media content.

## Dependency / planning checkpoint — 2026-09-04
- TASK-031 narration/audio lineage foundation: accepted.
- TASK-033 stock-media acquisition contract: frozen on protected `develop`.
- TASK-034 captions/timing contract: frozen on protected `develop`.
- TASK-035 audio-mix contract: frozen on protected `develop`.
- TASK-032 per-scene AI video workflow: DONE via PR #89 / squash `7b3569bd09d8a35ad7a57363eaccfbf4eb6d545c`.
- Former TASK-032 planning blocker is cleared.
- Fresh dedupe before this contract freeze found no canonical TASK-036 implementation branch or implementation PR.

## Reference / license checkpoint — 2026-09-04
References are for architectural/product study; SynVideo owns its domain/implementation.

- VideoSOS (`timoncool/videosos`): repository declares MIT.
- short-video-maker (`gyoridavid/short-video-maker`): repository declares MIT.
- OpenVideo/react-video-editor: current project uses a dual-license model; treat code as study-only by default unless later legal/commercial approval explicitly authorizes reuse.
- Remotion: current project uses the Remotion License with free-use eligibility limits/company-license path; TASK-037 must revalidate production eligibility before engine selection.
- Cutaway/RemotionUI: early-access product reference; no sufficiently clear public source-code license established in this planning checkpoint, therefore product/UX study-only.

Actual render-engine/runtime licensing remains a TASK-037 decision; TASK-036 remains render-engine neutral.

## TDD focus
Required coverage is frozen in `SCENE_EDITOR_COMPOSITION_V1`, including:
- composition create/load and upstream non-mutation;
- stable local scene identity, reorder/duplicate/remove invariants and last-scene protection;
- split/merge non-support;
- visual kind/project/transform validation;
- narration/caption duration constraints;
- exact dependency lineage and deterministic stale/broken detection;
- explicit reconciliation preserving unrelated edits;
- optimistic concurrency and ambiguous-save recovery;
- refresh/persisted-state recovery;
- immutable deterministic snapshot and later-change isolation;
- preview/snapshot semantic equivalence;
- owner/project isolation;
- keyboard/accessibility behavior;
- representative PostgreSQL/object-storage integration and full required CI.

## Activation gate
Remain BACKLOG after this planning contract is merged. PM/TL moves issue #71 to READY **last** only after:
1. the authoritative contract is accepted on protected `develop`;
2. duplicate branch/PR/issue checks are freshly clean;
3. concrete prerequisite implementations required by the selected V1 path are available/compatible;
4. bounded implementation WIP has capacity.

Developer then owns `feature/TASK-036-scene-editor-v1`; PM/TL does not implement runtime code.
