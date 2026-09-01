# TASK-025 — Provider-neutral visual generation foundation

Status: DONE
Milestone: F1 Creative Workflow
Wave: WAVE-F1-I early slot
Branch: `feature/TASK-025-visual-provider-foundation`
Base: `develop`
PR: #49
Accepted head: `e090ed3e56d201200e8f8f9d3e8efec2c9f9d42f`
Logical TL review: `5080539748`
CI: #230 green on accepted head
Squash merge: `1c550f3165efc5a541177deebd40bedbfd2ba16c`
Issue: #43 completed
Depends on: TASK-005 accepted; frozen `VISUAL_GENERATION_PROVIDER_V1`.

## Goal
Establish production-grade provider-neutral image and asynchronous video generation ports/registry bindings/fakes before live adapters or paid per-scene jobs are introduced.

## Frozen contract
`docs/contracts/VISUAL_GENERATION_PROVIDER_V1.md`.

## Accepted result
- separate synchronous `ImageGenerator` and asynchronous `VideoGenerator`;
- video `StartVideo` / `GetVideoOperation` / `OpenVideoResult` lifecycle with opaque operation IDs;
- streaming/closable provider-neutral generated binaries and bounded reference inputs;
- capability-specific image/video MIME validation;
- independent text/image/video registry bindings and multi-capability models;
- backward-compatible text resolution;
- safe provider-neutral visual error categories;
- deterministic fake request capture that snapshots arbitrary bounded `BinaryInput` values;
- cancellation, failed-operation, result-unavailable, reference-bound and wrong-MIME regressions;
- capability-specific registry binding is the single provider/model identity boundary: visual request/response payloads do not duplicate resolved provider/model identity;
- no persistence/jobs/http/runtime/media/frontend/live-adapter leakage.

## Verification
- exact-head CI #230 green;
- provider race tests and full backend verification green;
- logical TL APPROVE review `5080539748`;
- squash merged into `develop` as `1c550f3165efc5a541177deebd40bedbfd2ba16c`.

TASK-025 is complete. Follow-on live image/TTS adapter work must build on this accepted contract rather than reintroduce vendor identity into core request types.