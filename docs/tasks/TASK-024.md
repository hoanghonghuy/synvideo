# TASK-024 — Media Library + scene assignment workspace

Status: DONE
Milestone: F1 Creative Workflow
Planned wave: WAVE-F1-J
Branch: `feature/TASK-024-media-workspace`
Base: latest `develop`
Depends on: TASK-022 and TASK-023 accepted/stable — satisfied.
Issue: #42
PR: #56
Accepted head: `25db940f9b726fd50b00cda2cf9479c12292e8e2`
Squash merge: `f0aa549f932c27e10b23ecf8420fe6992f1866f7`

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
Required regressions covered under `MEDIA_LIBRARY_WORKSPACE_V1`, including preview-failure isolation, in-use delete conflict, per-scene replacement state, plan-version switching and stale async route protection.

## Accepted result
PR #56 was accepted on exact head `25db940f9b726fd50b00cda2cf9479c12292e8e2`, TL re-review `5090827272`, exact-head CI #279 green, and squash-merged to `develop` as `f0aa549f932c27e10b23ecf8420fe6992f1866f7`.

The task is complete. Do not re-claim or continue implementation from this task spec; any new outcome requires PM duplicate/overlap review under `docs/engineering/CONTROL_PLANE_PROTOCOL.md`.
