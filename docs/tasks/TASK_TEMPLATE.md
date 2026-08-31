# TASK-XXX — <title>

Status: BACKLOG
Milestone: <milestone>
Owner role: AI Developer
PR target: develop
Branch: `feature/TASK-XXX-<slug>`
Depends on: <task ids or none>

## Goal
What user/system outcome must exist after this task?

## Why
Why this task matters to the product/milestone.

## Read first
Only list documents/sections actually required for this task.
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `<exact doc path or heading>`

## Integration contract
List the frozen API/domain/event contract this task consumes or provides. Use `none` only when truly independent.

## Parallel safety
### Primary write paths
- `<owned path>`

### Allowed shared integration files
- `<small known hotspot or none>`

### Reserved / do not touch
- `<paths owned by parallel tasks or none>`

State merge/integration ordering if another task in the wave provides a runtime dependency.

## Scope
- ...

## Out of scope
- ...

## Required behavior
Describe observable behavior, states, validation, failure/retry semantics and persistence expectations.

## TDD plan
List the first meaningful RED tests/behaviors expected before implementation and the relevant integration/regression layers.

## Acceptance criteria
- [ ] ...
- [ ] ...

## Open-source research
- Relevant section: `<docs/research/...#section>` or `none`.
- Reuse decision must be recorded if source is adapted.

## Constraints
Architecture/security/i18n/performance constraints specific to this task.

## Verification
Commands/tests/E2E/manual scenarios required before PR.

## Delivery
PR must include:
- implementation summary;
- `TDD evidence` with RED/GREEN/REFACTOR notes;
- verification results;
- known limitations/assumptions;
- external code/license notes if any.
