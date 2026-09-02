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
| TASK-021 | Scene Plan durable generation + API integration | DONE | Issue #39 completed. PR #50 accepted head `3a78f24a...`, CI #242, TL review `5084672798`, squash `9d2b5306...`. |
| TASK-022 | Scene Plan creator workspace | CHANGES_REQUESTED | Issue #40 / PR #51. Head `1f5d00fc...`, CI #252, TL review `5085479067`. Fix partial-load truthfulness, resume ordering, Unicode/key-safe split, validation-error accessibility and Proposal lineage display. |
| TASK-023 | Media Library + Scene Binding API integration | READY | Issue #41. Backend media/runtime slot; range-read extension revalidated against accepted ObjectStorage; remote branch still absent at latest check. |
| TASK-024 | Media Library + scene assignment workspace | BACKLOG | Issue #42. Depends on accepted TASK-023 API and shares frontend router/locale surfaces with TASK-022. |
| TASK-025 | Provider-neutral visual generation foundation | DONE | Issue #43 completed. PR #49 accepted head `e090ed3e...`, CI #230, TL review `5080539748`, squash `1c550f316...`. |
| TASK-026 | Live OpenAI image generation adapter | READY | Issue #44. Current official API revalidated 2026-09-02; isolated provider-adapter surface; remote branch still absent at latest check. |
| TASK-027 | Provider-neutral TTS + OpenAI speech adapter foundation | BACKLOG | Issue #45. Dependency is satisfied and current speech endpoint remains available, but deliberately queued behind the 3-worktree cap; promote when a slot frees. |

## Current implementation / review slots

- **Dev A — TASK-022 `CHANGES_REQUESTED`**: continue only existing PR #51/worktree. Frontend-only Scene Plan workspace/router/locale surfaces.
- **Dev B — TASK-023 `READY`**: branch was still absent at latest check; may claim `feature/TASK-023-media-library-api` from latest `origin/develop`. Owns backend Media Library/Scene Binding HTTP + storage runtime/range-read composition.
- **Dev C — TASK-026 `READY`**: branch was still absent at latest check; may claim `feature/TASK-026-openai-image-provider` from latest `origin/develop`. Owns isolated `providers/openaiimage/**` adapter/tests only.

These lanes remain intentionally concurrent because their write surfaces are separated: frontend workspace vs backend media/runtime vs isolated provider adapter.

## Current review gate — TASK-022 / PR #51

Do not merge until:
1. failed generation-options / Project / plan-list / approved-Script context loads remain explicit retryable failures and are not disguised as valid empty/config states;
2. refresh resume cannot race with initial workspace loading and overwrite the exact `scene_plan_version` returned by a succeeded durable job;
3. split uses Unicode-safe boundaries and rejects invalid/duplicate/>64-rune scene keys before local mutation;
4. backend field validation errors are rendered/accessibly associated with the relevant scene controls, with safe fallback;
5. immutable `source_proposal_version` is visible alongside Script lineage;
6. deterministic regressions and fresh exact-head frontend CI are green.

## Why TASK-027 remains BACKLOG

TASK-027 is no longer dependency-blocked. It stays BACKLOG only to enforce the normal maximum of three implementation worktrees. Promote it when one current lane merges/releases a slot, provided no provider-core fix overlaps its minimal registry extension.

## Parallel safety

- TASK-022 and TASK-024 both use frontend router/locale/project-workspace integration; do not overlap them.
- TASK-023 owns shared backend `main.go` / `httpserver` composition; do not activate another backend integration task on those files concurrently.
- TASK-026 is isolated under `providers/openaiimage/**`; it must not modify main/http/runtime/settings/media/jobs/frontend.
- TASK-027 will require a minimum core providers TTS extension plus `providers/openaitts/**`; activate only when a worktree slot is free and no provider-core fix is active.
- Do not create micro-tasks merely to fill an implementation slot.

## Next activation path

1. Fix/review TASK-022 while TASK-023 and TASK-026 can start independently.
2. When TASK-022 merges, activate TASK-024 only if TASK-023 API is already accepted; otherwise use the freed slot for TASK-027.
3. When TASK-023 merges, TASK-024 becomes dependency-eligible after frontend hotspot release.
4. When TASK-026 or any current slot frees, promote TASK-027 TTS foundation.
5. Follow with secure multi-capability owner runtime/settings, durable per-scene visual/audio acquisition jobs, generated-output ingestion into Media Asset + Scene binding, captions/music, Scene Editor, render/export, publishing/channel management and production hardening/E2E.

## Product checkpoint

Stage 5 Script is creator-usable end to end. Stage 7 Scene Plan backend is complete; TASK-022 supplies the creator-facing workspace and is currently in review-fix. In parallel, TASK-023 turns accepted Media Asset + Scene Binding foundations into real creator APIs, while TASK-026 provides the first live image-generation adapter behind the accepted visual capability port.

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