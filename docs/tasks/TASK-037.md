# TASK-037 — Render & Export V1

Status: BACKLOG
Milestone: F1 Creative Workflow
Canonical branch when activated: `feature/TASK-037-render-export-v1`
Depends on: TASK-036 accepted composition snapshot/editor contract

## Product outcome
Creator can render an immutable composition snapshot into a durable video artifact, observe progress/failure/cancel, retry without losing edits and download reproducible MP4 plus supported subtitles.

## Scope
Immutable render request; durable render lifecycle; deterministic composition; first render engine/toolchain after license/runtime review; bounded temp/resource cleanup; presets/manual baseline settings; MP4/subtitle output; durable artifact provenance; creator progress/cancel/retry/history/download UI; integration/E2E failure recovery.

## Required behavior
Render never silently uses newer state; retry preserves exact logical snapshot/config unless creator changes them; partial output never appears successful; cancellation/cleanup idempotent; artifact records exact provenance; editor state survives failure.

## Activation gate
BACKLOG. Freeze render snapshot/output contract, revalidate FFmpeg/Remotion or selected engine license/runtime/deployment fit, then move issue #72 READY last.

## TDD focus
Exact snapshot, deterministic input, lifecycle/cancel/retry, cleanup, editor-state preservation, artifact provenance, subtitles and representative real render test.
