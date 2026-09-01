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
| TASK-017 | Secure BYOK text provider settings and owner-scoped runtime | CHANGES_REQUESTED | Issue #34 / PR #35. Review fixes pushed on head `488467e0...`; awaiting Team Lead delta re-review after the prior six security/runtime/TDD gates. |

## Active parallel wave — WAVE-F1-G

Frozen contracts relevant to active work:
- `docs/contracts/BYOK_TEXT_PROVIDER_RUNTIME_V1.md`;
- accepted `AI_PROPOSAL_JOB_V1`, `OPENAI_COMPAT_TEXT_PROVIDER_V1`, `SCENE_PLAN_V1`, and `MEDIA_ASSET_STORAGE_V1`.

Current implementation slots:
- **Dev A — TASK-017 `CHANGES_REQUESTED`**: continue only on PR #35/worktree; fix commit `488467e0...` is pushed and awaits Team Lead delta review.
- **Dev B — free**: TASK-015 is merged/DONE; cleanup its worktree before claiming a new task.
- **Dev C — free**: TASK-016 is merged/DONE; cleanup its worktree before claiming a new task.

## Why current isolation remains safe
TASK-015 and TASK-016 are now accepted and no longer occupy implementation write surfaces. TASK-017 owns the runtime/httpserver/provider-settings/frontend surface plus migration `0009` and must not reopen Scene Plan/media foundations.

TASK-017 remains intentionally deployment-definition based: creators configure credentials/model enablement but cannot submit arbitrary base URLs in V1. This closes the real live-AI usability gap without introducing an unreviewed SSRF/custom-endpoint surface.

## TASK-017 current review state
Previous Team Lead review on head `94d49abf...` requested:
1. strict authenticated/owner-scoped text-generation options with no global fallback;
2. exact API-key input preservation so whitespace mistakes are rejected server-side;
3. reuse of accepted TASK-013 OpenAI-compatible base-URL validation;
4. real PostgreSQL same-revision concurrent update coverage;
5. end-to-end Proposal job smoke proving owner credential isolation and secret-free durable payload/result;
6. first-create revision must be omitted.

Fix commit `488467e0...` claims all six are addressed. Team Lead must review that delta and CI before TASK-017 can merge.

## Isolation / merge rules
- Every implementation task uses a dedicated Git worktree; the shared/control checkout remains on `develop`.
- Maximum concurrent implementation worktrees normally equals 3.
- Review fixes stay on their existing branch/PR/worktree.
- TASK-017 must not modify accepted TASK-015 `sceneplan/**`/`0007` or TASK-016 `mediaasset/**`/`0008`, and must not start Script/Scene Plan/media/render/publish follow-ons.
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
- provider-neutral Scene Plan generation engine plus durable Scene Plan persistence/versioning;
- durable media metadata and S3-compatible object-storage foundation.

Current active gap is secure creator-configurable live-provider runtime in TASK-017. After TASK-017, AI Proposal becomes live-provider usable rather than merely provider-ready. The remaining F1 path is still substantial: Script durable generation/workspace, Scene Plan workspace, media/audio acquisition, Scene Editor, render/export and publishing.

## Allowed statuses
`BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.
