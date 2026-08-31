# TASK-007 — AI Proposal generation engine

Status: READY
Milestone: F1 Creative Workflow
Depends on: TASK-003 and TASK-005 accepted
Wave: WAVE-F1-B
Branch: `feature/TASK-007-ai-proposal-generation`
Base: `develop`

## Goal
Implement a provider-neutral application engine that transforms accepted Project + current Creative Brief context into a strictly validated AI Proposal candidate using the existing text-generation provider boundary.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/contracts/AI_PROPOSAL_GENERATION_V1.md`
- `docs/contracts/AI_PROPOSAL_V1.md`
- `docs/contracts/CREATIVE_BRIEF_V1.md`
- `docs/product/CREATIVE_WORKFLOW.md`
- accepted `apps/api/internal/providers/**`

## Contract
`AI_PROPOSAL_GENERATION_V1.md` and `AI_PROPOSAL_V1.md` are frozen. Do not change them from this branch.

This task does not persist Proposal versions and does not add HTTP routes. TASK-009 owns integration after this wave.

## Primary write paths
- `apps/api/internal/proposalgeneration/**`
- generation-engine-specific tests under the same package.

## Allowed shared integration files
None by default.

## Reserved / do not touch
- `apps/api/internal/creativeproposal/**`, Proposal migration/repository/API — TASK-006.
- `apps/api/internal/httpserver/**` and `apps/api/cmd/api/main.go`.
- `apps/web/**` — TASK-008.
- provider registry/error implementation unless a genuine blocker is escalated to Team Lead.

## Scope
- Versioned prompt/template identity `ai_proposal_v1`.
- Input composed from accepted Project + Creative Brief data and selected provider/model ids.
- Clear prompt separation between creator facts/constraints and requested AI recommendations.
- Call through provider-neutral text-generation capability only.
- Strict structured response parsing into a Proposal candidate matching frozen editable fields.
- Contract validation before success.
- Safe stable generation error categories.
- Preserve Creative Brief revision as `source_brief_revision` in candidate.
- Long/short format and duration context preserved in prompt behavior.
- Deterministic fake-provider tests; no live network.

## Out of scope
- Persistence/version allocation/approval.
- HTTP generation endpoint.
- Frontend Generate/Regenerate button.
- Live OpenAI/Gemini/etc adapters or credentials.
- Targeted subsection revision/regeneration.
- Script/scenes/media.

## TDD plan
Start RED for at least:
1. valid structured provider output -> validated candidate with source brief revision;
2. prompt contains creator must-include/must-avoid constraints and project duration/format context;
3. malformed JSON -> `GENERATION_INVALID_OUTPUT`;
4. structurally valid but contract-invalid candidate -> `GENERATION_INVALID_OUTPUT`;
5. provider unavailable/failure -> stable safe generation error without raw-secret leakage;
6. context cancellation/deadline propagates;
7. source Project/Creative Brief inputs are not mutated;
8. long-form duration/content format is not silently converted to short-form assumptions.

## Acceptance criteria
- [ ] Engine depends only on accepted provider-neutral interfaces/types, not vendor SDKs.
- [ ] Output matches frozen Proposal editable schema and carries source brief revision.
- [ ] Invalid/malformed model output never becomes a persisted-looking successful candidate.
- [ ] Prompt behavior distinguishes creator facts from AI recommendations.
- [ ] Stable safe errors and cancellation behavior are tested.
- [ ] No persistence/router/frontend/shared composition edits.
- [ ] TDD evidence truthful; `-race`/backend/full verification green where applicable.

## Verification
At minimum:
- targeted generation tests;
- `go test -count=1 -race ./internal/proposalgeneration/...`;
- `gofmt`, `go vet ./...`, `go test ./...`, backend build;
- full repository verification and `git diff --check`.

## Merge order
Merge-order independent from TASK-006 and TASK-008. TASK-009 consumes the accepted engine after this task merges.

Do not self-merge or self-mark DONE.
