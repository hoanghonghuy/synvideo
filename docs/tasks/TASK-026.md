# TASK-026 — Live OpenAI image generation adapter

Status: READY
Milestone: F1 Creative Workflow
Wave: WAVE-F1-J
Branch: `feature/TASK-026-openai-image-provider`
Base: `develop`
Depends on: TASK-025 accepted.

## Goal
Implement the first live image-generation HTTP adapter behind the provider-neutral ImageGenerator port, with bounded output decoding and strict credential/error safety.

## Frozen contract
`docs/contracts/OPENAI_IMAGE_PROVIDER_V1.md`.

## Current API revalidation
Revalidated at activation on 2026-09-02 against current official OpenAI API docs:
- Image generation remains available through `POST /v1/images/generations`;
- current state-of-the-art image model is `gpt-image-2`;
- the adapter contract already uses stable internal ModelID -> external model mapping, so implementation must not hard-code a deprecated GPT-Image-1 family identifier;
- keep model behavior/config data-driven and test against local fake upstreams only.

Do **not** implement OpenAI/Sora video in this task. Video remains a separately revalidated provider choice.

## Primary ownership
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

## Critical gates
1. External model ID comes only from injected stable model mapping; do not hard-code old image model aliases.
2. No raw upstream body, API key or Authorization leak.
3. Bounded response JSON and decoded image aggregate bytes.
4. Unsupported generic hints fail explicitly rather than being silently reinterpreted.
5. Redirects do not forward authorization cross-origin.
6. No production runtime registration or Media/job scope leakage.

## TDD
Implement every security/resource/error gate in `OPENAI_IMAGE_PROVIDER_V1`, using only local fake upstreams in CI. Include a regression proving a configured `gpt-image-2` external mapping is sent exactly as configured.

## Worktree / claim
Remote `feature/TASK-026-openai-image-provider` was absent when PM/TL promoted this task. Atomically claim it from latest `origin/develop`, use a dedicated worktree, and keep the shared/control checkout on `develop`.

Do not self-mark DONE or self-merge.