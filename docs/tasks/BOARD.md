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
| TASK-018 | Script durable generation integration | CHANGES_REQUESTED | Issue #36 / PR #48. CI #207 green on reviewed head `95e958f7...`; TL review `5079858129`. Fix request-time locale persistence, strict durable snapshot validation, real PG same-job concurrency proof, frozen status shape; sync `develop`. |
| TASK-019 | Script creator workspace | CHANGES_REQUESTED | Issue #37 / PR #47. CI #206 green on reviewed head `33b0d14a...`; TL review `5079860958`. Fix UUID fallback, stale-revision reconcile UX and missing frozen regression coverage; sync `develop`. |
| TASK-020 | Scene media binding foundation | DONE | Issue #38 completed. PR #46 accepted head `3924f069...`, CI #205, TL review `5079847789`, squash `b80b8e7b...`. |
| TASK-021 | Scene Plan durable generation + API integration | BACKLOG | Issue #39. Contract `SCENE_PLAN_JOB_V1` frozen. Activate after TASK-018 releases backend runtime/jobs/httpserver hotspot and revalidate against accepted Script-job pattern. |
| TASK-022 | Scene Plan creator workspace | BACKLOG | Issue #40. Contract `SCENE_PLAN_WORKSPACE_V1` frozen. Activate after TASK-019 releases router/locale/navigation and TASK-021 API is stable. |
| TASK-023 | Media Library + Scene Binding API integration | BACKLOG | Issue #41. Contract `MEDIA_LIBRARY_API_V1` frozen. TASK-020 prerequisite is now satisfied; still wait for shared backend runtime/httpserver surface to be free. |
| TASK-024 | Media Library + scene assignment workspace | BACKLOG | Issue #42. Contract `MEDIA_LIBRARY_WORKSPACE_V1` frozen. Activate after TASK-023 API and frontend shared surface are available. |
| TASK-025 | Provider-neutral visual generation foundation | READY | Issue #43. Contract `VISUAL_GENERATION_PROVIDER_V1` frozen. Isolated `providers/**` image + async video ports/registry/fakes; no persistence/jobs/HTTP/runtime/media/frontend. Remote task branch was absent at READY promotion. |
| TASK-026 | Live OpenAI image generation adapter | BACKLOG | Issue #44. Contract `OPENAI_IMAGE_PROVIDER_V1` frozen. Depends on TASK-025; revalidate official Images API immediately before READY. Intentionally excludes deprecated Sora Video API. |
| TASK-027 | Provider-neutral TTS + OpenAI speech adapter foundation | BACKLOG | Issue #45. Contract `TTS_PROVIDER_V1` frozen. Depends on TASK-025; explicit input-too-long errors, never silent narration truncation. |

## Current implementation slots

- **Dev A — TASK-018 `CHANGES_REQUESTED`**: continue only PR #48 / existing worktree. Backend Script generation integration owns `scriptgenerationjob/**`, Script generation persistence, `0010`, Script generation HTTP and minimal runtime composition.
- **Dev B — TASK-019 `CHANGES_REQUESTED`**: continue only PR #47 / existing worktree. Frontend Script workspace owns `features/script/**` plus minimal router/locale/project navigation.
- **Dev C — TASK-025 `READY`**: new work may atomically claim `feature/TASK-025-visual-provider-foundation` from latest `origin/develop`; owns only provider-neutral visual capability/registry/fakes.

TASK-020 work is merged and no longer occupies an implementation slot. Its old worktree should be cleaned by the worker before taking another task.

## Current review gates

### TASK-018 / PR #48
Do not merge until all frozen `SCRIPT_JOB_V1` gates are satisfied:
1. generated Script persists the request-time Project locale snapshot even if Project locale changes after enqueue;
2. durable payload rejects trailing JSON and structurally invalid snapshot identities/enums before provider resolution;
3. real PostgreSQL concurrent same-generation-job persistence proves exactly one durable Script version;
4. public job status matches the frozen safe V1 field set;
5. branch is synced with latest `develop` and exact-head CI is green.

### TASK-019 / PR #47
Do not merge until all frozen `SCRIPT_WORKSPACE_V1` gates are satisfied:
1. every explicit generation/retry sends a fresh valid UUID, including environments without `crypto.randomUUID`;
2. stale revision preserves local edits and exposes an explicit confirmed reload/reconcile path;
3. missing high-risk deterministic tests are added: provider-empty guidance, read-only history, save/approval flows, terminal failure preservation, terminal Retry fresh ID, no-secret recovery persistence;
4. branch is synced with latest `develop` and exact-head CI is green.

## Parallel safety

TASK-018, TASK-019 and TASK-025 have disjoint primary write surfaces:
- TASK-018: backend jobs/runtime/httpserver + Script generation persistence;
- TASK-019: frontend Script workspace/router/locale;
- TASK-025: provider-neutral core visual interfaces/registry/fakes.

TASK-021 is intentionally not READY while TASK-018 owns `main.go` / jobs registry / `httpserver`. TASK-022 is intentionally not READY while TASK-019 owns router/locale/navigation. TASK-023 also waits for the backend composition surface even though its TASK-020 data prerequisite is now accepted. This avoids fake parallelism and low-value split tasks.

## Lookahead activation sequence

The next intended promotion sequence is dependency/write-surface driven rather than numeric-only:

1. When TASK-018 merges: revalidate/promote TASK-021 (Scene Plan durable generation/API). TASK-023 may also become backend-ready, but only one task may own shared runtime/httpserver at a time.
2. When TASK-019 merges: frontend shared surface is free; TASK-022 becomes candidate READY once TASK-021 API is stable. TASK-024 waits for TASK-023.
3. When TASK-025 merges: revalidate current live provider APIs, then TASK-026 (OpenAI image adapter) and TASK-027 (TTS foundation) can be scheduled on independent adapter/core surfaces as slots allow.
4. After Scene Plan + Media API foundations: add secure multi-capability provider runtime/settings, durable per-scene visual generation/acquisition jobs, and generated-output ingestion into Media Asset + Scene binding.
5. Then add per-scene voice/TTS orchestration with deterministic chunk/stitch/timing, stock acquisition, captions/music composition.
6. Then Scene Editor → render/export → Channel Hub/publishing → production hardening/E2E.

## Generative media architecture gates

- Provider capabilities already include text, image, video, TTS, transcription and music; do not introduce vendor-specific capabilities into core domain types.
- Image generation may be synchronous/streaming, but video generation must expose an async Start/Poll/OpenResult lifecycle with opaque external operation identity.
- Future paid video orchestration must persist the external provider operation ID before polling/resume so a worker crash cannot blindly resubmit and duplicate generation/cost.
- Provider output URLs are not durable SynVideo asset identity; accepted generated bytes must be ingested into `MediaAsset` storage with provenance.
- Generation and Scene binding remain separate operations so failed acquisition does not destroy the currently selected asset.
- Credentials/ciphertext/base URLs/raw upstream responses never enter durable jobs or public domain resources.
- TTS providers must return explicit input-limit errors; never silently truncate approved narration. Later orchestration owns deterministic chunk/stitch/timing.

See `docs/research/GENERATIVE_MEDIA_ARCHITECTURE_2026-09.md` and `docs/tasks/PLANNING_F1_I_J.md`.

## Product progress checkpoint

Accepted through TASK-020 includes:
- runnable Vue/Go/PostgreSQL foundation and CI/local infrastructure;
- Project + Creative Brief + AI Proposal end-to-end creator workflow;
- durable jobs and secure owner-scoped live text BYOK runtime;
- Script persistence/approval + provider-neutral generation engine;
- Scene Plan provider-neutral generation engine + durable versioned persistence;
- Media Asset metadata + S3-compatible object storage;
- approved Scene Plan → primary visual Media Asset binding with replacement history.

Current critical gap remains **Stage 5 Script end-to-end usability** until TASK-018 and TASK-019 are accepted. In parallel, TASK-025 starts the visual-provider foundation without colliding with those fixes.

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
