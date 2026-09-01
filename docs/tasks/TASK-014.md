# TASK-014 — Scene Plan generation engine

Status: READY
Milestone: F1 Creative Workflow
Depends on: TASK-005, TASK-011 and TASK-012 accepted; frozen `SCENE_PLAN_GENERATION_V1`
Wave: WAVE-F1-E
Branch: `feature/TASK-014-scene-plan-generation`
Base: `develop`

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
`docs/contracts/SCENE_PLAN_GENERATION_V1.md` is authoritative.

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
- [ ] Frozen Scene Plan generation contract implemented without drift.
- [ ] Approved Script narration cannot silently change at Scene Plan generation.
- [ ] Long-form, locale, aspect and production direction survive prompt construction.
- [ ] Strict validated candidate only; no application-side invention of missing scenes/narration.
- [ ] Provider-neutral capability only; no vendor SDK.
- [ ] Context/error/immutability regression coverage complete.
- [ ] gofmt/vet/race/tests/build/full CI green.
- [ ] No persistence/jobs/HTTP/frontend/media scope leakage.
- [ ] TDD evidence truthful.

## Follow-on
Later Scene Plan persistence/integration will own versioning/editing, durable generation orchestration, downstream stale propagation and Scene Editor/media handoff.

## Worktree / claim
Atomically claim the remote task branch, then work only in a dedicated TASK-014 worktree.

Do not self-merge or self-mark DONE.
