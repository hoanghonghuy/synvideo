# TASK-027 — Provider-neutral TTS + OpenAI speech adapter foundation

Status: CHANGES_REQUESTED
Milestone: F1 Creative Workflow
Branch: `feature/TASK-027-tts-provider-foundation`
Base: latest `develop`
PR: #55
Current reviewed head: `1486daa4642755d7c4fa014e44bf7534020468d8`
CI: #270 green on reviewed head
Issue: #45
Depends on: TASK-025 accepted registry evolution — satisfied.

## Goal
Establish provider-neutral TTS/voice contracts and the first live OpenAI speech adapter while keeping narration integrity, streaming audio and provider limits explicit before scene-level audio jobs are built.

## Frozen contract
`docs/contracts/TTS_PROVIDER_V1.md`.

## Current API revalidation
Revalidated 2026-09-02 against official OpenAI API/model documentation: speech generation remains available through `/v1/audio/speech` and `gpt-4o-mini-tts` remains a current speech-generation model. External model/voice mapping stays injected adapter configuration rather than a hard-coded creator-visible identity.

## Primary ownership
- minimum TTS-specific extension under `apps/api/internal/providers/**`;
- deterministic TTS fake;
- isolated `apps/api/internal/providers/openaitts/**` or cohesive equivalent;
- adapter/core TTS tests only.

No jobs, Media Asset persistence, Scene Plan changes, provider settings/runtime UI, HTTP/main/frontend, transcription/captions/render.

## Accepted so far
- provider-neutral VoiceID/VoiceMetadata and speech request/response surface;
- TTS registry binding/resolution isolated from legacy text/image/video behavior;
- deterministic TTS fake with exact narration capture and cancellation;
- isolated OpenAI `/v1/audio/speech` adapter;
- request-time credential source, no redirects, bounded streaming and safe status errors;
- exact narration preservation with no truncation path.

## Authoritative blockers
Fix only on existing PR #55/worktree.

1. Plain HTTP is allowed too broadly for `*.test`; only validated loopback test targets may carry credentials over HTTP.
2. The default input bound must conservatively reflect the selected upstream model limit; over-bound narration must fail before credential lookup/network.
3. Empty request format must select a deterministic configured default rather than unconditionally choosing MP3 and breaking WAV-only configuration.
4. Sync/rebase latest `develop` after TASK-022 merge, preserve scope, run focused providers tests plus full/race/vet verification, and obtain fresh exact-head CI.

## Critical product gate
Never truncate, summarize or rewrite approved narration. Later durable scene-audio orchestration owns deterministic chunking, complete synthesis, ordered stitching and timing.

Do not self-mark DONE or self-merge.
