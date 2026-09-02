# TASK-024 — Media Library + scene assignment workspace

Status: READY
Milestone: F1 Creative Workflow
Planned wave: WAVE-F1-J
Branch: `feature/TASK-024-media-workspace`
Base: latest `develop`
Depends on: TASK-022 and TASK-023 accepted/stable — satisfied.
Issue: #42

## Goal
Deliver the creator-facing Media Library and manual scene primary-visual assignment workflow without waiting for AI generation providers.

## Frozen contract
`docs/contracts/MEDIA_LIBRARY_WORKSPACE_V1.md`.

## Primary ownership
- `apps/web/src/features/media/**` or cohesive Media Library feature;
- Media/Scene Binding frontend API/types/tests;
- minimal `/projects/:id/media` route;
- minimal Scene Plan/Project navigation/deep-link integration;
- Media locale keys and only unavoidable shared styles.

Frontend-only. Do not modify `apps/api/**`.

## Required UX
- upload with type/size/progress/cancel/error states;
- asset library preview/provenance/filter basics;
- authorized image/video preview using Media content endpoint/Range playback;
- explicit delete confirmation and asset-in-use handling;
- approved Scene Plan scene list with bound/unbound visual state;
- assign/replace image/video asset per scene;
- deterministic replacement history;
- switching approved Scene Plan versions never auto-copies bindings;
- restore historical asset only via normal new assignment.

## Mandatory isolation
No AI image/video generation, stock search, audio/TTS, crop/fit/timing editor, render/publish or backend changes.

## TDD
Cover every regression in `MEDIA_LIBRARY_WORKSPACE_V1`, especially one-preview failure isolation, in-use delete conflict, per-scene replacement state, plan version switch and stale async route protection.

## Claim gate
TASK-022 and TASK-023 are DONE and the shared frontend hotspot is released. This task is claimable now.

Claim atomically from the then-current `origin/develop` using `feature/TASK-024-media-workspace`. Truthful RED → GREEN → REFACTOR, focused frontend tests, full `npm --prefix apps/web run verify`, and fresh PR CI are required.

Do not self-mark DONE or self-merge.
