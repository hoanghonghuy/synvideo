# SynVideo Task Board

Current milestone: `F1 — CREATIVE WORKFLOW`

PM plans ahead. AI Developers may start only tasks explicitly marked `READY`; review fixes stay on the original branch/PR. Parallel work follows `docs/engineering/PARALLEL_WORK_PROTOCOL.md` and normally uses at most 3 implementation worktrees.

## Status synchronization invariant

`docs/tasks/BOARD.md`, the task file under `docs/tasks/TASK-XXX.md`, and the authoritative GitHub issue must agree on claimability.

- A PM transition to `READY`, `CHANGES_REQUESTED`, `DONE`, or another execution-relevant state must update all applicable control-plane sources in the same housekeeping pass.
- If an AI Developer observes conflicting states, it must **not guess** and must report the inconsistency instead of silently choosing an older source.
- A new implementation branch may be claimed only when both the current `develop` board and task file say `READY`; the GitHub issue should mirror that state.
- Existing review-fix branches continue their current PR even when the task is `CHANGES_REQUESTED`; they are not re-claimed as new work.

| ID | Task | Status | Spec / Notes |
|---|---|---|---|
| TASK-001 | Technical foundation and runnable project skeleton | DONE | Accepted PR #3. |
| TASK-002 | Project domain and persistence foundation | DONE | Accepted PR #8. |
| TASK-003 | Creative Brief backend and persistence | DONE | Accepted PR #9. |
| TASK-004 | Creative Brief frontend workspace | DONE | Accepted PR #10. |
| TASK-005 | AI provider capability and text-generation contracts | DONE | Accepted PR #11. |
| TASK-006 | AI Proposal domain, persistence and approval API | DONE | Accepted PR #17. |
| TASK-007 | AI Proposal generation engine | DONE | Accepted PR #16. |
| TASK-008 | AI Proposal frontend workspace | DONE | Accepted PR #20. |
| TASK-009 | AI Proposal generation job integration | DONE | Accepted PR #28. |
| TASK-010 | Durable job execution foundation | DONE | Accepted PR #21. |
| TASK-011 | Script domain, persistence and approval API | DONE | Accepted PR #22. |
| TASK-012 | Script generation engine | DONE | Accepted PR #24. |
| TASK-013 | Live OpenAI-compatible text provider adapter foundation | DONE | Accepted PR #29. |
| TASK-014 | Scene Plan generation engine | DONE | Accepted PR #27. |
| TASK-015 | Scene Plan domain and persistence foundation | DONE | Accepted PR #32. |
| TASK-016 | Media Asset + S3-compatible storage foundation | DONE | Accepted PR #33. |
| TASK-017 | Secure BYOK text provider settings and owner-scoped runtime | DONE | Accepted PR #35. |
| TASK-018 | Script durable generation integration | DONE | PR #48 squash `6bc3c86b...`. |
| TASK-019 | Script creator workspace | DONE | PR #47 squash `da01e58c...`. |
| TASK-020 | Scene media binding foundation | DONE | PR #46 squash `b80b8e7b...`. |
| TASK-021 | Scene Plan durable generation + API integration | DONE | PR #50 squash `9d2b5306...`. |
| TASK-022 | Scene Plan creator workspace | DONE | Issue #40 completed; PR #51 accepted head `e0cb568...`, CI #269, TL review `5089868599`, squash `c8d8618...`. |
| TASK-023 | Media Library + Scene Binding API integration | DONE | Issue #41 completed; PR #53 squash `7e3df69...`. |
| TASK-024 | Media Library + scene assignment workspace | READY | Issue #42. Dependencies TASK-022/TASK-023 are DONE; claim from latest `origin/develop` as `feature/TASK-024-media-workspace`. |
| TASK-025 | Provider-neutral visual generation foundation | DONE | Issue #43 completed; PR #49 squash `1c550f316...`. |
| TASK-026 | Live OpenAI image generation adapter | DONE | Issue #44 completed; PR #52 squash `cf4317ad...`. |
| TASK-027 | Provider-neutral TTS + OpenAI speech adapter foundation | CHANGES_REQUESTED | Issue #45 / PR #55. Head `1486daa...`; CI #270 green, TL blockers recorded in task/issue. Continue existing PR only. |
| TASK-028 | Secure multi-capability provider runtime and settings | BACKLOG | Issue #54 / `docs/tasks/TASK-028.md`. Requires accepted TASK-027, frozen `MULTI_CAPABILITY_PROVIDER_RUNTIME_V1`, and free shared settings/frontend hotspot. |

## Current implementation / review slots

- **Dev A — TASK-027 `CHANGES_REQUESTED`**: continue only existing PR #55; fix the authoritative blockers, sync latest `develop`, rerun full verification and wait for fresh exact-head CI.
- **Dev B — TASK-024 `READY`**: may claim now from latest `origin/develop` using `feature/TASK-024-media-workspace`.
- **Dev C — intentionally unallocated**: do not manufacture a conflicting task merely to fill the slot.

## Parallel safety

- TASK-024 owns Media Library frontend route/navigation/locale surfaces.
- TASK-027 owns only the minimum provider-core TTS extension plus isolated `providers/openaitts/**`.
- Do not activate TASK-028 provider-runtime/settings evolution until TASK-027 is accepted and TASK-024 releases shared frontend settings/router surfaces.
- Do not create micro-tasks merely to fill an implementation slot.

## Next activation path

1. Continue TASK-027 fixes on PR #55.
2. Start TASK-024 now from current `develop`.
3. After TASK-027 is accepted and TASK-024 releases the frontend hotspot, freeze `MULTI_CAPABILITY_PROVIDER_RUNTIME_V1` and promote TASK-028 when safe.
4. After TASK-028, plan durable per-scene generated-image acquisition/MediaAsset ingestion and durable narration/TTS orchestration as separate tasks.
5. Follow with stock/captions/music, Scene Editor/composition snapshot, render/export, publishing/channel management and production hardening/E2E.

## Architecture gates

- Core provider capabilities remain provider-neutral; vendor SDK/schema types stay in adapters.
- Image generation may be synchronous/streaming; video generation retains async Start/Poll/OpenResult with opaque external operation identity.
- Paid video orchestration must persist external operation identity before poll/resume to avoid duplicate submissions/cost after worker crash.
- Provider output URLs are not durable SynVideo assets; generated bytes must be ingested into MediaAsset storage with provenance.
- Credentials/ciphertext/base URLs/raw upstream responses never enter durable jobs or public resources.
- TTS input limits return explicit errors; later orchestration owns deterministic chunk/stitch/timing.
- Multi-capability runtime must reuse TASK-017's encrypted owner credential path rather than create image/TTS secret silos.

## Isolation / merge rules

- Each implementation task uses a dedicated worktree; shared/control checkout remains on `develop`.
- Maximum concurrent implementation worktrees normally equals 3.
- Review fixes stay on the original branch/PR.
- READY tasks are claimed by atomically creating the absent remote ref from latest `origin/develop`.
- Truthful RED → GREEN → REFACTOR is mandatory.
- Team Lead review + current-base green CI is the merge gate.
- Merged worktrees are cleaned before another claim.
- Do not self-merge or self-mark DONE.

Allowed statuses: `BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.
