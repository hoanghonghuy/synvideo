# SynVideo Task Board

Current milestone: `F1 — CREATIVE WORKFLOW`

PM plans ahead. AI Developers may start only PM-authorized executable work. Live authorization/lifecycle is on the authoritative GitHub task issue; this board is the queue/order/status mirror. Remote claim ownership is determined separately by canonical task branch / active PR.

Follow `docs/engineering/CONTROL_PLANE_PROTOCOL.md` before making workflow decisions.

## Task sizing rule
Prefer product/vertical feature slices once foundations exist. Do not create a separate task merely for one small field, endpoint, button, migration, validation, test or narrow bugfix when it belongs naturally inside an active feature. Split only for a distinct product/subsystem outcome, ownership/dependency boundary, substantial independent risk or real scope/merge coupling.

| ID | Task | Status | Notes |
|---|---|---|---|
| TASK-001 | Technical foundation and runnable project skeleton | DONE | Accepted. |
| TASK-002 | Project domain and persistence foundation | DONE | Accepted. |
| TASK-003 | Creative Brief backend and persistence | DONE | Accepted. |
| TASK-004 | Creative Brief frontend workspace | DONE | Accepted. |
| TASK-005 | AI provider capability and text-generation contracts | DONE | Accepted. |
| TASK-006 | AI Proposal domain, persistence and approval API | DONE | Accepted. |
| TASK-007 | AI Proposal generation engine | DONE | Accepted. |
| TASK-008 | AI Proposal frontend workspace | DONE | Accepted. |
| TASK-009 | AI Proposal generation job integration | DONE | Accepted. |
| TASK-010 | Durable job execution foundation | DONE | Accepted. |
| TASK-011 | Script domain, persistence and approval API | DONE | Accepted. |
| TASK-012 | Script generation engine | DONE | Accepted. |
| TASK-013 | Live OpenAI-compatible text provider adapter | DONE | Accepted. |
| TASK-014 | Scene Plan generation engine | DONE | Accepted. |
| TASK-015 | Scene Plan domain and persistence foundation | DONE | Accepted. |
| TASK-016 | Media Asset + S3-compatible storage foundation | DONE | Accepted. |
| TASK-017 | Secure BYOK text provider settings/runtime | DONE | Accepted. |
| TASK-018 | Script durable generation integration | DONE | Accepted. |
| TASK-019 | Script creator workspace | DONE | Accepted. |
| TASK-020 | Scene media binding foundation | DONE | Accepted. |
| TASK-021 | Scene Plan durable generation + API | DONE | Accepted. |
| TASK-022 | Scene Plan creator workspace | DONE | Accepted. |
| TASK-023 | Media Library + Scene Binding API | DONE | Accepted. |
| TASK-024 | Media Library + scene assignment workspace | DONE | Accepted. |
| TASK-025 | Provider-neutral visual generation foundation | DONE | Accepted. |
| TASK-026 | Live OpenAI image generation adapter | DONE | Accepted. |
| TASK-027 | Provider-neutral TTS + OpenAI speech adapter | DONE | Accepted. |
| TASK-028 | Secure multi-capability provider runtime/settings | DONE | Accepted PR #58. |
| TASK-029 | Durable generated-image acquisition + MediaAsset ingestion | DONE | Issue #60 closed; PR #63 squash `2dbba467...`. |
| TASK-030 | Per-scene AI Image Generation Workspace | DONE | Issue #65 closed; implementation accepted via PR #108. |
| TASK-031 | Scene Narration & Voice V1 | DONE | Issue #66 closed; implementation accepted via PR #77. |
| TASK-032 | Per-scene AI Video Generation V1 | DONE | Issue #67 closed; implementation accepted via PR #89. |
| TASK-033 | Stock Media Search & Acquisition V1 | READY | Issue #68; READY activation landed via PR #126; canonical implementation branch not yet claimed. |
| TASK-034 | Captions & Scene Timing V1 | DONE | Issue #69 closed; implementation accepted via PR #124. |
| TASK-035 | Background Music & Audio Mix V1 | IN_PROGRESS | Issue #70; canonical branch is claimed but currently requires sync to latest `develop` before implementation. |
| TASK-036 | Scene Editor V1 | BACKLOG | Issue #71; spec frozen; activation waits for accepted compatible TASK-033/TASK-035 implementation boundaries. |
| TASK-037 | Render & Export V1 | BACKLOG | Issue #72; depends on editor composition snapshot. |
| TASK-038 | Channel Hub & Publishing V1 | BACKLOG | Issue #73; depends on render artifact contract and platform revalidation. |

## Current F1 implementation supply
- TASK-035 is claimed on `feature/TASK-035-background-music-mix`; Developer must sync the stale branch to latest protected `develop` before adding implementation work.
- TASK-033 is independently `READY / CLAIMABLE`; Developer may create `feature/TASK-033-stock-media` from latest protected `develop` under its frozen task/contract.
- TASK-036 is the prepared NEXT BACKLOG item, but must not be activated until compatible TASK-033 stock-media and TASK-035 audio-mix implementation boundaries are accepted.
- Do not manufacture another feature task merely to fill a slot while this supply remains valid.

## Subsequent sequence
1. Finish TASK-035 audio mix and TASK-033 stock-media implementation independently within bounded WIP.
2. Revalidate and activate TASK-036 editor once those component boundaries are accepted and compatible.
3. TASK-037 render from immutable composition snapshots.
4. TASK-038 Channel Hub from accepted render artifacts.
5. Production hardening/E2E and richer intake follow product audit rather than micro-task generation.

## Architecture gates
- Provider capabilities remain provider-neutral; vendor types stay in adapters.
- Paid video orchestration persists external operation identity before poll/resume.
- Provider output URLs are not durable assets; generated/stock bytes ingest into MediaAsset.
- Credentials/ciphertext/base URLs/raw upstream responses never enter public/durable generic state.
- TASK-031 owns deterministic TTS chunk/stitch/timing and never truncates narration.
- Narration audio binding is separate from primary visual binding.
- Editor/render/publish consume immutable versioned snapshots and never silently follow newer upstream state.

## Isolation / merge rules
- Dedicated task isolation applies; cloud scheduled agents use canonical branch/atomic claim without inventing local worktree requirements.
- Maximum concurrent implementation tasks normally equals 3; use fewer when coupled.
- Review fixes stay on original branch/PR.
- READY activation requires docs/contracts/order on protected `develop`, fresh duplicate/overlap checks, then authoritative issue READY last.
- Truthful RED → GREEN → REFACTOR is mandatory.
- Team Lead exact-head/current-base green CI is merge gate.
- Implementation agents do not self-merge/self-mark DONE.

Allowed statuses: `BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.
