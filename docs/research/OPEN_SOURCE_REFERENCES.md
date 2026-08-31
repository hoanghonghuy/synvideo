# Open-Source / Reference Research

Purpose: prevent SynVideo agents from reinventing substantial subsystems before checking proven work.

**Important:** license notes here are research snapshots, not permanent legal truth. Verify the repository's current license immediately before copying/adapting code.

## Classification
- `REUSE-CANDIDATE`: potentially suitable for copying/adapting after current license and fit review.
- `LIBRARY`: prefer integrating as a dependency/API rather than copying internals.
- `STUDY-ONLY`: learn architecture/UX/flows; do not copy code.
- `OPTIONAL-INTEGRATION`: useful provider/tool but not core product requirement.

## End-to-end AI video pipelines
### MoneyPrinterTurbo
Repository: https://github.com/harry0703/MoneyPrinterTurbo
Snapshot classification: `REUSE-CANDIDATE`
Snapshot license: MIT (verified during PM bootstrap on 2026-08-31; re-check before reuse).
Study/use for:
- topic/script-to-video orchestration;
- stock footage workflow;
- TTS/subtitle/media composition patterns;
- failure points and practical pipeline sequencing.
Do not adopt its full product architecture blindly; SynVideo has a broader editable project/channel scope.

### short-video-maker
Repository: https://github.com/gyoridavid/short-video-maker
Snapshot classification: `REUSE-CANDIDATE` / architectural reference
Snapshot license: MIT (verified during PM bootstrap on 2026-08-31; re-check before reuse).
Study/use for:
- compact scene abstraction;
- Pexels/stock integration;
- TTS/caption/render pipeline;
- Remotion-oriented composition ideas;
- operational constraints for smaller machines.
Despite the name, do not allow its short-video assumptions to constrain SynVideo long-form support.

## AI-native editor / provider aggregation
### VideoSOS
Repository: https://github.com/timoncool/videosos
Classification: `STUDY-FIRST`; verify license and exact source provenance before any reuse.
Study for:
- provider/model catalog UX;
- image/video generation integration;
- cost tracking;
- timeline/editor behavior;
- generation history.
Its browser-local architecture is not automatically suitable for SynVideo's account/project/channel workspace.

## Editor architecture
### cutaway
Repository: https://github.com/S07K/cutaway
Snapshot classification: `REUSE-CANDIDATE` / `STUDY`
Snapshot license: MIT (verified during PM bootstrap on 2026-08-31; re-check before reuse).
Study for separation between timeline/editor state, rendering/media adapters and presentation.

### OpenVideo / react-video-editor
Repository: https://github.com/openvideodev/react-video-editor
Classification: `STUDY-ONLY` by default.
Reason: project uses a custom license with eligibility and derivative-product restrictions. Do not copy/adapt code into SynVideo unless a fresh legal/license review explicitly permits the intended use.
Study UI/editor patterns only.

## Rendering / composition
### Remotion
Repository/docs: https://github.com/remotion-dev/remotion / https://www.remotion.dev/
Classification: `LIBRARY-CANDIDATE`, subject to current licensing/commercial terms.
Use case: programmatic React-based composition/rendering.
Rule: verify current licensing for intended commercial/entity usage before adoption; do not assume MIT.

### FFmpeg / ffmpeg.wasm
Classification: `LIBRARY-CANDIDATE` depending runtime architecture.
Study/use for codecs, muxing, trimming, transcoding and low-level media processing. Review LGPL/GPL build/component implications for the exact distribution strategy.

## Local/self-hosted AI video
### ComfyUI + supported video workflows/models
Classification: `OPTIONAL-INTEGRATION`.
Potential future use: advanced/self-hosted generation workflows.
Do not make high-VRAM local inference a baseline requirement for the web product. Prefer provider adapters so hosted and self-hosted backends can coexist.

## Platform publishing APIs
These are not source-code reuse candidates; they are integration references whose capabilities/policies can change.

### YouTube Data API
- https://developers.google.com/youtube/v3/docs
- https://developers.google.com/youtube/v3/docs/videos/insert
Use for connected-channel metadata/video upload and supported management operations. Re-check OAuth scopes, quotas, verification/audit policy and scheduling semantics in the implementation task.

### TikTok Content Posting API
- https://developers.tiktok.com/products/content-posting-api/
- https://developers.tiktok.com/docs/en/content-posting-api-get-started
Use for direct-post or upload-to-draft capability where approved. Re-check scopes, audit requirements, media restrictions and creator-info UX rules in the implementation task.

## Research rule for coding agents
Before implementing a subsystem:
1. search this file by subsystem;
2. inspect the current upstream repo/docs;
3. verify license and activity;
4. choose reuse/library/study/reject;
5. record the decision in the task/ADR;
6. then implement.
