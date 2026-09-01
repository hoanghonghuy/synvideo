# TASK-025 — Provider-neutral visual generation foundation

Status: CHANGES_REQUESTED
Milestone: F1 Creative Workflow
Wave: WAVE-F1-I early slot
Branch: `feature/TASK-025-visual-provider-foundation`
Base: `develop`
PR: #49
Review head: `77bd30c6e19bc212d3ec1b369908fe3ba0189833`
Logical TL review: `5080350928`
CI: #224 green on reviewed head
Issue: #43
Depends on: TASK-005 accepted; frozen `VISUAL_GENERATION_PROVIDER_V1`.

## Goal
Establish production-grade provider-neutral image and asynchronous video generation ports/registry bindings/fakes before live adapters or paid per-scene jobs are introduced.

## Frozen contract
`docs/contracts/VISUAL_GENERATION_PROVIDER_V1.md`.

## Primary ownership
- `apps/api/internal/providers/**` visual interfaces/types/registry extension;
- deterministic visual provider fakes/tests only.

No persistence, jobs, HTTP, runtime composition, provider settings, Media Asset code, live adapters, frontend or migrations.

## Review result on `77bd30c...`
The previous blocker set is resolved:
- image responses now reject video MIME and video results reject image MIME;
- deterministic fakes snapshot arbitrary bounded public `BinaryInput` implementations rather than relying on optional cloner behavior;
- regressions cover wrong-family MIME, reference MIME/size rejection, video cancellation, failed operations and result-unavailable behavior;
- exact-head CI #224 is green and ownership remains providers-only.

## Remaining blocker
The visual port still has two sources of model identity. `ImageGenerationRequest` / `VideoGenerationRequest` carry `ProviderID` and `ModelID`, and image response echoes them, even though the frozen contract says provider/model identity is resolved before the capability port is called.

Keeping those fields permits inconsistent calls such as resolving generator binding for model A while a request claims model B. Make the capability-specific registry binding/resolution the single source of truth: remove provider/model identity from visual request payloads and unnecessary response echo fields (or an equivalent design that makes contradictory identity impossible), then update fakes/tests.

## Accepted architecture to preserve
- separate synchronous `ImageGenerator` from asynchronous `VideoGenerator`;
- video `StartVideo` / `GetVideoOperation` / `OpenVideoResult` lifecycle;
- opaque external operation IDs;
- streaming/closable provider-neutral generated binaries;
- capability-specific image/video MIME validation;
- independent text/image/video registry bindings and multi-capability models;
- backward-compatible legacy text resolution;
- safe provider-neutral error categories;
- deterministic deep-cloned fake request capture;
- no production registration or cross-layer leakage.

## Final verification gate
After the identity fix:
- update all visual fakes/tests to prove resolved binding is the only model/provider identity source;
- targeted visual tests + `go test -race ./internal/providers/...`;
- full backend verification/vet;
- exact-head CI green;
- providers-only ownership boundary retained;
- sync latest `develop` before final merge if needed.

Continue only on the existing TASK-025 worktree/PR #49. Do not self-merge or self-mark DONE.