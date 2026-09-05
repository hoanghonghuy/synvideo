# TASK-035 — Background Music & Audio Mix V1

Status: READY
Priority: P1
Milestone: F1 Creative Workflow
Canonical branch when activated: `feature/TASK-035-background-music-mix`
Issue: #70
Depends on: MediaAsset audio support; TASK-031 accepted narration/audio lineage and measured-duration semantics
Contract: `docs/contracts/AUDIO_MIX_V1.md`

## Product outcome
Creator can select or upload durable project music, configure trim/loop/level/narration-aware ducking and timing, recover the mix across refreshes, see truthful stale/broken state when source assumptions change, and expose a stable composition-ready audio-mix snapshot for editor/render tasks.

## Scope
- Persist a versioned project audio-mix resource referencing durable audio `MediaAsset` IDs rather than ephemeral URLs.
- Bind narration-aware behavior to an exact accepted narration/audio lineage and measured narration duration.
- Support bounded music trim/start offset, loop policy, music gain and baseline narration-aware ducking parameters.
- Define deterministic validation for invalid asset kind, project isolation, timing bounds and unsupported configuration.
- Make stale/broken state explicit when narration lineage changes or referenced music becomes unavailable/deleted.
- Preserve creator edits across refresh/retry and protect concurrent writes with version/optimistic-concurrency semantics.
- Expose a render-engine-neutral immutable/versioned mix snapshot for TASK-036/TASK-037 composition/render consumers.
- Add creator selection/preview/configuration UX plus i18n/accessibility and regression coverage.

## Required behavior
1. Music references a durable audio `MediaAsset` in the same project/principal scope.
2. Narration-aware mix state records the exact narration/audio lineage it was authored against; narration replacement/regeneration cannot silently leave incompatible ducking/timing assumptions current.
3. Timing and gain parameters are bounded and deterministic; caller-supplied values cannot fabricate source duration or bypass asset metadata.
4. Loop behavior is explicit. V1 does not infer hidden looping/crossfade behavior outside the frozen contract.
5. Referenced music deletion/unavailability produces truthful `BROKEN`/equivalent state rather than silently substituting another asset.
6. Stale narration lineage remains inspectable/recoverable but is not silently rebound to a new narration source.
7. Concurrent/stale writes fail predictably rather than silently overwriting newer creator changes.
8. Browser refresh/retry recovers persisted mix state; preview controls must reflect persisted truth rather than in-memory-only state.
9. Composition/render consumers obtain an immutable/versioned audio-mix snapshot without mutating the live editor resource.
10. Domain state remains render-engine neutral; do not persist FFmpeg filter graphs, command fragments or provider-specific runtime details.

## Acceptance criteria
- `AUDIO_MIX_V1` contract is implemented without hidden narration-lineage rebinding or hidden music substitution.
- Mix create/read/update flows persist across refresh and enforce project isolation plus audio-asset type validation.
- Narration replacement/regeneration marks narration-dependent mix assumptions stale deterministically.
- Deleted/unavailable referenced music is surfaced truthfully and covered by regression tests.
- Trim/start/loop/gain/ducking validation rejects out-of-contract values deterministically.
- Optimistic/concurrency conflicts are tested.
- Creator UI exposes current/stale/broken/loading/error states truthfully and does not offer unsupported controls as active features.
- Composition/render consumers can read a stable versioned audio-mix snapshot.
- RED → GREEN → REFACTOR evidence follows `docs/engineering/TDD_PROTOCOL.md`.
- Required `Frontend`, `Backend`, `Local Infrastructure` CI remains green.

## Non-scope
- Final waveform editor or DAW-style multitrack editing.
- Final render-engine filter implementation; owned by downstream composition/render tasks.
- Automatic music generation/licensing marketplace.
- Advanced side-chain compressor curves, keyframe automation or multi-track music layering unless separately specified.

## Dependencies / relations
- TASK-031 is DONE and supplies accepted narration/audio lineage plus measured duration semantics.
- TASK-034 owns captions/timing and does not own audio mixing.
- TASK-036 consumes the immutable audio-mix snapshot in the scene/composition editor.
- TASK-037 owns final render/export integration.

## Activation gate
Activation revalidated on protected `develop` after TASK-046 completion: no canonical implementation branch or active implementation PR exists for TASK-035, planning contract remains on `develop`, TASK-031 is DONE, and bounded implementation capacity has one free slot. Developer may claim `feature/TASK-035-background-music-mix` from latest protected `develop`; runtime implementation still belongs to Developer and must follow TDD plus the frozen `AUDIO_MIX_V1` contract.

## TDD focus
Asset type/isolation, narration-lineage staleness, timing/gain/loop validation, persistence/reload, deletion/broken-state safety, optimistic concurrency, immutable snapshot behavior and frontend truthfulness.
