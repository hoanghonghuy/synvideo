# TASK-026 — Live OpenAI image generation adapter

Status: BACKLOG
Milestone: F1 Creative Workflow
Planned wave: WAVE-F1-J candidate
Branch when activated: `feature/TASK-026-openai-image-provider`
Base: `develop`
Depends on: TASK-025 accepted.

## Goal
Implement the first live image-generation HTTP adapter behind the provider-neutral ImageGenerator port, with bounded output decoding and strict credential/error safety.

## Frozen contract
`docs/contracts/OPENAI_IMAGE_PROVIDER_V1.md`.

## Research decision
Do **not** implement OpenAI/Sora video in this task. Official OpenAI docs currently schedule the Sora Video API shutdown for 2026-09-24, so a new production adapter would be immediately deprecated.

## Primary ownership when activated
- new isolated `apps/api/internal/providers/openaiimage/**` or cohesive equivalent;
- adapter-local tests/httptest fixtures only.

Do not touch `main.go`, httpserver, provider settings/runtime, Media Asset/storage, jobs, Scene Plan or frontend.

## Required capability
- validated injected API key/base URL/model map/bounds;
- current OpenAI Images generation endpoint translation;
- conservative aspect/count mapping;
- bounded response/base64 image decode into provider-neutral generated binary;
- safe 401/403/429/4xx/5xx/invalid-response handling;
- cancellation/response close/redirect safety;
- deterministic image-capability registration factory.

## TDD
Implement every security/resource/error gate in `OPENAI_IMAGE_PROVIDER_V1`, using only local fake upstreams in CI.

## Activation gate
Do not claim until TASK-025 is merged. Revalidate official OpenAI Images endpoint/model behavior immediately before READY because media APIs change faster than core contracts.

Do not self-mark READY/DONE or self-merge.