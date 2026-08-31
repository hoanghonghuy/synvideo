# AI Proposal Generation V1 Contract

Status: FROZEN for `WAVE-F1-B`.

This contract isolates proposal-generation logic from Proposal persistence/API and from the frontend.

## Input
The generation engine receives already-authorized, already-loaded domain data:
- Project fields needed for creative context: `id`, `content_format`, `aspect_ratio`, `target_duration_seconds`, `locale`.
- The current Creative Brief including its `revision` and creator-authored fields from `CREATIVE_BRIEF_V1.md`.
- Stable `provider_id` and `model_id` selecting a text-generation implementation from the accepted provider boundary.

No API key/credential material is part of this request type.

## Output candidate
The engine returns a validated candidate containing exactly the **editable proposal content** fields defined by `AI_PROPOSAL_V1.md` plus:
- `source_brief_revision`: copied from the input Creative Brief revision.

It does **not** allocate Proposal `version`, persistence `revision`, status, timestamps or approval state.

## Prompt/output behavior
- Prompt/template has an explicit version identifier, initially `ai_proposal_v1`.
- Creator-provided facts/constraints must be clearly separated from requested AI recommendations in prompt construction.
- Project long/short format and duration intent must be preserved; do not force long-form projects into short-form structures.
- Provider output must be parsed into a strict structured candidate and validated against the frozen Proposal field rules before returning success.
- Do not persist raw provider payload as Proposal content.
- Do not silently accept malformed/partial JSON by inventing missing required fields in application code.
- Unknown extra fields may be rejected to keep schema drift visible.
- Research gaps/warnings remain explicit output fields rather than being hidden.

## Provider boundary
Use the accepted provider-neutral `providers.TextGenerator` / registry capability. No vendor SDK imports are allowed in the generation-engine package.

Context cancellation/deadline must propagate.

## Stable engine errors
At minimum distinguish:
- `GENERATION_PROVIDER_UNAVAILABLE` / provider resolution or availability failure;
- `GENERATION_PROVIDER_FAILED` / safe mapped execution failure;
- `GENERATION_INVALID_OUTPUT` / provider responded but content cannot satisfy the frozen structured contract;
- standard context cancellation/deadline behavior.

Presentation-safe messages must not expose raw secret-bearing provider output/errors.

## Deterministic verification
Tests use the accepted deterministic fake provider. No live network calls in CI.

Required contract behaviors include:
1. valid provider JSON produces a validated candidate matching the source Creative Brief revision;
2. prompt includes creator must-include/must-avoid facts and project format/duration context;
3. malformed JSON is rejected as `GENERATION_INVALID_OUTPUT`;
4. structurally valid JSON with invalid field cardinality/length is rejected;
5. provider failure maps to safe stable generation error;
6. context cancellation propagates;
7. input objects are not mutated;
8. no provider/model-specific raw schema leaks into Proposal candidate types.

## Integration after this wave
A later integration task owns:
- selecting/current Creative Brief under owner scope;
- calling this engine;
- persisting the candidate through Proposal `CreateDraft`;
- exposing the generation HTTP endpoint;
- frontend Generate/Regenerate actions and full end-to-end smoke.
