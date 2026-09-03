# TASK-034 — Captions & Scene Timing V1

Status: BACKLOG
Milestone: F1 Creative Workflow
Canonical branch when activated: `feature/TASK-034-captions-timing`
Depends on: TASK-031 accepted scene narration/timing foundation

## Product outcome
Creator can derive/generate timed captions from current narration/audio, edit text/timing safely, choose baseline style, and keep caption state truthful when narration/audio changes.

## Scope
Versioned caption domain tied to exact audio lineage; ordered validated timing; initial generation/rebuild path; explicit stale/invalidation rules; editable text/timing with concurrency/history; render-neutral style model; creator UI; subtitle-export-ready representation; tests/i18n/accessibility.

## Required behavior
Captions identify exact source version; narration replacement cannot silently leave captions current; manual edits are not silently overwritten; timing remains within duration; stale captions can be intentionally rebuilt; style remains render-engine neutral.

## Activation gate
BACKLOG. After TASK-031, freeze caption lineage/timing/style contract on `develop`, then move issue #69 READY last.

## TDD focus
Lineage staleness, timing validation, manual edit preservation, rebuild semantics, concurrency, isolation and frontend truthfulness.
