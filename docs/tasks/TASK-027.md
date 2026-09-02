# TASK-027 — Provider-neutral TTS + OpenAI speech adapter foundation

Status: BACKLOG
Milestone: F1 Creative Workflow
Planned wave: post-WAVE-F1-I/J candidate
Branch when activated: `feature/TASK-027-tts-provider-foundation`
Base: `develop`
Depends on: TASK-025 accepted registry evolution.

## Goal
Establish provider-neutral TTS/voice contracts and the first live OpenAI speech adapter while keeping narration integrity, streaming audio and provider limits explicit before scene-level audio jobs are built.

## Frozen contract
`docs/contracts/TTS_PROVIDER_V1.md`.

## Current API revalidation
Revalidated 2026-09-02: OpenAI speech generation remains available through `/v1/audio/speech`; current model catalog includes GPT-4o Mini TTS for speech generation. The contract remains structurally valid because external model/voice mapping is adapter configuration rather than a hard-coded model identity.

## Primary ownership when activated
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

## Activation gate
Dependency and current-provider revalidation gates are satisfied. This task remains BACKLOG only because TASK-022, TASK-023 and TASK-026 occupy the normal maximum three implementation slots. Promote TASK-027 immediately when one slot is released, provided no active provider-core fix overlaps the minimal TTS registry extension.

Do not self-mark READY/DONE or self-merge.