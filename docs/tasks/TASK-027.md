# TASK-027 — Provider-neutral TTS + OpenAI speech adapter foundation

Status: READY
Milestone: F1 Creative Workflow
Planned wave: post-WAVE-F1-I/J candidate
Branch: `feature/TASK-027-tts-provider-foundation`
Base: latest `develop`
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

## Required capability
- stable internal VoiceID/VoiceMetadata;
- SpeechSynthesizer port + registry resolver;
- exact-text request semantics and streaming generated audio;
- explicit finite input limit with **no silent truncation**;
- deterministic fake;
- OpenAI `/v1/audio/speech` adapter with model/voice mapping, bounded streaming, redirect/error/secret safety.

## Critical product gate
Approved scene narration can exceed a provider's per-request TTS limit. This adapter must fail explicitly rather than truncate/summarize. A later durable scene-audio orchestration task owns deterministic chunking, synthesis of every chunk, ordered stitching and timing.

## TDD
Implement all registry/fake/live-adapter tests in `TTS_PROVIDER_V1`, including exact narration preservation, over-limit no-network behavior, stream close/cancel and credential/raw-body sentinel safety.

## Claim rules
- claim only by creating the branch from latest `origin/develop`;
- do not reuse an older provider/image worktree;
- preserve all existing text/image/video registry behavior;
- truthful RED → GREEN → REFACTOR is mandatory;
- run focused tests, `go test -race ./...`, full backend verification and fresh PR CI before review.

Do not self-mark DONE or self-merge.