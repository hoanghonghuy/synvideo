# CAPTIONS_TIMING_V1

## Purpose
Define the provider- and render-engine-neutral contract for editable timed captions derived from an exact narration/audio lineage.

## Core model
A caption document is a versioned project resource with:
- stable caption document ID;
- project ID;
- source audio/narration lineage ID;
- source measured duration;
- revision/version token;
- lifecycle state: `CURRENT`, `STALE`, `REBUILDING`, or `ERROR` where applicable;
- ordered caption segments;
- baseline style object;
- created/updated metadata sufficient for recovery/history without exposing provider secrets.

Each caption segment has a stable segment ID, text, start time and end time. Segment identity must survive ordinary text/timing edits so creator changes are not confused with regeneration.

## Timing invariants
- Time values use one documented canonical unit consistently at API/domain boundaries.
- For every segment: `0 <= start < end <= source_duration`.
- Segment ordering is deterministic by start time then stable segment identity.
- V1 does not permit overlapping segments in persisted current caption state. Rejected overlap must have deterministic validation semantics.
- Caption source duration is copied from the exact accepted narration/audio lineage and cannot be caller-forged to make invalid timings appear valid.

## Lineage and staleness
A caption document is `CURRENT` only when its recorded source lineage exactly matches the project's currently selected narration/audio lineage.

When narration/audio is replaced, regenerated, rebound or otherwise changes lineage:
- existing caption documents are preserved;
- they become `STALE` deterministically;
- they are never silently rebound to the new source;
- stale content remains inspectable so creator work is recoverable;
- downstream composition/render consumers must not mistake stale captions for current captions.

## Derive / rebuild semantics
Initial derive creates a caption revision from the current exact source lineage.

Rebuild is explicit creator intent. It creates a new revision/source binding and must not silently overwrite manually edited caption content in-place. Old revision/history remains recoverable according to repository retention conventions.

Retries of the same logical derive/rebuild request must be idempotent where the operation may be asynchronous or provider-backed. If an upstream operation has paid/ambiguous-submit semantics, the existing durable-job exactly-once rules apply.

## Editing and concurrency
Text/timing/style edits operate on an expected caption revision/version. A stale writer must receive a deterministic conflict rather than silently replacing newer creator changes.

Manual edits do not alter source lineage identity. Editing a stale caption does not make it current.

## Style boundary
V1 style is deliberately render-engine neutral. Baseline fields may describe semantic presentation such as alignment, position preset, font-family token, relative size/weight, foreground/background treatment and emphasis only when the downstream composition/render layer can truthfully support them.

Do not persist engine-specific filter strings, command fragments or provider-specific implementation details in the domain contract.

## Snapshot boundary
TASK-036/TASK-037 consumers receive a versioned immutable caption snapshot identified by caption document/revision and exact source lineage. Snapshot reads must not mutate the live caption document.

Only `CURRENT` captions may be selected by default for a new composition snapshot. Explicit historical/stale inspection is separate from making stale captions production-current.

## API/UI truthfulness
API and creator UI must distinguish loading/current/stale/rebuilding/error states. Unsupported controls must not be offered as active features. Browser refresh must recover persisted caption state rather than depend on in-memory progress only.

## Isolation and privacy
Every caption read/write/rebuild is scoped through the authenticated project/principal boundary. Cross-project access must fail deterministically. Logs/observability must not dump full transcript text merely for lifecycle diagnostics.

## Required regression coverage
- derive against exact current narration lineage;
- narration lineage change => deterministic stale captions;
- stale caption edit does not make it current;
- explicit rebuild creates new current revision without silent manual-edit loss;
- negative/reversed/out-of-duration/overlapping segment rejection;
- optimistic concurrency conflict;
- project isolation;
- immutable snapshot behavior;
- truthful creator UI lifecycle states.
