# TASK-035 — Background Music & Audio Mix V1

Status: BACKLOG
Milestone: F1 Creative Workflow
Canonical branch when activated: `feature/TASK-035-background-music-mix`
Depends on: MediaAsset audio support; TASK-031 narration-aware semantics

## Product outcome
Creator can select/upload project music, control baseline level/trim/loop/ducking relative to narration, preview the relationship and persist composition-ready audio-mix configuration.

## Scope
Music selection from durable audio assets; durable trim/loop/level/ducking/timing config; validation against timing/narration lineage; replacement/history; creator selection/preview controls; render-neutral composition representation; tests/i18n/accessibility.

## Required behavior
Music references durable MediaAssets; settings are bounded/deterministic; referenced asset replacement/deletion is truthful; narration replacement invalidates incompatible timing/ducking assumptions; manual edits survive refresh.

## Activation gate
BACKLOG. Freeze audio-mix contract after TASK-031, then move issue #70 READY last.

## TDD focus
Timing bounds, asset type/isolation, narration-lineage invalidation, persistence/reload, deletion safety and dirty/stale recovery.
