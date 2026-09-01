# TASK-012 — Script generation engine

Status: DONE
Milestone: F1 Creative Workflow
Depends on: TASK-005 accepted; frozen `SCRIPT_V1` + `SCRIPT_GENERATION_V1`
Wave: WAVE-F1-D
Branch: `feature/TASK-012-script-generation`
Base: `develop`
Accepted PR: #24
Squash merge: `0a3d2fb9ae82eed4b2520cf4e05377bd63e07c9d`

## Acceptance record
Team Lead logical APPROVE on head `2883cfcc` accepted the implementation after verifying frozen `SCRIPT_GENERATION_V1` behavior, isolated `scriptgeneration/**` scope, strict structured output validation, Unicode rune limits, long-form preservation, safe provider-neutral error mapping, in-flight context propagation, input immutability and CI #128 green.

Formal GitHub APPROVE is unavailable because PR author/reviewer are the same account; review #5073033845 records the project Team Lead logical approval.

## Goal
Implement the provider-neutral Script generation engine defined by `docs/contracts/SCRIPT_GENERATION_V1.md`: transform an already-approved Proposal + Project writing context into a strict validated Script candidate, without persistence, jobs, HTTP or frontend work.

This task is intentionally independent from TASK-011's persistence branch. It develops against frozen contracts and mirrors the accepted proposal-generation isolation pattern.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/contracts/SCRIPT_V1.md`
- `docs/contracts/SCRIPT_GENERATION_V1.md`
- `docs/contracts/AI_PROPOSAL_V1.md`
- `docs/product/CREATIVE_WORKFLOW.md`
- accepted `apps/api/internal/providers/**`
- accepted `apps/api/internal/proposalgeneration/**` as architecture reference only.

## Frozen contract
`SCRIPT_GENERATION_V1.md` is authoritative. Do not change it from this branch.

## Primary write paths
- `apps/api/internal/scriptgeneration/**`
- task-specific tests only.

## Reserved / do not touch
- `apps/api/internal/script/**`, Script PostgreSQL repository, migration `0005`, Script HTTP routes — TASK-011.
- `apps/api/internal/jobs/**` / migration `0004` — accepted TASK-010.
- Proposal generation/persistence behavior.
- `apps/web/**`.
- provider registry core or vendor adapters unless a genuine frozen-boundary defect is discovered and escalated instead of silently changed.
- Proposal/Script generation HTTP/job integration.

## Scope
- Script-generation input snapshot types for Project writing context + approved Proposal editorial context + provider/model selection.
- Candidate types containing exactly Script editable fields plus `source_proposal_version`.
- Prompt builder with explicit `script_v1` prompt version.
- Clear approved-editorial-context vs generation-instruction separation.
- Preserve Project locale, content format and duration intent.
- Preserve long-form capability.
- Explicit research-gap/warning constraints; no silent factual invention instruction.
- Strict structured JSON parsing with unknown-field rejection where practical.
- Validation matching frozen Script section/cardinality/key/Unicode character rules.
- Provider-neutral registry/TextGenerator resolution and call.
- Safe stable error mapping.
- In-flight context cancellation/deadline propagation.
- Input/nested-slice immutability.

## Out of scope
- loading Project/Proposal from repositories;
- checking owner visibility at persistence boundaries;
- Script `CreateDraft` persistence;
- jobs enqueue/worker registration;
- HTTP Generate/Regenerate endpoints;
- live provider/BYOK configuration;
- frontend Script editor/generation UI;
- Scene Plan generation.

## Important invariants
- Candidate `source_proposal_version` is copied from input, never model-controlled.
- Provider output cannot set Script version/revision/status/content_locale/timestamps/approval.
- Invalid or partial provider JSON is never repaired by inventing missing required content in application code.
- Long-form Projects are not squeezed into short-video structure.
- Unicode limits use rune/character semantics.
- Raw provider errors/output are not exposed through presentation-safe engine errors.
- Cancellation/deadline during provider execution remains a context error, not `GENERATION_PROVIDER_FAILED`.

## TDD coverage
1. valid provider JSON -> validated Script candidate + copied source Proposal version;
2. prompt contains approved Proposal context, Project locale/format/duration;
3. prompt includes research gaps/warnings as non-invention constraints;
4. long-form input is not forced to short-form;
5. malformed/trailing JSON -> `GENERATION_INVALID_OUTPUT`;
6. missing/empty sections, duplicate/invalid keys, length/cardinality violations -> invalid output;
7. multibyte Unicode exactly-at-limit passes, one-character-over fails;
8. provider unavailable/failure -> safe stable errors;
9. in-flight cancel and deadline propagate `context.Canceled` / `context.DeadlineExceeded`;
10. input Project/Proposal and nested arrays/structure are not mutated;
11. candidate/request types do not leak provider-specific raw schemas.

## Acceptance criteria
- [x] Frozen `SCRIPT_GENERATION_V1` behavior is implemented without contract changes.
- [x] Engine uses accepted provider-neutral `TextGenerator` capability only.
- [x] Structured output is strict and validated against Script V1 rules.
- [x] Long-form and locale intent are preserved in prompt behavior.
- [x] Research gaps/warnings are explicit constraints against silent invention.
- [x] Context cancellation/deadline propagate during in-flight generation.
- [x] Input immutability is regression-tested.
- [x] No persistence/jobs/HTTP/frontend/provider-SDK scope leakage.
- [x] TDD evidence truthful; gofmt/vet/tests/build/full CI green.

## Integration
After TASK-011 persistence is accepted, a later Script-generation job integration task can combine TASK-010 durable jobs + TASK-012 engine + Script `CreateDraft`. This task itself stays persistence-free.

## Worktree / claim
The dedicated TASK-012 worktree should be cleaned up after merge before its developer claims another task.
