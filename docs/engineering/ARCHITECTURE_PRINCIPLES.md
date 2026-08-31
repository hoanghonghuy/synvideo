# Architecture Principles

These constraints are product-derived and remain technology-agnostic until the codebase/stack is audited.

## 1. Domain before provider
Core concepts: workspace/user, project, creative brief, proposal, script, scene, asset, track/audio/caption state, generation job, render, channel connection, publication.

Provider names/models are boundary configuration, not primary domain types.

## 2. Durable project state
A browser refresh, worker restart or provider callback must not silently destroy project progress. Expensive results and user approvals need durable state appropriate to the chosen architecture.

## 3. Asynchronous jobs are explicit
AI generation, transcription, media analysis, rendering, upload/publishing and other long operations are jobs with lifecycle, retry/error state and correlation to project/scene/assets.

Avoid request handlers that pretend multi-minute external work is synchronous.

## 4. Idempotency and duplicate protection
Retryable operations, callbacks and publishing actions need clear idempotency semantics so retries do not create duplicate media, charges or posts.

## 5. Asset identity and provenance
Generated, uploaded, stock and imported assets require stable identity and provenance. Do not rely only on ephemeral URLs from third-party providers.

## 6. Granular regeneration
Preserve accepted work. Regenerating scene 8 should not reset scenes 1–7 unless the user explicitly changes an upstream dependency that requires it.

## 7. Capability-driven providers
Interfaces expose capabilities instead of pretending every backend/platform has identical features.

Examples:
- video generation: supported modes/durations/aspect ratios;
- TTS: languages/voices/timing support;
- channel platform: publish/schedule/update/read analytics capabilities.

## 8. Secret boundaries
Provider/API credentials are encrypted/stored server-side where architecture requires it and are never logged or returned unnecessarily.

## 9. i18n boundary
User-visible copy comes from locale resources. Domain/internal errors should carry stable codes so presentation can localize messages.

## 10. Evolvable scene/timeline model
Scene-first UX must not make later multi-track editing impossible. Keep ordering, timing and asset relationships explicit enough for evolution.
