# TASK-031 — Scene Narration & Voice V1

Status: DONE
Milestone: F1 Creative Workflow
Owner role: AI Developer
PR target: develop
Canonical branch: `feature/TASK-031-scene-narration-voice`
Depends on: TASK-016, TASK-027, TASK-028, accepted Scene Plan + durable jobs foundations

## Problem / Goal
Deliver the production path from approved scene narration to durable audio with exact-text guarantees, deterministic chunk/stitch recovery, measured timing, dedicated scene narration history and creator UI as one feature.

## Frozen contract
`docs/contracts/SCENE_NARRATION_V1.md`

## Scope delivered
Immutable narration snapshot; owner-scoped TTS model/voice resolution; deterministic orchestration chunking; durable synthesis recovery; stitch/finalization/duration; generated-audio MediaAsset; dedicated scene narration binding/history; feature API/status/result; creator selection/status/refresh/audio preview/alternative/assign UI; PostgreSQL/object-storage/audio integration.

## Required behavior delivered
Narration is preserved exactly without silent trim/rewrite/truncation; execution uses snapped source; chunks synthesize in order; invalid/disabled options fail before paid work; durable retry reuses checkpointed work; checkpoint read/write failures fail safely instead of repeating paid TTS; final creator-visible audio is stored as MediaAsset with measured duration; narration binding/history is separate from visual binding; refresh resumes job state and deliberate regeneration preserves alternatives/history.

## Acceptance criteria
- [x] `SCENE_NARRATION_V1` is on `develop`.
- [x] exact narration/chunking invariants proven.
- [x] durable paid-work recovery proven, including read/write checkpoint failure regression tests.
- [x] final generated audio + measured duration persisted.
- [x] dedicated narration binding/history enforced.
- [x] complete creator workflow delivered.
- [x] secret-safe real integration/full CI green.

## Acceptance evidence
- Implementation PR #77 accepted exact head `5b871e5fcab47b597f3522a95c5bddc39e570421`.
- Final exact-head CI #354 passed `Frontend`, `Backend`, and `Local Infrastructure`; Backend ran migrations, gofmt, vet, full tests, build and real S3-compatible integration storage.
- Prior durable-recovery blocker was resolved before merge: `GetChunk` failure stops before TTS; `PutChunk` failure stops before later paid work/finalization.
- Squash merge on protected `develop`: `6753f8878da989c09b9015dea2be089cd874defc`.
- Issue #66 is closed completed.

## Primary write surface
Dedicated narration orchestration/domain/job/binding packages, migrations/repositories/handlers, minimal runtime wiring; dedicated frontend narration feature/API/tests with minimal route/locale integration.

## Completion
DONE. Do not re-claim TASK-031. New regressions or independent follow-on outcomes must be deduped against existing tasks and tracked separately.
