# TASK-034 — Captions & Scene Timing V1

Status: READY
Priority: P1
Milestone: F1 Creative Workflow
Canonical branch when activated: `feature/TASK-034-captions-timing`
Issue: #69
Depends on: TASK-031 accepted narration/audio lineage and measured duration semantics
Contract: `docs/contracts/CAPTIONS_TIMING_V1.md`

## Product outcome
Creator can derive timed captions from the exact current narration/audio lineage, edit text/timing/style safely, recover work across refreshes, detect stale captions after narration changes, intentionally rebuild, and expose composition-ready caption state for later editor/render tasks.

## Scope
- Persist a versioned caption document bound to an exact source audio/narration lineage identifier and measured duration.
- Maintain ordered caption segments with stable IDs, text and bounded start/end times.
- Provide an initial derive/generate/rebuild path from the current narration/audio source without silently overwriting creator edits.
- Support creator editing of text and timing with optimistic/concurrency-safe persistence semantics consistent with existing project resources.
- Persist a render-engine-neutral baseline style model rather than FFmpeg/Remotion-specific fields.
- Make current/stale/rebuilding/error state explicit in API and UI.
- Preserve a recoverable revision/history boundary sufficient to avoid accidental silent loss during replacement/rebuild.
- Expose a composition-ready caption representation for TASK-036 and subtitle-export-ready data for TASK-037.
- Add frontend creator workflow, i18n/accessibility and regression tests.

## Required behavior
1. A caption document identifies the exact source narration/audio lineage from which it was derived.
2. Replacing/regenerating narration cannot silently leave old captions marked current; mismatch becomes deterministic `STALE` state.
3. Stale captions remain inspectable/editable but are not silently rebound to new audio.
4. Rebuild is intentional and creates a new caption revision; manually edited text/timing is never overwritten without an explicit creator action.
5. Segment ordering is deterministic; each segment satisfies `0 <= start < end <= source_duration` and overlaps follow the frozen contract.
6. Caption document timing cannot claim a source duration different from its bound audio lineage.
7. Style is provider/render-engine neutral and unsupported controls are not exposed as working features.
8. Concurrent/stale writes fail predictably rather than last-write-wins silently.
9. API/UI expose current/stale/rebuilding/error truthfully and survive browser refresh.
10. Composition consumers can obtain an immutable/versioned caption snapshot without mutating the live editor resource.

## Acceptance criteria
- `CAPTIONS_TIMING_V1` contract is implemented without hidden source-lineage rebinding.
- Caption create/derive/read/update/rebuild flows persist across refresh and enforce project isolation.
- Narration replacement/regeneration makes previous caption lineage stale deterministically.
- Manual edits survive ordinary reads/retries and are only replaced through explicit rebuild/replace semantics.
- Timing validation rejects negative, reversed, out-of-duration and contract-invalid overlap cases.
- Optimistic/concurrency conflict behavior is covered by tests.
- Creator UI represents loading/current/stale/rebuilding/error states truthfully and does not offer unsupported style/timing controls.
- Composition/export consumers can read a stable versioned caption snapshot.
- RED → GREEN → REFACTOR evidence follows `docs/engineering/TDD_PROTOCOL.md`.
- Required `Frontend`, `Backend`, `Local Infrastructure` CI remains green.

## Non-scope
- Final render-engine subtitle burn-in implementation; owned by TASK-037.
- Full timeline/scene editor UX; owned by TASK-036.
- Speech recognition provider marketplace or automatic speaker diarization.
- Advanced karaoke/word-level animation unless separately specified.

## Dependencies / relations
- TASK-031 is DONE and supplies accepted narration/audio lineage plus measured duration semantics.
- TASK-035 consumes narration timing but owns music/mix composition, not captions.
- TASK-036 consumes the caption snapshot inside the scene editor/composition contract.
- TASK-037 consumes composition-ready caption data for subtitle rendering/export.

## Activation gate
Activation revalidated on protected `develop` after TASK-046 completion: no canonical `feature/TASK-034-captions-timing` branch or active TASK-034 implementation PR exists, TASK-031 remains DONE with accepted narration/audio lineage and measured-duration semantics, and bounded implementation capacity is available. Developer may claim the canonical branch from latest protected `develop`; runtime implementation belongs to Developer and must follow TDD plus the frozen `CAPTIONS_TIMING_V1` contract.

## TDD focus
Lineage staleness, duration/timing validation, overlap policy, manual-edit preservation, explicit rebuild, optimistic concurrency, project isolation, immutable snapshot behavior and frontend truthfulness.
