# TASK-036 — Scene Editor V1

Status: BACKLOG
Milestone: F1 Creative Workflow
Canonical branch when activated: `feature/TASK-036-scene-editor-v1`
Depends on: stable accepted visual/audio/caption/music foundations

## Product outcome
Deliver first baseline comprehensive scene editor: edit scene order/content/timing, visual treatment, narration/audio, captions, music and transitions; preview; persist immutable composition snapshot suitable for deterministic rendering.

## Scope
Versioned composition domain; reorder/duplicate/delete and frozen split/merge behavior; narration/reference, visual crop/fit/basic transform, duration, captions/style, narration audio, music relationship, transition; project defaults; deterministic snapshot/provenance; dirty/save/stale semantics; scene list/inspector/preview UI; future path to richer timeline; integration/accessibility coverage.

## Required behavior
Do not silently mutate upstream approved history; snapshot exact dependency lineage; underlying asset replacement does not silently rewrite snapshot; stale dependencies are surfaced/reconcilable; preview and render input semantics align; manual edits survive refresh/concurrency according to contract.

## Activation gate
BACKLOG. Freeze composition/editor contract after prerequisites; re-check reference licenses; move issue #71 READY last.

## Reference gate
Re-check cutaway, VideoSOS, OpenVideo/react-video-editor, Remotion and short-video-maker. OpenVideo remains study-only unless legal review allows reuse.

## TDD focus
Composition normalization/versioning, stale dependency detection, reorder/delete/duplicate invariants, dependency lineage, save recovery and preview equivalence.
