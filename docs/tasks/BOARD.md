# SynVideo Task Board

Current milestone: `F1 — CREATIVE WORKFLOW`

PM plans ahead. AI Developers may start only tasks explicitly marked `READY`; review fixes stay on the original branch/PR. Parallel work follows `docs/engineering/PARALLEL_WORK_PROTOCOL.md` and normally uses at most 3 implementation worktrees.

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
| TASK-018 | Script durable generation integration | DONE | PR #48 accepted head `c5e682d8...`, CI #215, squash `6bc3c86b...`. |
| TASK-019 | Script creator workspace | DONE | PR #47 accepted head `a9f0c5ad...`, CI #223, TL review `5080349001`, squash `da01e58c...`; Issue #37 completed. |
| TASK-020 | Scene media binding foundation | DONE | PR #46 accepted; squash `b80b8e7b...`. |
| TASK-021 | Scene Plan durable generation + API integration | READY | Issue #39. `SCENE_PLAN_JOB_V1` frozen. Backend hotspot is free; remote branch absent at promotion. |
| TASK-022 | Scene Plan creator workspace | BACKLOG | Issue #40. `SCENE_PLAN_WORKSPACE_V1` frozen. Activate after TASK-021 API is accepted/stable. |
| TASK-023 | Media Library + Scene Binding API integration | BACKLOG | Issue #41. `MEDIA_LIBRARY_API_V1` frozen. Do not run concurrently with TASK-021 because both own shared backend composition/httpserver. |
| TASK-024 | Media Library + scene assignment workspace | BACKLOG | Issue #42. `MEDIA_LIBRARY_WORKSPACE_V1` frozen. Activate after TASK-023 API. |
| TASK-025 | Provider-neutral visual generation foundation | CHANGES_REQUESTED | Issue #43 / PR #49. Head `77bd30c6...`, CI #224, TL review `5080350928`. Prior MIME/deep-copy/test blockers fixed; remove duplicated ProviderID/ModelID from visual port request/response identity. |
| TASK-026 | Live OpenAI image generation adapter | BACKLOG | Issue #44. Depends on accepted TASK-025; revalidate current Images API before READY. |
| TASK-027 | Provider-neutral TTS + OpenAI speech adapter foundation | BACKLOG | Issue #45. Depends on accepted TASK-025; never silently truncate narration. |

## Current implementation slots

- **Dev A — TASK-021 `READY`**: may atomically claim `feature/TASK-021-scene-plan-generation-integration` from latest `origin/develop`. Owns Scene Plan durable generation/API + minimal runtime composition.
- **Dev B — free**: do not fill with a task that conflicts with TASK-021 or depends on unfinished TASK-025. TASK-022 waits for TASK-021; TASK-023 conflicts with TASK-021 backend hotspot.
- **Dev C — TASK-025 `CHANGES_REQUESTED`**: continue only PR #49 / existing worktree, providers-only.

## Current review gate — TASK-025 / PR #49

Previous review blockers are resolved. Remaining gate:
1. registry/model resolution must be the single source of provider/model identity;
2. remove `ProviderID` / `ModelID` from visual port request payloads and unnecessary response echo fields, or an equivalent design that makes contradictory identity impossible;
3. update fakes/tests accordingly while preserving image sync vs video async Start/Poll/OpenResult architecture;
4. sync latest `develop` if needed and obtain exact-head green CI before merge.

## Parallel safety

- TASK-021 and TASK-023 both need shared backend `main.go` / `httpserver` composition; only one runs at a time.
- TASK-022 and TASK-024 both use frontend router/locale/project-workspace integration; sequence them.
- TASK-025 is isolated under provider-core visual abstractions.
- TASK-026 and TASK-027 remain blocked until TASK-025 is accepted.
- Do not create micro-tasks merely to fill an implementation slot.

## Next activation path

1. TASK-021 Scene Plan durable generation/API.
2. After TASK-021 acceptance, activate TASK-022 Scene Plan creator workspace.
3. Then TASK-023 Media Library + Scene Binding API, followed by TASK-024 workspace.
4. When TASK-025 is accepted, revalidate and schedule TASK-026 OpenAI Image adapter and TASK-027 TTS foundation on independent provider surfaces as slots allow.
5. Follow with secure multi-capability runtime/settings, durable per-scene visual/audio acquisition jobs, generated-output ingestion into Media Asset + Scene binding, captions/music, Scene Editor, render/export, publishing/channel management and production hardening/E2E.

## Product checkpoint

Stage 5 Script is now creator-usable end to end: persistence/approval, provider-neutral generation engine, durable owner-scoped BYOK generation backend and creator workspace are all accepted.

Accepted foundations also include Scene Plan generation + persistence, Media Asset S3-compatible storage and approved Scene Plan → primary visual Media Asset binding with replacement history. The current critical path is Stage 7 Scene Plan durable generation/workspace, then Media Library/assignment and visual/audio acquisition.

## Architecture gates

- Core provider capabilities remain provider-neutral; vendor SDK/schema types stay at adapters.
- Image generation may be synchronous/streaming; video generation retains async Start/Poll/OpenResult with opaque external operation identity.
- Paid video orchestration must persist external operation identity before poll/resume to avoid duplicate submissions/cost after worker crash.
- Provider output URLs are not durable SynVideo assets; generated bytes must be ingested into MediaAsset storage with provenance.
- Credentials/ciphertext/base URLs/raw upstream responses never enter durable jobs or public resources.
- TTS input limits must return explicit errors; later orchestration owns deterministic chunk/stitch/timing.

## Isolation / merge rules

- Each implementation task uses a dedicated worktree; shared/control checkout remains on `develop`.
- Maximum concurrent implementation worktrees normally equals 3.
- Review fixes stay on the original branch/PR.
- READY tasks are claimed by atomically creating the absent remote ref from latest `origin/develop`.
- Truthful RED → GREEN → REFACTOR is mandatory.
- Team Lead review + exact-head green CI is the merge gate.
- Merged worktrees are cleaned before another claim.
- Do not self-merge or self-mark DONE.

Allowed statuses: `BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.
