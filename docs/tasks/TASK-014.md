# TASK-014 — Scene Plan generation engine

Status: DONE
Milestone: F1 Creative Workflow
Depends on: TASK-005, TASK-011 and TASK-012 accepted; frozen `SCENE_PLAN_GENERATION_V1`
Wave: WAVE-F1-E
Branch: `feature/TASK-014-scene-plan-generation`
Base: `develop`
Accepted PR: #27
Squash merge: `6b5c9d3c76681addd1dcb1791a93c62c41770158`

## Acceptance record
Team Lead logical APPROVE on head `acb8cc21` with CI #140 green.

Accepted behavior:
- isolated `apps/api/internal/sceneplangeneration/**` scope;
- strict `scene_plan_v1` provider-neutral structured generation;
- Project locale/format/aspect/duration and Proposal production directions preserved;
- scene cardinality/key/source-type/duration/Unicode validation;
- Script section reference/order/contiguity validation;
- approved Script narration may be segmented but cannot be silently rewritten/added/omitted because canonicalized scene coverage must equal the approved section body;
- long-form support, safe provider errors, in-flight context cancellation/deadline and deep input immutability;
- no persistence/jobs/HTTP/frontend/media scope leakage.

## Goal
Implement provider-neutral Scene Plan candidate generation from an already-approved Script plus matching source Proposal production direction, before any media generation occurs.

This is a substantial downstream capability but remains isolated from persistence/HTTP/frontend so it can run safely in parallel with TASK-009 and TASK-013.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/product/CREATIVE_WORKFLOW.md`
- `docs/contracts/SCRIPT_V1.md`
- `docs/contracts/SCRIPT_GENERATION_V1.md`
- `docs/contracts/SCENE_PLAN_GENERATION_V1.md`
- accepted `providers.TextGenerator` and Script/Proposal domain conventions.

## Frozen contract
`SCENE_PLAN_GENERATION_V1.md` is authoritative.

## Primary ownership
- `apps/api/internal/sceneplangeneration/**`;
- task-specific tests only.

## Reserved / do not touch
- `apps/api/cmd/api/main.go`;
- `apps/api/internal/httpserver/**`;
- `apps/web/**`;
- jobs/persistence/migrations;
- Proposal generation integration — TASK-009;
- OpenAI-compatible provider adapter — TASK-013;
- Script persistence/generation behavior;
- actual asset/media/audio generation.

## Scope
- immutable Project/approved Script/source Proposal generation input snapshots;
- Scene Plan candidate types from frozen contract;
- explicit `scene_plan_v1` prompt;
- preserve approved Script narration exactly across scene segmentation using canonical whitespace coverage validation;
- section-reference/order/contiguity validation;
- scene keys/source-type/duration/Unicode validation;
- Project locale/format/aspect/duration preservation;
- Proposal visual/voice/music/caption direction plus warnings/research gaps as planning constraints;
- strict structured JSON parse with unknown-field rejection;
- provider-neutral registry/TextGenerator use only;
- safe generation error mapping;
- in-flight context cancellation/deadline propagation;
- deep input immutability.

## Important product invariant
Scene Plan may segment approved narration but must not become a hidden Script-rewrite step.

For every approved Script section, normalized concatenation of its contiguous output scene narration must equal the normalized approved Script body. Added, omitted or paraphrased narration is `GENERATION_INVALID_OUTPUT`.

## TDD plan
Truthful RED -> GREEN -> REFACTOR covers at least:
1. valid multi-scene output with exact source Script/Proposal versions;
2. prompt contains approved narration and production directions;
3. long-form Script preserves long-form capability;
4. malformed/trailing/unknown JSON rejected;
5. invalid cardinality/key/source type/duration/Unicode boundaries rejected;
6. unknown Script section key rejected;
7. section reorder/non-contiguous grouping rejected;
8. missing narration coverage rejected;
9. added/paraphrased narration rejected;
10. whitespace-only segmentation differences accepted under canonical normalization;
11. safe provider unavailable/execution mapping;
12. in-flight cancel/deadline propagation;
13. Project/Script/Proposal nested input immutability;
14. no provider-specific raw schema leakage.

## Acceptance criteria
- [x] Frozen Scene Plan generation contract implemented without drift.
- [x] Approved Script narration cannot silently change at Scene Plan generation.
- [x] Long-form, locale, aspect and production direction survive prompt construction.
- [x] Strict validated candidate only; no application-side invention of missing scenes/narration.
- [x] Provider-neutral capability only; no vendor SDK.
- [x] Context/error/immutability regression coverage complete.
- [x] gofmt/vet/race/tests/build/full CI green.
- [x] No persistence/jobs/HTTP/frontend/media scope leakage.
- [x] TDD evidence truthful.

## Follow-on
Later Scene Plan persistence/integration will own versioning/editing, durable generation orchestration, downstream stale propagation and Scene Editor/media handoff.

## Worktree cleanup
The TASK-014 implementation worktree should be removed after confirming the merged commit before this developer claims another task.
