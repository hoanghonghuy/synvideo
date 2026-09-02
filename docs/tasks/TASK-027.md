# TASK-027 — Provider-neutral TTS + OpenAI speech adapter foundation

Status: DONE
Milestone: F1 Creative Workflow
Branch: `feature/TASK-027-tts-provider-foundation`
Base: latest `develop`
PR: #55
Issue: #45
Accepted head: `e00413b1d1698945c216d6cf9a01e961181b16b1`
Exact-head CI: #281
Squash merge: `a98928fb051fe1b9ff7fb5465b7dd12fd8d03dd8`
Depends on: TASK-025 accepted registry evolution — satisfied.

## Goal
Establish provider-neutral TTS/voice contracts and the first live OpenAI speech adapter while keeping narration integrity, streaming audio and provider limits explicit before scene-level audio jobs are built.

## Frozen contract
`docs/contracts/TTS_PROVIDER_V1.md`.

## API revalidation
Revalidated 2026-09-02 against official OpenAI API/model documentation: speech generation remains available through `/v1/audio/speech` and `gpt-4o-mini-tts` remains a current speech-generation model. External model/voice mapping stays injected adapter configuration rather than a hard-coded creator-visible identity.

## Accepted scope
- provider-neutral VoiceID/VoiceMetadata and speech request/response surface;
- TTS registry binding/resolution isolated from legacy text/image/video behavior;
- deterministic TTS fake with exact narration capture and cancellation;
- isolated OpenAI `/v1/audio/speech` adapter;
- request-time credential source, no redirects, bounded streaming and safe status errors;
- exact narration preservation with no truncation path;
- plain HTTP restricted to validated loopback targets;
- conservative finite input bounds rejecting overflow before credentials/network;
- deterministic configured default audio format.

No jobs, Media Asset persistence, Scene Plan changes, provider settings/runtime UI, HTTP/main/frontend, transcription/captions/render scope was added.

## Critical product gate
Never truncate, summarize or rewrite approved narration. Later durable scene-audio orchestration owns deterministic chunking, complete synthesis, ordered stitching and timing.

## Accepted result
PR #55 was accepted on exact head `e00413b1d1698945c216d6cf9a01e961181b16b1`, exact-head CI #281 passed, and the PR was squash-merged into `develop` as `a98928fb051fe1b9ff7fb5465b7dd12fd8d03dd8`.

The task is complete. Previous `CHANGES_REQUESTED` blockers are historical/resolved and must not be replayed against future work. Do not re-claim this task; new work requires PM duplicate/overlap review under `docs/engineering/CONTROL_PLANE_PROTOCOL.md`.
