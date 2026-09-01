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
| TASK-018 | Script durable generation integration | DONE | PR #48 accepted; squash `6bc3c86b...`. |
| TASK-019 | Script creator workspace | DONE | PR #47 accepted; squash `da01e58c...`. |
| TASK-020 | Scene media binding foundation | DONE | PR #46 accepted; squash `b80b8e7b...`. |
| TASK-021 | Scene Plan durable generation + API integration | CHANGES_REQUESTED | Issue #39 / PR #50. Reviewed head `286c583b...`, CI #231, TL review `5080593928`. Fix strict pre-provider snapshot validation, job-kind scoped status GET, then sync latest `develop`. |
| TASK-022 | Scene Plan creator workspace | BACKLOG | Issue #40. `SCENE_PLAN_WORKSPACE_V1` frozen. Activate after TASK-021 API is accepted/stable. |
| TASK-023 | Media Library + Scene Binding API integration | BACKLOG | Issue #41. `MEDIA_LIBRARY_API_V1` frozen. Do not run concurrently with TASK-021 because both own shared backend composition/httpserver. |
| TASK-024 | Media Library + scene assignment workspace | BACKLOG | Issue #42. `MEDIA_LIBRARY_WORKSPACE_V1` frozen. Activate after TASK-023 API. |
| TASK-025 | Provider-neutral visual generation foundation | DONE | Issue #43 completed. PR #49 accepted head `e090ed3e...`, CI #230, TL review `5080539748`, squash `1c550f316...`. |
| TASK-026 | Live OpenAI image generation adapter | BACKLOG | Issue #44. TASK-025 prerequisite is now satisfied; revalidate current Images API before READY and schedule only on an isolated provider-adapter slot. |
| TASK-027 | Provider-neutral TTS + OpenAI speech adapter foundation | BACKLOG | Issue #45. TASK-025 prerequisite is now satisfied; never silently truncate narration. |

## Current implementation / review slots

- **Dev A — TASK-021 `CHANGES_REQUESTED`**: continue only PR #50 / existing worktree. Backend Scene Plan generation/API owns the shared runtime/httpserver hotspot until accepted.
- **Dev B — free**: do not start TASK-022 until TASK-021 API is accepted; do not start TASK-023 because it conflicts with TASK-021 backend composition.
- **Dev C — released after TASK-025 DONE**: clean the old TASK-025 worktree before claiming another task. TASK-026/027 are candidate follow-ons but remain BACKLOG until deliberate activation/revalidation.

## Current review gate — TASK-021 / PR #50

Do not merge until:
1. durable Script/Proposal/Project snapshot validation is complete before credential/provider resolution; malformed approved Script snapshot data (including body/key/duplicate/size/duration/notes invariants and equivalent bounded fields) terminalizes as `GENERATION_INVALID_PAYLOAD`, with tests proving resolver/provider is not called;
2. `GetGeneration` verifies `job.Kind == scene_plan_generation_v1` and treats other feature job IDs as not found/non-disclosed;
3. branch is synced with latest `develop`, including TASK-025 squash `1c550f316...` and PM control-plane commits;
4. fresh exact-head CI/race/full verify is green;
5. already-correct source snapshot, idempotency, narration preservation, locale, owner runtime and single-executor behavior is preserved.

## Parallel safety

- TASK-021 and TASK-023 both need shared backend `main.go` / `httpserver` composition; only one runs at a time.
- TASK-022 and TASK-024 both use frontend router/locale/project-workspace integration; sequence them.
- TASK-025 provider-neutral visual core is accepted; TASK-026 and TASK-027 may be activated later on deliberately isolated surfaces after current-provider API revalidation.
- Do not create micro-tasks merely to fill an implementation slot.

## Next activation path

1. Fix/accept TASK-021 Scene Plan durable generation/API.
2. After TASK-021 acceptance, activate TASK-022 Scene Plan creator workspace.
3. Then TASK-023 Media Library + Scene Binding API, followed by TASK-024 workspace.
4. With TASK-025 accepted, revalidate and schedule TASK-026 OpenAI Image adapter and TASK-027 TTS foundation on independent provider surfaces as implementation slots permit.
5. Follow with secure multi-capability runtime/settings, durable per-scene visual/audio acquisition jobs, generated-output ingestion into Media Asset + Scene binding, captions/music, Scene Editor, render/export, publishing/channel management and production hardening/E2E.

## Product checkpoint

Stage 5 Script is creator-usable end to end. Provider-neutral visual image/video capability ports are also accepted. Scene Plan already has generation engine + persistence foundation; TASK-021 is converting it into the durable creator-facing backend Stage 7 capability.

Accepted foundations also include Media Asset S3-compatible storage and approved Scene Plan → primary visual Media Asset binding with replacement history. The current critical path remains Scene Plan durable generation/workspace, then Media Library/assignment and visual/audio acquisition.

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