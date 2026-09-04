# TASK-037 — Render & Export V1

Status: BACKLOG / SPEC FROZEN
Milestone: F1 Creative Workflow
Canonical branch when activated: `feature/TASK-037-render-export-v1`
Depends on: accepted TASK-036 composition contract plus concrete snapshot-producing editor implementation and production render-safety/runtime gates
Authoritative contract: `docs/contracts/RENDER_EXPORT_V1.md`

## Product outcome
Creator can render one exact immutable composition snapshot into a durable MP4 artifact, observe truthful progress/failure/cancel, retry without losing or silently changing edits, recover state after refresh/restart, download the rendered artifact plus supported subtitle output, and inspect render history/provenance.

## Scope
- exact immutable composition snapshot + normalized output-config request identity;
- idempotent render acceptance and durable render lifecycle;
- truthful `queued|running|succeeded|failed|cancelled` creator states plus bounded safe progress;
- automatic same-logical-job retry/recovery distinct from creator-requested retry history;
- creator cancellation with durable intent/fenced finalization;
- renderer/toolchain adapter boundary with no raw engine/shell options in the public/domain contract;
- bounded process execution, concurrency, wall time, temp disk and log capture;
- MP4 baseline with versioned compatibility profile; exact codec/encoder build frozen at READY after legal/runtime validation;
- exact caption semantics in video plus optional durable WebVTT sidecar;
- durable `RenderArtifact` provenance referencing final MediaAsset(s), exact composition snapshot/config and exact toolchain profile version;
- bounded intermediate/temp cleanup aligned with `TEMP_OBJECT_LIFECYCLE_V1` when durable intermediates are used;
- creator progress/cancel/retry/history/download UI with refresh recovery and owner/project isolation;
- representative real render + PostgreSQL + S3-compatible integration coverage.

## Required behavior
- A render never consults newer mutable editor/media/audio/caption state after request acceptance.
- Repeating an ambiguous create with the same request identity returns/resumes the same logical render rather than duplicating expensive work.
- Automatic retry preserves exact snapshot/config and cannot publish multiple authoritative successful artifact sets.
- Creator retry of terminal work creates linked history instead of mutating the old terminal job.
- Partial/local output is never success; success exists only after final object storage + MediaAsset/RenderArtifact provenance commit.
- Cancellation is idempotent, durable and fenced so late/stale execution cannot commit success after ownership/cancellation is lost.
- Renderer commands/paths/options are server-controlled and derived from validated domain values; creator strings are never concatenated into shell commands.
- Resource exhaustion/process failure maps to stable safe categories instead of leaking raw renderer dumps or silently crashing the service where reasonably preventable.
- Final artifact history records exact snapshot/config/toolchain/output-hash provenance so later editor/toolchain changes do not rewrite history.
- Render reproducibility means reproducible intent/provenance and representative deterministic semantics; do not promise universal bitwise equality across arbitrary future encoders/hardware.

## Planning / dependency checkpoint — 2026-09-04
- `SCENE_EDITOR_COMPOSITION_V1` is accepted on protected `develop` via PR #110 / squash `1a7708b2cc59f41da575edae8f49927ffe277cb5`.
- TASK-036 implementation itself remains BACKLOG, so no concrete renderable composition snapshot producer exists yet.
- `JOB_EXECUTION_V1`, MediaAsset/S3 foundations and `TEMP_OBJECT_LIFECYCLE_V1` define reusable durable-job/storage boundaries.
- TASK-042 owns bounded generic executor behavior for non-cooperative cancellation/lease loss and is a production render safety dependency.
- TASK-047 owns durable temporary-object cleanup convergence when render uses recoverable object-storage intermediates.
- TASK-049 owns untrusted media/decoder resource-safety hardening relevant to render input processing.
- TASK-041 deployment topology/resources are required before freezing numeric render concurrency/time/disk budgets.
- Fresh planning dedupe found no canonical TASK-037 implementation branch and no TASK-037 implementation PR.

## Engine / license checkpoint — 2026-09-04
- **FFmpeg is the primary first-engine candidate, not yet an activated dependency.** Current official licensing says the base project is LGPL v2.1+, while optional GPL libraries/settings can change the resulting binary's obligations; `--enable-nonfree` can make a binary unredistributable. The exact deployed build/config, encoder choices and codec/patent/commercial implications must be revalidated and pinned before READY.
- **Remotion stays reference/later-adapter by default.** Current licensing is free for eligible individuals/small organizations but requires commercial licensing outside that eligibility and explicitly treats automated/batch video products as a commercial category. Selecting Remotion requires deliberate licensing plus Node/Chromium/deployment review.
- TASK-037 stays renderer-neutral at the domain boundary, so a later engine can be added without rewriting immutable render identity/provenance semantics.

Official references:
- https://ffmpeg.org/doxygen/trunk/md_LICENSE.html
- https://ffmpeg.org/legal.html
- https://github.com/remotion-dev/remotion/blob/main/LICENSE.md
- https://www.remotion.dev/

## TDD focus
Required coverage is frozen in `RENDER_EXPORT_V1`, including:
- exact snapshot/config idempotent create;
- no mutable-state reads after acceptance;
- corrupt/missing/cross-project input rejection;
- safe normalized presets/manual settings;
- durable lifecycle/progress/refresh recovery;
- auto retry vs creator retry history;
- queued/running cancellation and fenced race behavior;
- server-controlled renderer process execution plus resource bounds;
- MP4 + optional WebVTT durable artifact finalization;
- upload/metadata/temp-cleanup failure windows;
- exact provenance across later editor/toolchain changes;
- owner/project download isolation;
- representative real composition render plus PostgreSQL/S3 integration and required CI.

## Activation gate
Remain **BACKLOG / SPEC FROZEN** after planning merge. PM/TL moves issue #72 to READY **last** only after:
1. concrete TASK-036 implementation can create the accepted immutable composition snapshot;
2. duplicate branch/PR/issue checks are freshly clean;
3. the first renderer/toolchain/output profile is selected and pinned after current license/build/codec/deployment review;
4. required TASK-041/TASK-042/TASK-047/TASK-049 production safety dependencies for that chosen path are reconciled;
5. initial render concurrency/wall-time/temp-disk budgets are frozen against the deployment resources;
6. bounded implementation WIP has capacity.

Developer then owns `feature/TASK-037-render-export-v1`; PM/TL does not implement runtime render code.
