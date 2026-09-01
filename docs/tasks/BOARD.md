# SynVideo Task Board

Current milestone: `F1 — CREATIVE WORKFLOW`

PM plans ahead. AI Developers may start only tasks explicitly marked `READY`. Review fixes stay on the original branch/PR. Parallel tasks must follow `docs/engineering/PARALLEL_WORK_PROTOCOL.md`.

| ID | Task | Status | Spec / Notes |
|---|---|---|---|
| BOOT-001 | Initialize repository and establish `develop` | DONE | Initial repository/control branch established. |
| AUDIT-001 | Audit pre-existing codebase | CANCELLED | Not applicable: repository started from product scaffold. |
| PLAN-001 | Define Foundation milestone and first implementation task set | DONE | Initial architecture/dependency chain defined. |
| TASK-001 | Technical foundation and runnable project skeleton | DONE | Accepted and squash-merged PR #3. |
| TASK-002 | Project domain and persistence foundation | DONE | Accepted and squash-merged PR #8. |
| TASK-003 | Creative Brief backend and persistence | DONE | Accepted and squash-merged PR #9. |
| TASK-004 | Creative Brief frontend workspace | DONE | Accepted and squash-merged PR #10. |
| TASK-005 | AI provider capability and text-generation contracts | DONE | Accepted and squash-merged PR #11. |
| TASK-006 | AI Proposal domain, persistence and approval API | DONE | Accepted and squash-merged PR #17. |
| TASK-007 | AI Proposal generation engine | DONE | Accepted and squash-merged PR #16. |
| TASK-008 | AI Proposal frontend workspace | DONE | Accepted and squash-merged PR #20. |
| TASK-009 | AI Proposal generation job integration | DONE | PR #28 accepted after delta review and squash-merged as `fb8977aa...`; issue #15 closed. |
| TASK-010 | Durable job execution foundation | DONE | Accepted and squash-merged PR #21. |
| TASK-011 | Script domain, persistence and approval API | DONE | Accepted and squash-merged PR #22. |
| TASK-012 | Script generation engine | DONE | Accepted and squash-merged PR #24. |
| TASK-013 | Live OpenAI-compatible text provider adapter foundation | DONE | Accepted and squash-merged PR #29. |
| TASK-014 | Scene Plan generation engine | DONE | Accepted and squash-merged PR #27. |
| TASK-015 | Scene Plan domain and persistence foundation | DONE | PR #32 accepted on head `306d9dae...`, CI #168 green, squash-merged as `66034b8e...`; issue #30 closed. |
| TASK-016 | Media Asset + S3-compatible storage foundation | DONE | PR #33 accepted on head `7141c02b...`, CI #167 green, squash-merged as `a12a9856...`; issue #31 closed. |
| TASK-017 | Secure BYOK text provider settings and owner-scoped runtime | DONE | PR #35 accepted on head `8dd10cae...`, CI #177 green, logical review `5078306924`, squash-merged as `6fbfdbc0...`; issue #34 closed. |
| TASK-018 | Script durable generation integration | READY | Issue #36. Frozen `SCRIPT_JOB_V1`. Backend durable jobs + owner runtime + DB-idempotent Script persistence + HTTP/runtime. |
| TASK-019 | Script creator workspace | READY | Issue #37. Frozen `SCRIPT_WORKSPACE_V1`. Frontend-only Stage 5 history/edit/approve/generate/recovery workspace. |
| TASK-020 | Scene media binding foundation | READY | Issue #38. Frozen `SCENE_MEDIA_BINDING_V1`. Durable approved-scene primary visual assignment + replacement history. |

## Active parallel wave — WAVE-F1-H

Frozen contracts:
- `docs/contracts/SCRIPT_JOB_V1.md`;
- `docs/contracts/SCRIPT_WORKSPACE_V1.md`;
- `docs/contracts/SCENE_MEDIA_BINDING_V1.md`;
- accepted `SCRIPT_V1`, `SCRIPT_GENERATION_V1`, `JOB_EXECUTION_V1`, `BYOK_TEXT_PROVIDER_RUNTIME_V1`, `SCENE_PLAN_V1`, `MEDIA_ASSET_STORAGE_V1`.

Current implementation slots:
- **Dev A — TASK-018 `READY`**: `feature/TASK-018-script-generation-integration`; backend-only Script durable generation integration.
- **Dev B — TASK-019 `READY`**: `feature/TASK-019-script-workspace`; frontend-only Script creator workspace, implementing against frozen TASK-018 API contract.
- **Dev C — TASK-020 `READY`**: `feature/TASK-020-scene-media-binding`; isolated scene/media domain + PostgreSQL foundation.

## Why this wave is parallel-safe and still substantial
TASK-018 owns the shared backend runtime/job/httpserver hotspot and migration `0010`; it does not touch web or scene/media work.

TASK-019 owns the Script frontend surface plus the one router/locale integration slot; it does not touch backend code. It can build against the frozen Script-generation API while TASK-018 is still in progress.

TASK-020 owns a new `scenemedia/**` domain/repository surface and migration `0011`; it deliberately does not touch `main.go`, `httpserver/**`, jobs or `apps/web/**`.

This means the three tasks advance real product capabilities without creating three developers who all edit the same composition/router files. None is a micro-task:
- TASK-018 closes the missing live/durable backend half of Stage 5;
- TASK-019 closes the missing creator-facing half of Stage 5;
- TASK-020 creates the history-safe asset-selection model required by Stage 8 and the Scene Editor.

## Isolation / merge rules
- Every implementation task uses a dedicated Git worktree; shared/control checkout remains on `develop`.
- Maximum concurrent implementation worktrees normally equals 3.
- Review fixes stay on the original branch/PR/worktree.
- New task branches are claimed by atomically creating the absent remote ref from latest `origin/develop`; a plain same-base push is not an exclusive lock.
- Every task follows `docs/engineering/TDD_PROTOCOL.md`; RED → GREEN → REFACTOR evidence must be truthful.
- Merged worktrees are cleaned before another claim.
- Do not self-merge or self-mark DONE; Team Lead review + exact-head green CI is the merge gate.

## Why Scene Plan integration is not in this wave
The next obvious capability is durable Scene Plan generation + API/workspace, but assigning it concurrently now would create avoidable merge hotspots:
- its backend integration needs the same `main.go` / jobs registry / `httpserver/server.go` surface reserved by TASK-018;
- its frontend workspace needs the same router/locale/project-navigation surface reserved by TASK-019.

It is therefore intentionally sequenced immediately after those hotspots are released rather than split into low-value fragments.

## Planned product path after WAVE-F1-H — not READY yet
Freeze each capability only when its write surface is released.

1. **TASK-021 candidate — Scene Plan durable generation + API integration**: TASK-014 engine + TASK-015 persistence + generic jobs + owner-scoped runtime, with DB-idempotent generation and feature-specific endpoints.
2. **TASK-022 candidate — Scene Plan creator workspace**: history/edit/approve/staleness + durable Generate/Regenerate and recovery.
3. **Media library/upload + scene binding API/workspace**: expose accepted Media Asset storage and TASK-020 bindings safely to creators; upload/list/preview/delete-in-use semantics and scene replacement UI.
4. **Visual acquisition/generation pipeline**: provider-neutral stock/generated-image/generated-video capabilities, durable per-scene jobs, provenance, retry/replacement and ingestion into Media Asset storage.
5. **Voice/audio generation and timing**: provider-neutral TTS/audio generation, narration timing/alignment, replaceable voice assets and audio metadata.
6. **Captions/music composition model**: caption tracks/style intent and music assets/levels/timing represented independently from rendered output.
7. **Scene Editor**: editable draft that composes approved Scene Plan + selected media + voice/captions/music; crop/fit/timing/transitions/replacement/regeneration without destroying upstream history.
8. **Render/export pipeline**: deterministic composition snapshot, durable render jobs, progress/retry, output asset/version history and downloadable exports.
9. **Publishing/channel management**: secure channel credentials, YouTube/TikTok/etc. publication jobs, scheduling, idempotency/retry and publication history.
10. **Production hardening / creator completeness**: richer source intake (URLs/files/research inputs), project-level workflow status, observability/quotas, cleanup/retention, deployment documentation and end-to-end product regression suite.

Do not mark these READY merely to fill slots. Prefer cohesive capabilities that a creator or a downstream stage can actually use.

## Product progress checkpoint
Accepted through TASK-017:
- runnable Vue/Go/PostgreSQL foundation and CI/local infrastructure;
- Project persistence/owner boundary;
- Creative Brief persistence and creator workspace with raw source text intake;
- provider-neutral text-generation contracts;
- AI Proposal persistence/versioning/approval, generation engine, creator workspace and durable live-provider generation;
- generic durable PostgreSQL jobs/lease/retry execution;
- Script persistence/versioning/approval and provider-neutral generation engine;
- live OpenAI-compatible provider adapter plus secure owner-scoped BYOK settings/runtime;
- Scene Plan provider-neutral generation engine plus durable versioned persistence;
- durable Media Asset metadata and S3-compatible object storage.

The largest current gap is that Stage 5 Script is not yet creator-usable end-to-end; WAVE-F1-H closes that while preparing Stage 8 binding semantics. After H, the critical path is Scene Plan creator workflow → media/audio acquisition → Scene Editor → render/export → publishing.

## Allowed statuses
`BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.
