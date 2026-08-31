# ADR 0003 — Scene-first editor before full NLE timeline

Status: Accepted

## Decision
The first comprehensive editor is scene-first. Each scene can manage narration, visual/media, duration, captions, audio choices, transition and granular regenerate/replace actions.

The model must permit later mapping to a richer multi-track timeline without rewriting all project semantics.

## Why
AI-assisted creators usually need to fix AI choices at scene granularity before they need a professional NLE. A scene-first workflow is more approachable while still supporting long-form content if scene/project modeling is not duration-limited.

## Consequences
- No short-video-only assumptions in scene counts/duration/storage.
- Timeline concepts that matter later (time ranges, ordering, asset identity) should not be discarded.
