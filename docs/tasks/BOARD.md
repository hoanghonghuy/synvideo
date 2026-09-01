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
| TASK-015 | Scene Plan domain and persistence foundation | CHANGES_REQUESTED | Issue #30 / PR #32. Core implementation + CI green; add missing owner/foreign-source/whitespace TDD gates from Team Lead review. |
| TASK-016 | Media Asset + S3-compatible storage foundation | CHANGES_REQUESTED | Issue #31 / PR #33. Fix owner↔project persistence invariant, object-key identity binding, and cross-owner Open/Delete regression. |
| TASK-017 | Secure BYOK text provider settings and owner-scoped runtime | READY | Issue #34. Frozen `BYOK_TEXT_PROVIDER_RUNTIME_V1`; migration `0009`; provider settings/runtime + Proposal live-provider integration + settings UI. |

## Active parallel wave — WAVE-F1-G

Frozen contracts relevant to active work:
- `docs/contracts/SCENE_PLAN_V1.md`;
- `docs/contracts/MEDIA_ASSET_STORAGE_V1.md`;
- `docs/contracts/BYOK_TEXT_PROVIDER_RUNTIME_V1.md`;
- accepted `AI_PROPOSAL_JOB_V1` and `OPENAI_COMPAT_TEXT_PROVIDER_V1`.

Current implementation slots:
- **Dev A — TASK-017 `READY`**: atomically claim `feature/TASK-017-byok-provider-runtime` from latest `origin/develop`, then implement only the frozen BYOK/runtime task.
- **Dev B — TASK-015 `CHANGES_REQUESTED`**: continue only on PR #32/worktree and fix current Team Lead review gates.
- **Dev C — TASK-016 `CHANGES_REQUESTED`**: continue only on PR #33/worktree and fix current Team Lead review gates.

## Why WAVE-F1-G is safe
TASK-015 remains isolated to Scene Plan domain/PostgreSQL/migration `0007`. TASK-016 remains isolated to media/storage/PostgreSQL/migration `0008`. TASK-017 owns the now-released runtime/httpserver/provider-settings/frontend surface and migration `0009`. The migrations do not depend on each other's newly-created tables.

TASK-017 is intentionally deployment-definition based: creators configure credentials/model enablement but cannot submit arbitrary base URLs in V1. This closes the real live-AI usability gap without introducing an unreviewed SSRF/custom-endpoint surface.

## Current review gates

### TASK-015 / PR #32
1. Real PostgreSQL cross-owner `UpdateDraft` and `Approve` non-disclosure regressions.
2. True foreign-owner Script source rejection for `CreateDraft`.
3. Explicit whitespace-only narration segmentation acceptance regression.

### TASK-016 / PR #33
1. Persisted media owner must match the actual Project owner.
2. Canonical object-key project/asset UUIDs must equal `MediaAsset.ProjectID` / `MediaAsset.ID`, not merely parse as UUIDs.
3. Foreign owner/project `Open` and `Delete` must fail before any object-store call.

## Isolation / merge rules
- Every implementation task uses a dedicated Git worktree; the shared/control checkout remains on `develop`.
- Maximum concurrent implementation worktrees normally equals 3.
- Review fixes stay on their existing branch/PR/worktree.
- TASK-015 must not touch `main.go`, `httpserver/**`, `apps/web/**`, jobs, providers or media/storage.
- TASK-016 must not touch `main.go`, `httpserver/**`, `apps/web/**`, jobs, Proposal/Script/Scene Plan packages or AI text providers.
- TASK-017 must not touch TASK-015 `sceneplan/**`/`0007` or TASK-016 `mediaasset/**`/`0008`, and must not start Script/Scene Plan/media/render/publish follow-ons.
- New task branches must be claimed by atomically creating the absent remote ref at latest `origin/develop`; a plain same-base push is not an exclusive lock.
- Every task follows `docs/engineering/TDD_PROTOCOL.md`; RED → GREEN → REFACTOR evidence must be truthful.
- Merged worktrees should be cleaned before the developer claims another task.
- Do not self-merge or self-mark DONE; Team Lead review + green CI is the merge gate.

## Planned follow-on — not READY yet
These are the next product capabilities, but shared write surfaces/dependencies must be released and contracts frozen before implementation:

1. **TASK-018 candidate — Script durable generation integration**: generic jobs + TASK-012 engine + idempotent Script draft persistence using the same owner-scoped provider runtime from TASK-017.
2. **Script creator workspace**: Script history/edit/stale/approval plus durable Generate/Regenerate states/recovery.
3. **Scene Plan durable generation/API/workspace**: integrate TASK-014 engine with accepted TASK-015 persistence and owner-scoped live provider runtime.
4. **Scene-level media acquisition/generation**: build on accepted TASK-015 + TASK-016, with per-scene replacement/retry and durable asset relationships.
5. **Voice/audio generation and timing**: provider-neutral TTS/audio assets, narration alignment and replaceable per-scene audio.
6. **Scene Editor**: creator controls for media, crop/fit, captions, transitions, timing and regenerated/replaced assets without losing approved history.
7. **Render/export pipeline**: durable render jobs, deterministic composition, progress/retry, downloadable outputs and render version history.
8. **Publishing/channel management**: secure channel credentials, YouTube/TikTok/etc. publishing jobs, scheduling, retry/idempotency and publication history.

Do not mark follow-ons READY merely to fill slots. Prefer one substantial, contract-frozen task per released write surface.

## Product progress checkpoint
Accepted capabilities now cover:
- runnable Vue/Go/PostgreSQL foundation and CI/local infrastructure;
- Project persistence/owner boundary;
- Creative Brief persistence and creator workspace;
- provider-neutral text-generation contracts;
- AI Proposal persistence/versioning/approval, generation engine, creator workspace and durable generation jobs;
- generic durable PostgreSQL jobs/lease/retry execution;
- Script persistence/versioning/approval and provider-neutral generation engine;
- live OpenAI-compatible provider adapter foundation;
- provider-neutral Scene Plan generation engine.

Current wave advances three gaps in parallel:
- finish durable Scene Plan persistence/versioning;
- finish durable media/object-storage foundation;
- make live AI credentials/provider runtime safely creator-configurable.

After TASK-017, AI Proposal is live-provider usable rather than merely provider-ready. The remaining F1 path is still substantial: Script durable generation/workspace, Scene Plan workspace, media/audio acquisition, Scene Editor, render/export and publishing.

## Allowed statuses
`BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.
