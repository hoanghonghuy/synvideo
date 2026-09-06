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
| TASK-033 | Stock Media Search & Acquisition V1 | DONE | Issue #68 closed; implementation accepted via PR #130 / squash `85ca3877...`. |
| TASK-034 | Captions & Scene Timing V1 | DONE | Issue #69 closed; implementation accepted via PR #124. |
| TASK-035 | Background Music & Audio Mix V1 | DONE | Issue #70 closed; implementation accepted via PR #129 / squash `0eb43d88...`. |
| TASK-036 | Scene Editor V1 | IN_PROGRESS | Issue #71; claimed on canonical `feature/TASK-036-scene-editor-v1` with draft PR #134. Exact inspected head `fc8c8085...`; E2E #104 green, CI #601 red at Backend `go vet`; PM/TL blockers/decisions canonicalized on #71/#134. |
| TASK-037 | Render & Export V1 | BACKLOG | Issue #72; NEXT BACKLOG, contract frozen; waits for a concrete accepted TASK-036 immutable composition snapshot plus renderer/deployment/safety activation decisions. |
| TASK-038 | Channel Hub & Publishing V1 | BACKLOG | Issue #73; depends on render artifact contract and platform revalidation. |

## Current F1 implementation supply
- TASK-036 is active/claimed on canonical `feature/TASK-036-scene-editor-v1` through draft PR #134. PM/TL exact-head fingerprint at this board refresh: `fc8c80852852612298f51c1d5a063c664a45607f`; E2E Acceptance #104 SUCCESS; CI #601 FAILURE at Backend `go vet ./...`. Developer action boundaries and frozen Scene Editor decisions are canonical on issue #71 / PR #134.
- There is currently no independent CLAIMABLE READY task while TASK-036 is active. Do not fabricate READY state around real dependency/activation gates.
- TASK-037 is NEXT BACKLOG. Keep it BACKLOG until TASK-036 produces the accepted immutable composition snapshot and its own READY-time renderer/toolchain/deployment/safety decisions are frozen.

## Subsequent sequence
1. Complete and accept TASK-036 Scene Editor through its canonical branch/PR.
2. Revalidate TASK-037 Render & Export against the concrete immutable composition snapshot, production safety dependencies, deployment resources and renderer/toolchain/license choices; activate only after those gates are satisfied.
3. TASK-038 Channel Hub from accepted render artifacts.
4. Production hardening/E2E and richer intake continue through their existing audited task owners and activation gates.

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
