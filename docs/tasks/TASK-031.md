# TASK-031 — Scene Narration & Voice V1

Status: READY
Milestone: F1 Creative Workflow
Owner role: AI Developer
PR target: develop
Canonical branch when activated: `feature/TASK-031-scene-narration-voice`
Depends on: TASK-016, TASK-027, TASK-028, accepted Scene Plan + durable jobs foundations

## Problem / Goal
Deliver the production path from approved scene narration to durable audio with exact-text guarantees, deterministic chunk/stitch recovery, measured timing, dedicated scene narration history and creator UI as one feature.

## Frozen contract
`docs/contracts/SCENE_NARRATION_V1.md`

## Scope
Immutable narration snapshot; owner-scoped TTS model/voice resolution; deterministic orchestration chunking; durable synthesis recovery; stitch/finalization/duration; generated-audio MediaAsset; dedicated scene narration binding/history; feature API/status/result; creator selection/status/refresh/audio preview/alternative/assign UI; PostgreSQL/object-storage/audio integration.

## Required behavior
Never trim/rewrite/truncate narration; execute against snapped source; all chunks synthesized in order; invalid/disabled options fail before paid work; duplicate request/worker delivery does not duplicate logical outcome; retry reuses durable chunk work where safe; final asset is normal creator-visible audio; audio binding separate from visual; asset and binding success distinct; refresh resumes same job; deliberate regeneration preserves history.

## Acceptance criteria
- [x] `SCENE_NARRATION_V1` is on `develop`.
- [ ] exact narration/chunking invariants proven.
- [ ] durable paid-work recovery proven.
- [ ] final generated audio + measured duration persisted.
- [ ] dedicated narration binding/history enforced.
- [ ] complete creator workflow delivered.
- [ ] secret-safe and real integration/full CI green.

## Primary write surface
Dedicated narration orchestration/domain/job/binding packages, migrations/repositories/handlers, minimal runtime wiring; dedicated frontend narration feature/API/tests with minimal route/locale integration.

## Open-source gate
Inspect current MoneyPrinterTurbo and short-video-maker references/licenses before implementation.

## TDD
First RED set covers exact narration, chunk recomposition, idempotency, durable chunk recovery, final asset ingestion and replacement history.

## Activation gate
READY — task/contract are on protected `develop`, foundational dependencies are accepted, duplicate/canonical-branch checks are clean, and issue #66 is the authoritative live authorization once updated to READY. Claim only by atomically creating `feature/TASK-031-scene-narration-voice` from current `origin/develop`.
