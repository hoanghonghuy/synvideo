# RENDER_EXPORT_V1

Status: FROZEN FOR PM/TL PLANNING
Applies to: TASK-037 Render & Export V1
Upstream authority: `SCENE_EDITOR_COMPOSITION_V1`
Downstream consumer: TASK-038 Channel Hub & Publishing V1

## Purpose
Define a durable, restart-safe and render-engine-neutral contract for turning one immutable Scene Editor composition snapshot into creator-visible export artifacts without consulting mutable project state during rendering.

TASK-037 owns render/export orchestration, durable render lifecycle, output provenance, bounded execution/cancellation/cleanup and creator-facing progress/retry/download behavior. It does not redefine Script, Scene Plan, media-generation, caption, audio-mix or editor semantics.

## Core invariant
A render job is bound to one exact immutable composition snapshot and one exact normalized output configuration. Once accepted, that logical job must never silently read newer scene/media/narration/caption/audio state.

Automatic retry/recovery of the same logical job may repeat local processing, but must not change the snapshot or output configuration. A creator who wants changed composition or export settings starts an explicitly new logical render.

## Render request identity
A render request records at least:
- `render_job_id`;
- authenticated owner/project identity;
- exact immutable `composition_snapshot_id`;
- composition snapshot schema version, revision and canonical digest;
- normalized output profile/configuration;
- creator request/idempotency identity;
- optional explicit `retry_of_render_job_id` for a creator-requested retry of a terminal prior job;
- creation metadata.

The client never supplies mutable asset URLs, storage keys, renderer commands, process paths or arbitrary engine flags as render identity.

### Idempotent acceptance
Repeating the same ambiguous create request with the same accepted request identity returns/resumes the same logical render job. It must not enqueue two logically equivalent expensive renders merely because the client did not receive the first response.

Changing the snapshot or normalized output configuration requires a new logical request identity.

## Snapshot validation
Before expensive execution begins, the worker establishes that the immutable snapshot is still internally resolvable under the same owner/project boundary.

Validation includes, at minimum:
- snapshot identity/digest matches the accepted render request;
- ordered composition scenes and timing are structurally valid;
- exact referenced MediaAssets exist and belong to the project;
- stored media metadata required for rendering is consistent with the referenced asset identity;
- exact narration/caption/audio-mix revisions referenced by the snapshot remain readable;
- unresolved stale/broken state prohibited by `SCENE_EDITOR_COMPOSITION_V1` was not smuggled into the snapshot;
- no cross-project resource can be substituted by request input.

A later change to the live editor or current binding does not invalidate an already-valid immutable snapshot merely because something newer exists. Missing/corrupt required historical inputs do fail the render truthfully.

## Input integrity
Render uses SynVideo-managed durable asset identity, not temporary upstream URLs.

Where durable metadata provides byte size and SHA-256, the render path must preserve an integrity-check boundary sufficient to detect wrong/corrupt object content before treating it as a successful render input. Exact probing strategy is implementation-specific but must remain bounded.

Creator-provided media remains untrusted input even after successful upload. Decoder/probe execution must therefore stay within the media-validation/resource-safety boundary defined by TASK-049; render must not assume a trusted MIME label makes arbitrary bytes safe.

## Normalized output configuration
The public/domain configuration is semantic and bounded. It does not expose raw FFmpeg/Remotion/encoder command strings.

V1 records at least:
- stable `profile_id` or `custom` selection;
- output canvas width/height compatible with the project aspect ratio;
- frame-rate value from an allowlisted bounded set;
- semantic quality tier or bounded quality value mapped by the selected engine adapter;
- audio enabled/disabled according to composition content;
- subtitle export mode;
- container family fixed to the accepted V1 MP4 baseline;
- schema version.

Common creator presets may include platform-oriented names, but platform branding must not change deterministic output semantics behind an existing profile ID. If preset semantics change, version the profile.

### Manual settings
Manual V1 settings remain constrained to creator-relevant values such as supported resolution/frame rate/quality/subtitle mode. No public field accepts codec names, filter graphs, shell fragments, executable paths, browser flags or arbitrary renderer options.

## MP4 baseline and codec gate
V1 product output requires a durable `video/mp4` baseline artifact.

The contract deliberately does **not** freeze a specific video/audio encoder implementation before the production build/license/deployment gate. The first implementation must freeze one compatibility profile at READY time with:
- exact video/audio codec choice;
- selected encoder implementation;
- toolchain/build version and configuration;
- browser/platform playback verification;
- license/distribution obligations;
- any relevant codec/patent/commercial review for the intended deployment/distribution jurisdictions.

Once a profile version is accepted, a render job records that version. A later codec/toolchain change creates a new profile/toolchain version rather than silently changing reproducibility semantics of historical jobs.

## Subtitle behavior
When the immutable composition snapshot contains enabled captions, the video rendering path must honor the exact accepted caption timing/style semantics required by the snapshot.

V1 export also supports an optional sidecar WebVTT artifact derived from the exact caption snapshot:
- `off`: no sidecar;
- `webvtt`: create a durable `text/vtt` sidecar;
- `webvtt_and_video`: create the sidecar and include the composition's visible caption treatment in the MP4 when that treatment is enabled by the snapshot.

Sidecar text/timing must come from the exact snapshot lineage; it must not query a newer caption revision during render. Unsupported styling that cannot be represented in WebVTT may be omitted from the sidecar while remaining represented in the video according to the accepted composition semantics; this limitation must be documented truthfully.

## Render lifecycle
Creator-visible lifecycle states are:
- `queued`;
- `running`;
- `succeeded`;
- `failed`;
- `cancelled`.

An implementation may use internal transitional states such as `cancel_requested`, `validating`, `acquiring_inputs`, `rendering`, `finalizing` or `cleanup_pending`, but public state must remain truthful and monotonic toward one terminal outcome.

`succeeded`, terminal `failed`, and `cancelled` are immutable terminal records. A later creator retry creates a new render job referencing the same snapshot/config by default rather than rewriting terminal history.

## Progress
The UI/API exposes truthful progress sufficient for a creator to understand long-running work.

Progress includes a stable safe phase and may include a bounded percentage when the engine can justify one. Rules:
- percentage never decreases for the same execution attempt;
- do not invent precision when the renderer cannot estimate it;
- process-local logs/frames are not exposed raw;
- a retry/reclaimed execution may restart phase-local work while the logical job remains the same;
- progress is advisory and never used as the durable success criterion.

## Automatic retry vs creator retry
Automatic retry is recovery of the same logical render job after a retryable infrastructure/process failure. It keeps the same snapshot/config and must not publish multiple final artifacts.

Creator retry after terminal `failed` or `cancelled` creates a new render job with `retry_of_render_job_id` and the same snapshot/config unless the creator explicitly changes settings, in which case it is a new render request rather than a retry.

Raw engine exit text does not decide retryability directly. The adapter maps failures into stable safe categories such as input-invalid, dependency-unavailable, renderer-transient, resource-limit, cancelled, unsupported-profile and internal-render-error.

## Cancellation
TASK-037 introduces creator-requested cancellation for render work even though the original generic durable-job V1 did not expose cancellation.

Required semantics:
- cancel is owner/project scoped and idempotent;
- cancelling queued work prevents it from starting;
- cancelling running work records durable cancellation intent before signaling execution context/process termination;
- cooperative subprocess/engine work receives bounded cancellation promptly;
- stale or late execution after cancellation/lease loss cannot commit success using obsolete ownership;
- if cancellation races with final success, one atomic/fenced terminal decision wins; the job may not be both succeeded and cancelled;
- partial/uncommitted output from a cancelled job never appears as a successful artifact;
- repeated cancel on a terminal job is deterministic and non-destructive.

TASK-042 owns the generic executor guarantee that non-cooperative handlers cannot hang cancellation/lease-loss control flow indefinitely. TASK-037 must integrate with that boundary rather than inventing unsafe goroutine/process termination semantics.

## Process and renderer safety
The selected render adapter must never concatenate untrusted creator strings into a shell command.

Rules:
- prefer direct process execution with an argument vector rather than `sh -c` or equivalent;
- executable identity/path is server-controlled;
- input/output temporary paths are server-generated;
- renderer options are derived only from validated normalized domain values;
- environment variables passed to the renderer are allowlisted and contain no unrelated application secrets;
- stdout/stderr capture is bounded;
- process trees are terminated/reaped under bounded cancellation semantics;
- renderer error logs are classified/redacted before persistence or creator exposure.

## Resource budgets and concurrency
Rendering is CPU, memory, disk and I/O intensive. V1 must not allow unbounded concurrent render processes per instance.

The contract requires configurable/explicit production bounds for:
- concurrent render workers/processes per instance;
- maximum wall-clock execution per attempt;
- temporary working-disk budget per job and/or worker;
- input/output size constraints compatible with accepted media limits;
- subprocess output/log capture;
- retry/backoff count and scheduling.

Exact numeric defaults are frozen at READY time after TASK-041 deployment topology and expected instance resources are known. Resource exhaustion maps to a stable safe failure instead of crashing the whole API/worker where reasonably preventable.

## Temporary and intermediate data
Process-local scratch files are not durable product resources and are created only under server-controlled paths.

When render recovery requires durable object-storage intermediates/checkpoints, they must reuse `TEMP_OBJECT_LIFECYCLE_V1` ownership, retention and cleanup-convergence semantics rather than inventing an ad-hoc prefix with no reaper.

Rules for all intermediates:
- they are attributable to owner/project/render job;
- successful finalization makes no longer needed intermediates cleanup-eligible;
- terminal failure/cancellation also converges cleanup;
- inline cleanup failure does not turn a safely committed final artifact into a render failure, but it remains observable/retryable by the cleanup subsystem;
- crash/retry must not leave indefinite creator-content debris.

## Finalization and durable artifacts
A render is successful only after final output bytes are safely stored in SynVideo-managed object storage and durable metadata/provenance is committed.

V1 creates a `RenderArtifact` domain record that references:
- render job ID;
- project/owner scope;
- exact composition snapshot ID/revision/digest;
- normalized output profile/config version;
- durable MP4 MediaAsset ID;
- optional subtitle MediaAsset ID;
- exact engine adapter/toolchain profile/version identifier;
- output byte size/hash/duration/resolution/frame-rate metadata where available;
- created timestamp.

The final MP4 and subtitle objects use existing durable MediaAsset/object-storage semantics where compatible. `RenderArtifact` supplies render-specific provenance/history rather than overloading generic MediaAsset metadata as the sole render record.

### Atomic success boundary
Partial output, a local file, or an object-storage upload without committed render/artifact metadata is not success.

The implementation must make the failure windows explicit:
1. engine finishes but final upload fails => job not succeeded;
2. upload succeeds but MediaAsset/artifact persistence fails => compensating cleanup or durable orphan reconciliation is required;
3. artifact commits successfully => later temp-cleanup failure does not revoke success;
4. only the fenced owner of the current execution may finalize the render.

One logical render job produces at most one authoritative successful artifact set.

## Reproducibility and provenance
SynVideo guarantees reproducible **render intent and provenance**, not universal bit-for-bit equality across arbitrary future renderer builds/hardware.

Historical artifact provenance records enough identity to answer:
- which immutable composition snapshot was rendered;
- which normalized output profile/config was used;
- which exact durable source assets/revisions were in that snapshot;
- which accepted renderer/toolchain profile version produced the artifact;
- what output hash/metadata was actually stored.

For the same snapshot/config and the same pinned software-renderer/toolchain profile, representative deterministic integration fixtures should produce stable semantic results and, where the chosen pipeline permits, stable bytes after removing nondeterministic metadata. Hardware/encoder paths that cannot promise byte identity must not claim it.

## Engine abstraction
Core render/domain code depends on a small adapter boundary rather than FFmpeg, Remotion or cloud-vendor types.

Conceptually the adapter receives:
- validated immutable render snapshot/input manifest;
- normalized accepted output configuration;
- server-controlled working/output target;
- context/cancellation signal;
- bounded progress callback.

It returns normalized output metadata or a stable classified error. Raw engine command lines, browser internals and vendor payloads do not enter public domain resources.

## Engine/license checkpoint — 2026-09-04
### FFmpeg
FFmpeg is the primary first-engine candidate because it can be orchestrated behind the Go backend adapter without requiring a second application runtime solely for rendering, and its CLI/process model fits explicit resource/cancellation boundaries.

Current official FFmpeg licensing states that the base project is LGPL v2.1+ but optional GPL components change the resulting FFmpeg license to GPL; external libraries such as `libx264`/`libx265` require GPL enablement, while `--enable-nonfree` can make a resulting binary unredistributable. FFmpeg also explicitly warns that codec/patent considerations depend on the chosen formats and jurisdiction.

Therefore TASK-037 does **not** authorize “whatever ffmpeg package happens to exist on the host.” READY must inspect/pin the actual build and its `-buildconf`/enabled libraries, license obligations and codec profile.

Official references:
- https://ffmpeg.org/doxygen/trunk/md_LICENSE.html
- https://ffmpeg.org/legal.html

### Remotion
Remotion remains a valid architectural/reference or later adapter option, especially for React/browser-driven composition workflows, but it is not frozen as SynVideo V1's default renderer.

Current Remotion licensing permits free use for individuals/small eligible organizations and requires a company license outside that eligibility; its current commercial offering specifically distinguishes automated/batch video products. Selecting it for production therefore requires a deliberate commercial/license decision plus Node/Chromium/runtime/deployment review rather than accidental dependency adoption.

Official references:
- https://github.com/remotion-dev/remotion/blob/main/LICENSE.md
- https://www.remotion.dev/

## Creator API/UI truthfulness
Creator-facing behavior must distinguish at least:
- no render yet;
- queued;
- running with safe phase/progress;
- cancellation pending where applicable;
- cancelled;
- failed with stable actionable category and retry affordance;
- succeeded with exact artifact metadata/download actions;
- historical renders for older snapshots/configurations.

Refresh/reopen must recover durable state. The UI must not show a local optimistic success before final durable artifact commit.

Downloads remain owner/project authorized and must not expose raw object-storage credentials/keys. Expiring download mechanisms, if later used, are delivery details rather than durable artifact identity.

## Security and isolation
- Every render create/get/list/cancel/retry/artifact operation is owner/project scoped and non-disclosing.
- A render snapshot from another project cannot be rendered through guessed identity.
- Renderer processes receive only the files/config required for the job.
- No BYOK provider credentials are required merely to render already-acquired assets.
- Logs/diagnostics must not include private media bytes, signed URLs, raw authorization material, arbitrary input paths or full renderer dumps by default.
- Final and temporary object keys are server-controlled.

## Required TDD / integration gates
1. create accepts one exact immutable composition snapshot/config and ambiguous retry returns the same logical render job;
2. changed snapshot/config requires a new logical render identity;
3. render never reads newer mutable scene/media/caption/audio state after acceptance;
4. missing/cross-project/corrupt required input fails safely before false success;
5. normalized presets/manual settings reject raw engine/shell options and unsafe values;
6. queued/running/succeeded/failed/cancelled lifecycle is durable across refresh;
7. progress phase/percent semantics are truthful and bounded;
8. automatic transient retry preserves the exact snapshot/config and cannot commit duplicate successful artifacts;
9. creator retry creates linked history without rewriting the terminal prior job;
10. queued cancellation prevents execution;
11. running cancellation is durable/idempotent and late/stale work cannot commit success;
12. cancellation/finalization race has one fenced terminal result;
13. subprocess execution uses server-controlled executable/arguments/paths and propagates bounded context cancellation;
14. wall-clock/concurrency/temp/log limits produce stable safe behavior;
15. successful MP4 bytes become a durable MediaAsset plus RenderArtifact provenance record;
16. optional WebVTT sidecar is derived from the exact caption snapshot;
17. engine-complete + upload failure cannot report success;
18. final upload + metadata failure compensates/reconciles orphan output;
19. final artifact success survives independent temporary-cleanup retry while cleanup remains observable;
20. terminal failure/cancellation intermediates converge through the accepted temporary-object lifecycle;
21. historical artifact retains exact snapshot/config/toolchain/output hash provenance after later editor/toolchain changes;
22. authorized download/history works after full browser/API refresh and cross-owner access is non-disclosing;
23. representative real render fixture validates video timing, selected visual, narration/audio, captions and transition semantics against the composition snapshot;
24. representative real PostgreSQL + S3-compatible storage integration proves durable lifecycle/finalization;
25. required `Frontend`, `Backend`, `Local Infrastructure` CI remains green; E2E critical-path coverage is added when TASK-046 harness is available.

## Activation rule
Merging this contract freezes TASK-037 product/render semantics but does **not** automatically authorize implementation.

Before issue #72 becomes READY, PM/TL must:
1. confirm TASK-036 contract is accepted and the concrete Scene Editor implementation can actually create the immutable snapshot defined there;
2. rerun duplicate branch/PR/issue checks;
3. select and pin the first renderer/toolchain profile after current license/build/codec/deployment review;
4. reconcile TASK-041 deployment resources plus TASK-042 cancellation safety, TASK-047 temporary-object lifecycle and TASK-049 decoder/media-safety dependencies required by the chosen implementation path;
5. freeze initial render concurrency/time/disk budgets and output compatibility profile;
6. reconcile bounded implementation WIP;
7. move the authoritative issue to READY last.

Developer implementation, once activated, belongs on `feature/TASK-037-render-export-v1`. PM/TL does not implement runtime rendering code in this planning branch.
