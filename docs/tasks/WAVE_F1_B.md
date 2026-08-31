# WAVE-F1-B — AI Proposal foundation

Status: OPEN
Milestone: F1 Creative Workflow

## Product outcome
Advance Stage 3–4 from accepted Creative Brief into a durable, reviewable and approvable AI Proposal without coupling persistence/UI to any vendor provider.

The wave intentionally separates three implementation surfaces behind frozen contracts, then uses TASK-009 as the integration gate. Completion of individual wave tasks does **not** declare AI Proposal product-complete.

## Frozen contracts
- `docs/contracts/AI_PROPOSAL_V1.md`
- `docs/contracts/AI_PROPOSAL_GENERATION_V1.md`

## Parallel tasks
| Task | Owner surface | Runtime dependency | Merge rule |
|---|---|---|---|
| TASK-006 | Proposal domain/Postgres/API | accepted Project + Creative Brief | independent; merge when accepted |
| TASK-007 | Proposal generation engine | accepted provider boundary + Project/Brief types | independent; merge when accepted |
| TASK-008 | Proposal frontend workspace | frozen HTTP/resource contract | develop with mocks; final real smoke after TASK-006 merges |

## Conflict control
- TASK-006 owns `creativeproposal`, Proposal migration/repository/HTTP and minimal backend composition.
- TASK-007 owns only `proposalgeneration` and does not touch router, DB or frontend.
- TASK-008 owns `apps/web` Proposal feature plus minimal route/i18n/navigation hotspots and does not touch backend.
- Contract files are PM-owned and frozen during the wave.

## Integration gate
TASK-009 remains BLOCKED until TASK-006/007/008 are accepted. It is the only task allowed to connect generation engine -> Proposal CreateDraft -> generation HTTP endpoint -> frontend Generate/Regenerate action.

## TDD
Every task follows `docs/engineering/TDD_PROTOCOL.md`; truthful RED -> GREEN -> REFACTOR evidence is required. No coverage-only test may be represented as a behavioral RED failure.

## Completion rule
WAVE-F1-B foundation is complete when TASK-006/007/008 are merged. Stage 3–4 AI Proposal is complete only after TASK-009 is also accepted with real end-to-end smoke.
