# AUDIO_MIX_V1

## Purpose
Define the provider- and render-engine-neutral contract for a persistent project audio mix combining narration with one durable background-music asset.

## Core model
An audio-mix document is a versioned project resource containing:
- stable mix document ID;
- project ID;
- referenced background-music `MediaAsset` ID;
- exact narration/audio lineage ID used for narration-aware behavior;
- authoritative measured narration duration from that lineage;
- revision/version token;
- lifecycle state: `CURRENT`, `STALE`, `BROKEN`, or `ERROR` where applicable;
- music timing/loop configuration;
- narration/music gain and ducking configuration;
- created/updated metadata sufficient for recovery/history without exposing secrets.

V1 models one narration source plus one background-music asset. Extra music tracks or arbitrary multitrack graphs require a later contract.

## Asset and isolation invariants
- The referenced music must be a durable audio `MediaAsset` accessible inside the same authenticated project/principal boundary.
- Ephemeral provider URLs, browser object URLs and raw storage keys are not domain identities.
- Wrong media type or cross-project references fail deterministically.
- Asset metadata remains authoritative for properties such as measured media duration when available; callers cannot forge duration to bypass validation.

## Narration lineage and staleness
Narration-aware behavior records the exact accepted narration/audio lineage it was authored against.

When the project's selected narration lineage changes:
- the previous mix document is preserved;
- narration-dependent timing/ducking assumptions become `STALE` deterministically;
- the mix is never silently rebound to the new narration source;
- stale configuration remains inspectable so creator work is recoverable;
- downstream composition/render consumers must not mistake stale mix state for current production state.

Editing stale mix parameters does not by itself make the mix current. Rebinding/rebuilding against the new narration lineage requires explicit creator intent.

## Music timing and loop semantics
V1 persists explicit, bounded values for:
- music source trim start;
- project/composition start offset;
- loop policy;
- music gain/level.

Timing uses one documented canonical unit consistently at API/domain boundaries.

Validation must reject negative/out-of-range trims, impossible offsets and configurations that require media outside the referenced asset unless an explicit loop policy permits repetition.

Looping is explicit. `NO_LOOP` means playback ends when the selected source range ends. `LOOP_TO_TARGET` may repeat the selected music range only according to deterministic downstream composition rules. V1 does not imply hidden crossfade, beat matching or time stretching.

## Narration-aware ducking
V1 supports a render-neutral baseline ducking model. Persist semantic parameters such as:
- enabled/disabled;
- target music reduction while narration is active;
- bounded attack/release or equivalent transition-duration semantics only when supported consistently by downstream composition.

Do not persist FFmpeg filter graphs, command strings or engine-specific compressor/filter primitives in the domain model.

Ducking semantics are relative to the exact bound narration lineage. A narration lineage change therefore makes narration-dependent ducking assumptions stale until explicitly rebound/rebuilt.

## Gain bounds
Narration/music levels use one documented semantic unit and bounded range. API and UI must expose the same accepted bounds. Invalid or unsupported values fail before any render/provider work.

V1 must avoid hidden normalization, clipping correction or loudness mastering that is not represented truthfully by the contract.

## Broken asset semantics
If the referenced music asset is deleted, becomes unavailable, or is no longer a valid audio asset:
- the mix becomes `BROKEN` or equivalent truthful non-current state;
- the reference/history is preserved for diagnosis/recovery;
- the system must not silently substitute another asset;
- default composition/render flows must not treat the broken mix as production-current.

## Editing and concurrency
Mix edits operate on an expected revision/version. A stale writer receives a deterministic conflict rather than silently replacing newer creator changes.

Retries of the same logical write are idempotent where applicable. Reads and browser refresh recover persisted state rather than rely on in-memory progress.

## Preview boundary
Creator preview may use implementation-specific playback mechanisms, but preview state must derive from persisted mix configuration and referenced assets. UI must distinguish loading/current/stale/broken/error states and must not claim unsupported controls work.

Preview is not itself the authoritative render output and must not mutate the mix document implicitly.

## Snapshot boundary
TASK-036/TASK-037 consumers receive a versioned immutable audio-mix snapshot identified by mix document/revision, referenced music asset and exact narration lineage.

Snapshot reads do not mutate the live mix document. New compositions select only production-current mix state by default; explicit historical/stale inspection is separate from making stale state current.

## Privacy and observability
Diagnostics may include stable IDs, state transitions and validation categories but should not dump user media contents or sensitive provider/storage details. Project/principal isolation applies to every read/write/preview/rebind path.

## Required regression coverage
- valid same-project audio asset binding;
- wrong media type and cross-project rejection;
- persisted trim/start/loop/gain round-trip;
- out-of-contract timing/gain rejection;
- narration lineage change => deterministic stale mix;
- stale edit does not silently rebind narration;
- referenced music deletion/unavailability => truthful broken state;
- optimistic concurrency conflict;
- immutable snapshot behavior;
- browser refresh/persisted-state recovery;
- truthful creator UI lifecycle states.
