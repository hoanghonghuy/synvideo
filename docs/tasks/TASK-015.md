# TASK-015 — Scene Plan domain and persistence foundation

Status: READY
Milestone: F1 Creative Workflow
Depends on: TASK-011 and TASK-014 accepted; frozen `SCENE_PLAN_V1`
Wave: WAVE-F1-F
Branch: `feature/TASK-015-scene-plan-persistence`
Base: `develop`

## Goal
Implement the durable editable Scene Plan Stage 7 domain and PostgreSQL persistence foundation from approved Script versions, preserving approved narration while enabling scene-level planning/versioning before media generation.

## Authoritative contract
`docs/contracts/SCENE_PLAN_V1.md`

Read first:
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/contracts/SCRIPT_V1.md`
- `docs/contracts/SCENE_PLAN_GENERATION_V1.md`
- `docs/contracts/SCENE_PLAN_V1.md`
- accepted Script and Scene Plan generation implementations.

## Primary ownership
- `apps/api/internal/sceneplan/**`;
- Scene Plan PostgreSQL repository implementation;
- migration **`0007_create_scene_plans.sql`**;
- task-specific real PostgreSQL tests.

## Mandatory isolation
Do not touch:
- `apps/api/cmd/api/main.go`;
- `apps/api/internal/httpserver/**`;
- `apps/web/**`;
- generic jobs;
- Proposal generation integration / TASK-009 paths;
- `apps/api/internal/providers/**`;
- media/object storage / TASK-016 paths;
- render/publish code.

No public Scene Plan routes in this task.

## Scope
- Scene Plan domain types/status/version/revision/content validation;
- ordered scene validation with Unicode limits and source-type enum;
- approved Script narration coverage validation using canonical whitespace;
- owner-visible approved Script source enforcement;
- immutable source Script/Proposal/locale metadata;
- internal `CreateDraft`;
- list/get/update draft/approve service + repository operations;
- new draft atomically supersedes only prior active unapproved draft;
- approved/superseded immutable;
- stale revision protection;
- owner isolation/non-disclosure;
- real PostgreSQL concurrency behavior.

## TDD plan
Truthful RED → GREEN → REFACTOR must cover at least:
1. first draft from owner-visible approved Script;
2. reject unapproved/superseded/foreign Script source;
3. scene key/cardinality/source type/duration/Unicode validation;
4. exact approved narration coverage across segmentation;
5. reject omitted/added/paraphrased narration;
6. whitespace-only segmentation differences accepted;
7. second draft supersedes only active unapproved draft;
8. approved history immutable;
9. stale update/approval;
10. source metadata immutable;
11. concurrent draft creation leaves unique monotonic versions + one active draft;
12. concurrent same-revision update gives one success and stale competitors;
13. owner isolation in list/get/update/approve;
14. full `go test -race`, real PostgreSQL integration and `make verify`.

## Acceptance criteria
- [ ] Frozen `SCENE_PLAN_V1` implemented without drift.
- [ ] Migration is exactly `0007_create_scene_plans.sql`.
- [ ] Source must be an approved owner-visible Script.
- [ ] Scene Plan cannot silently rewrite approved Script narration.
- [ ] Version/revision/status invariants match accepted Proposal/Script conventions.
- [ ] Real PostgreSQL concurrency/owner-isolation coverage is green.
- [ ] No HTTP/frontend/jobs/media/provider-generation scope leakage.
- [ ] TDD evidence is truthful.

## Worktree
Atomically create the absent remote branch ref, then create/use a dedicated TASK-015 worktree. The shared control checkout stays on `develop`.

Do not self-merge or self-mark DONE.