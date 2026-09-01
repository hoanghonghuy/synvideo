# F1 Two-Wave Lookahead — planned WAVE-F1-I / WAVE-F1-J

Status: PLANNING. This file does not authorize branch claims; only `BOARD.md` status `READY` does.

## Current active wave — WAVE-F1-H
- TASK-018 — Script durable generation integration — backend shared runtime hotspot.
- TASK-019 — Script creator workspace — frontend shared route/locale hotspot.
- TASK-020 — Scene Media Binding foundation — isolated scene/media persistence.

All three branches are claimed; future tasks stay BACKLOG until slots and write surfaces release.

## Candidate WAVE-F1-I after H
### Slot A — TASK-021 Scene Plan durable generation + API
Uses the backend runtime/jobs/httpserver slot after TASK-018.

### Slot B — TASK-022 Scene Plan creator workspace
Uses the frontend route/locale/navigation slot after TASK-019 and can implement against frozen TASK-021 API once PM/TL revalidates it.

### Slot C — TASK-025 provider-neutral visual generation foundation
Uses isolated `providers/**` surface and can be promoted earlier than the full wave if one implementation slot becomes free and no active task touches core provider contracts.

This wave completes Stage 7 backend/frontend while preparing image/video provider primitives without competing for the same files.

## Candidate WAVE-F1-J
### Slot A — TASK-023 Media Library + Scene Binding API
Uses backend runtime/config/httpserver after TASK-021 and accepted TASK-020.

### Slot B — TASK-024 Media Library + scene assignment workspace
Uses frontend shared route/locale after TASK-022, implementing against frozen TASK-023 API.

### Slot C — TASK-026 live OpenAI image adapter
Isolated adapter package after TASK-025.

TASK-027 TTS foundation remains the next isolated provider task and can replace TASK-026 in Slot C if priorities change, or run immediately after a slot releases.

## Research-driven sequence after TASK-027
Do not jump directly from a live adapter to paid per-scene generation jobs.

### A. Multi-capability secure provider runtime
Current TASK-017 provider settings/runtime are text-specific. Evolve the secure owner/provider credential/catalog model to expose capability-aware image/video/TTS models and voices without duplicating plaintext/credential paths.

Prefer one encrypted credential per owner/provider where the upstream credential legitimately spans capabilities, with explicit capability/model/voice enablement and safe runtime resolvers.

### B. Live video provider adapter research gate
OpenAI currently marks Sora Video API for shutdown on 2026-09-24, so no new Sora production adapter is planned.

Google Gemini API Veo 3.1 currently exposes long-running video operations and no shutdown date is announced for the current preview family. Revalidate immediately before freezing a live video adapter task; media APIs have short deprecation cycles.

### C. Durable per-scene visual orchestration
Only after provider ports + live adapter + secure runtime exist:
- snapshot approved Scene Plan version/scene intent;
- persist a feature request record including opaque external video operation ID after submit;
- generic job retries poll/resume the same external operation;
- successful bytes ingest into Media Asset;
- binding to scene is explicit/idempotent and does not destroy old selected asset/history;
- no provider secret in durable payload/result.

### D. Durable scene narration/TTS orchestration
- snapshot exact approved Scene Plan narration;
- deterministically chunk only when provider limit requires it;
- synthesize every chunk, never truncate;
- stitch/measure timing;
- ingest final audio into Media Asset;
- preserve per-scene audio replacement history in a later audio-binding model.

### E. Stock/captions/music/editor/render/publish
After visual/audio identity and timing are durable:
1. stock provider search/acquisition with license/source attribution;
2. captions/transcription/caption style model;
3. background music asset/level/timing model;
4. scene-first editable composition snapshot;
5. deterministic render/export jobs;
6. Channel Hub credentials/publish/schedule/history;
7. production hardening, richer source intake and full E2E regressions.

## PM rule
Freeze at most ~2 waves ahead for volatile provider APIs. Stable internal domain contracts may be frozen earlier; external adapter details are revalidated immediately before READY.