# SynVideo Task Board

Current milestone: `F1 — CREATIVE WORKFLOW`

PM plans ahead. AI Developers may start only tasks explicitly marked `READY`; review fixes stay on the original branch/PR. Parallel work follows `docs/engineering/PARALLEL_WORK_PROTOCOL.md` and normally uses at most 3 implementation worktrees.

| ID | Task | Status | Spec / Notes |
|---|---|---|---|
| BOOT-001 | Initialize repository and establish `develop` | DONE | Initial repository/control branch established. |
| AUDIT-001 | Audit pre-existing codebase | CANCELLED | Repository started from product scaffold. |
| PLAN-001 | Define Foundation milestone and first implementation task set | DONE | Architecture/dependency chain defined. |
| TASK-001 | Technical foundation and runnable project skeleton | DONE | Accepted PR #3. |
| TASK-002 | Project domain and persistence foundation | DONE | Accepted PR #8. |
| TASK-003 | Creative Brief backend and persistence | DONE | Accepted PR #9. |
| TASK-004 | Creative Brief frontend workspace | DONE | Accepted PR #10. |
| TASK-005 | AI provider capability and text-generation contracts | DONE | Accepted PR #11. |
| TASK-006 | AI Proposal domain, persistence and approval API | DONE | Accepted PR #17. |
| TASK-007 | AI Proposal generation engine | DONE | Accepted PR #16. |
| TASK-008 | AI Proposal frontend workspace | DONE | Accepted PR #20. |
| TASK-009 | AI Proposal generation job integration | DONE | Accepted PR #28; squash `fb8977aa...`. |
| TASK-010 | Durable job execution foundation | DONE | Accepted PR #21. |
| TASK-011 | Script domain, persistence and approval API | DONE | Accepted PR #22. |
| TASK-012 | Script generation engine | DONE | Accepted PR #24. |
| TASK-013 | Live OpenAI-compatible text provider adapter foundation | DONE | Accepted PR #29. |
| TASK-014 | Scene Plan generation engine | DONE | Accepted PR #27. |
| TASK-015 | Scene Plan domain and persistence foundation | DONE | Accepted PR #32; squash `66034b8e...`. |
| TASK-016 | Media Asset + S3-compatible storage foundation | DONE | Accepted PR #33; squash `a12a9856...`. |
| TASK-017 | Secure BYOK text provider settings and owner-scoped runtime | DONE | Accepted PR #35; squash `6fbfdbc0...`. |
| TASK-018 | Script durable generation integration | DONE | Issue #36 completed. PR #48 accepted head `c5e682d8...`, CI #215, TL review `5080177556`, squash `6bc3c86b...`. |
| TASK-019 | Script creator workspace | REVIEW | Issue #37 / PR #47. Functional delta logically approved head `5270f245...`, CI #216, TL review `5080179535`; rebase/sync latest `develop` after TASK-018 merge and rerun CI before merge. |
| TASK-020 | Scene media binding foundation | DONE | Issue #38 completed. PR #46 accepted head `3924f069...`, CI #205, TL review `5079847789`, squash `b80b8e7b...`. |
| TASK-021 | Scene Plan durable generation + API integration | BACKLOG | Issue #39. `SCENE_PLAN_JOB_V1` frozen. Backend hotspot is now released by TASK-018; revalidate/promote when an implementation slot is deliberately assigned. |
| TASK-022 | Scene Plan creator workspace | BACKLOG | Issue #40. `SCENE_PLAN_WORKSPACE_V1` frozen. Activate after TASK-019 releases frontend shared surface and TASK-021 API is accepted/stable. |
| TASK-023 | Media Library + Scene Binding API integration | BACKLOG | Issue #41. `MEDIA_LIBRARY_API_V1` frozen. TASK-020 prerequisite satisfied; compete with TASK-021 for shared backend runtime/httpserver slot, so do not run both on the same hotspot. |
| TASK-024 | Media Library + scene assignment workspace | BACKLOG | Issue #42. `MEDIA_LIBRARY_WORKSPACE_V1` frozen. Activate after TASK-023 API and frontend shared surface are available. |
| TASK-025 | Provider-neutral visual generation foundation | CHANGES_REQUESTED | Issue #43 / PR #49. Head `b0b33467...`, CI #217, TL review `5080185028`. Fix capability-specific result MIME validation, unconditional fake deep-copy of BinaryInput references, and frozen boundary tests. |
| TASK-026 | Live OpenAI image generation adapter | BACKLOG | Issue #44. `OPENAI_IMAGE_PROVIDER_V1` frozen. Depends on accepted TASK-025; revalidate official Images API immediately before READY. |
| TASK-027 | Provider-neutral TTS + OpenAI speech adapter foundation | BACKLOG | Issue #45. `TTS_PROVIDER_V1` frozen. Depends on accepted TASK-025; explicit input-too-long errors, never silent narration truncation. |

## Current implementation/review slots

- **Dev A — released after TASK-018 DONE.** Backend runtime/jobs/httpserver hotspot is free. TASK-021 is the preferred next critical-path backend candidate, but only promote/claim it deliberately after reviewing current `develop` and TASK-019 sync impact.
- **Dev B — TASK-019 `REVIEW`.** Continue only PR #47 / existing worktree. No more product behavior redesign is requested; sync/rebase latest `develop`, resolve genuine conflicts only, rerun CI and submit the new head for delta verification.
- **Dev C — TASK-025 `CHANGES_REQUESTED`.** Continue only PR #49 / existing worktree. Owns `providers/**` visual foundation fixes only.

One implementation slot is currently free because TASK-018 merged. Do not automatically fill it with a micro-task. The next substantial candidate is TASK-021 because it advances Stage 6 and now has its backend hotspot released.

## Current review gates

### TASK-019 / PR #47
Functional review is already accepted. Final merge gate only:
1. sync/rebase onto latest `develop` including accepted TASK-018 and PM/control-plane commits;
2. keep the already-approved Script workspace behavior intact;
3. exact new head is mergeable and CI green;
4. Team Lead verifies rebase/conflict delta before squash merge.

### TASK-025 / PR #49
Do not merge until:
1. Image generation outputs reject video MIME and video results reject image MIME;
2. fake request capture deep-copies any valid caller-provided `BinaryInput`, without relying on optional `BinaryInputCloner`;
3. deterministic tests cover wrong-family output MIME, reference MIME/size rejection, failed video operation/result-unavailable behavior and video cancellation;
4. providers-only ownership boundary remains intact;
5. exact-head CI is green.

## Parallel safety and activation

- TASK-021 and TASK-023 both need shared backend composition/httpserver work, so only one should own that hotspot at a time.
- TASK-022 and TASK-024 both need frontend router/locale/project workspace integration, so sequence them rather than creating fake parallelism.
- TASK-025 is isolated in provider-core visual abstractions; TASK-026/027 remain blocked on its acceptance.
- Prefer substantial creator/downstream capabilities over 1–2 line wiring tasks.

Recommended critical-path activation after current reviews:
1. finish TASK-019 final sync/merge;
2. fix/merge TASK-025;
3. use the free backend slot for TASK-021 Scene Plan durable generation + API integration;
4. once TASK-019 is merged and TASK-021 API stabilizes, TASK-022 becomes the next creator-facing Stage 6 workspace;
5. TASK-023/024 then expose Media Library + scene assignment before visual generation orchestration;
6. after TASK-025, revalidate and schedule TASK-026 image adapter / TASK-027 TTS on independent provider surfaces as slots permit.

## Generative media architecture gates

- Core provider capabilities stay provider-neutral; do not leak vendor schemas/SDK types into product domain contracts.
- Image generation may be synchronous/streaming; video generation must retain an async Start/Poll/OpenResult lifecycle with opaque external operation identity.
- Future paid video orchestration must persist external provider operation ID before polling/resume so worker crashes do not blindly resubmit and duplicate cost.
- Provider output URLs are not durable SynVideo asset identity; accepted generated bytes must be ingested into Media Asset storage with provenance.
- Generation and Scene binding remain separate so failed acquisition does not destroy the current selected asset.
- Credentials/ciphertext/base URLs/raw upstream responses never enter durable jobs or public domain resources.
- TTS input limits return explicit errors; later orchestration owns deterministic chunk/stitch/timing rather than silent truncation.

See `docs/research/GENERATIVE_MEDIA_ARCHITECTURE_2026-09.md` and `docs/tasks/PLANNING_F1_I_J.md`.

## Product progress checkpoint

Accepted through TASK-020 plus TASK-018 now includes:
- runnable Vue/Go/PostgreSQL foundation and CI/local infrastructure;
- Project + Creative Brief + AI Proposal end-to-end creator workflow;
- durable PostgreSQL jobs and secure owner-scoped live text BYOK runtime;
- Script persistence/approval, provider-neutral generation engine and durable live generation backend;
- Scene Plan provider-neutral generation engine + durable versioned persistence;
- Media Asset metadata + S3-compatible object storage;
- approved Scene Plan → primary visual Media Asset binding with replacement history.

The immediate creator-facing gap is now only TASK-019 final merge for complete Stage 5 Script usability. After that, critical path advances to Scene Plan durable generation/workspace, then Media Library/assignment, visual/audio acquisition, Scene Editor, render/export and publishing.

## Isolation / merge rules

- Every implementation task uses a dedicated Git worktree; shared/control checkout remains on `develop`.
- Maximum concurrent implementation worktrees normally equals 3.
- Review fixes stay on the original branch/PR/worktree.
- A READY branch is claimed by atomically creating the absent remote ref from latest `origin/develop`.
- Every task follows truthful RED → GREEN → REFACTOR.
- Team Lead review + exact-head green CI is the merge gate.
- Merged task worktrees are cleaned before another claim.
- Do not self-merge or self-mark DONE.

## Allowed statuses
`BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.