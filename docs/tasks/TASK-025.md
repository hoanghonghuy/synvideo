# TASK-025 — Provider-neutral visual generation foundation

Status: CHANGES_REQUESTED
Milestone: F1 Creative Workflow
Wave: WAVE-F1-I early slot
Branch: `feature/TASK-025-visual-provider-foundation`
Base: `develop`
PR: #49
Review head: `b0b33467ee264c6bf5b5db182371a464161aa7db`
Logical TL review: `5080185028`
CI: #217 green on reviewed head
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

## Current review blockers
Fix only on the existing PR/worktree while preserving the accepted architecture.

1. **Capability-specific result MIME validation.** The current shared generated MIME allowlist lets `ImageGenerationResponse.Validate()` accept `video/mp4`/`video/webm`, and video result opening can accept image MIME. Keep the shared `GeneratedBinary` abstraction, but image outputs must validate against image MIME only and video results against video MIME only. Add wrong-family regression tests in both directions.
2. **Deterministic fake deep-copy is conditional.** `cloneImageRequest`/`cloneVideoRequest` only deep-copy reference binaries if caller inputs implement optional `BinaryInputCloner`; otherwise fake capture retains caller-owned objects. Frozen contract requires captured deep-cloned requests for any valid public `BinaryInput`. Snapshot/copy bounded input content into fake-owned immutable data or use an equivalent guarantee, and test a custom mutable `BinaryInput` without `BinaryInputCloner`.
3. **Complete frozen boundary coverage.** Add focused tests for reference MIME/size rejection, failed video operation/result-unavailable behavior, video context cancellation, and the capability-specific result MIME gates.

## Accepted architecture to preserve
- separate synchronous `ImageGenerator` from asynchronous `VideoGenerator`;
- video `StartVideo` / `GetVideoOperation` / `OpenVideoResult` lifecycle;
- opaque external operation IDs;
- streaming/closable provider-neutral `GeneratedBinary`;
- independent text/image/video registry bindings and multi-capability models;
- backward-compatible legacy text resolution;
- safe provider-neutral error categories;
- no production registration or cross-layer leakage.

## Verification gate
After fixes:
- targeted visual tests including the new boundary regressions;
- `go test -race ./internal/providers/...`;
- full backend verification / vet;
- exact-head CI green;
- no paths outside the frozen TASK-025 ownership boundary.

## Worktree protocol
Continue only in the existing TASK-025 dedicated worktree/PR #49. Do not create a replacement branch, self-merge, or self-mark DONE.