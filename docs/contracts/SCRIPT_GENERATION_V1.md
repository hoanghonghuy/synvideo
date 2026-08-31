# Script Generation V1 Contract

Status: FROZEN for `WAVE-F1-D`.

This contract isolates provider-neutral Script candidate generation from Script persistence/API, durable job orchestration and the frontend.

## Product boundary
Script generation transforms an already-approved AI Proposal into editable Script text before Script approval and Scene Plan work.

The engine does not persist Script versions, approve Scripts, enqueue jobs, expose HTTP routes or call vendor-specific SDKs.

## Input
The generation engine receives already-authorized, already-loaded domain data:
- Project context needed for writing: `id`, `content_format`, `aspect_ratio`, `target_duration_seconds`, `locale`.
- One **approved** Proposal version and its editorial content from `AI_PROPOSAL_V1.md`, including:
  - `version`;
  - title/hook options;
  - audience/objective summaries;
  - narrative angle;
  - estimated duration and format rationale;
  - ordered structure;
  - visual/voice/music/caption direction;
  - CTA;
  - research gaps and warnings.
- Stable `provider_id` and `model_id` selecting the accepted provider-neutral text-generation capability.

The engine treats the supplied Proposal as the approved editorial source. It does not silently fetch or blend a newer Creative Brief revision.

No credentials/API keys are part of the request type.

## Output candidate
The engine returns a validated candidate containing exactly the editable Script content defined by `SCRIPT_V1.md` plus:
- `source_proposal_version`: copied from the input approved Proposal version.

Editable candidate fields:
- `sections`: ordered 1..200 items with stable lowercase slug-like unique `key`, optional `heading` max 300 characters and required `body` 1..20000 characters;
- `estimated_duration_seconds`: optional 1..43200;
- `notes`: optional max 10000 characters.

The candidate does **not** allocate Script `version`, persistence `revision`, status, `content_locale`, timestamps or approval state.

## Prompt/output behavior
- Prompt/template has an explicit version identifier, initially `script_v1`.
- The prompt must clearly identify the Proposal as creator-approved editorial direction, not raw provider instructions.
- Preserve Project content locale and format/duration intent.
- Long-form Projects must remain long-form capable; do not force a short-video section count or per-section brevity rule beyond the frozen Script limits.
- Proposal structure/order should guide Script section organization without requiring one-to-one copying when a coherent Script needs multiple sections.
- Proposal `research_gaps` and `warnings` must be surfaced as constraints. The model must not silently invent factual answers to unresolved research gaps.
- Provider output must be strict structured JSON parsed into the frozen candidate shape and validated before success.
- Do not silently fill missing required Script sections/body in application code.
- Unknown extra fields may be rejected so schema drift remains visible.
- Character limits use Unicode character/rune semantics consistent with `SCRIPT_V1`.

## Provider boundary
Use the accepted provider-neutral `providers.TextGenerator` / registry capability. No provider SDK imports are allowed in the Script generation package.

Context cancellation/deadline must propagate, including cancellation/deadline that occurs while the provider call is in flight.

## Stable engine errors
At minimum distinguish:
- `GENERATION_PROVIDER_UNAVAILABLE` — provider/model cannot be resolved or is unavailable;
- `GENERATION_PROVIDER_FAILED` — safe mapped provider execution failure;
- `GENERATION_INVALID_OUTPUT` — provider returned content that cannot satisfy the frozen structured Script candidate contract;
- standard `context.Canceled` / `context.DeadlineExceeded` propagation.

Presentation-safe errors must not expose raw secret-bearing provider errors or raw provider output.

## Deterministic verification
CI uses the accepted deterministic fake provider; no live provider/network calls.

Required behavior includes:
1. valid provider JSON produces a validated candidate with the exact `source_proposal_version`;
2. prompt includes approved Proposal editorial context, Project locale/format/duration and explicit research-gap/warning constraints;
3. long-form input is not forced into short-form wording/structure;
4. malformed JSON is rejected as `GENERATION_INVALID_OUTPUT`;
5. structurally valid JSON violating section/cardinality/key/Unicode character limits is rejected;
6. provider resolution/execution failure maps to safe stable errors;
7. cancellation/deadline during an in-flight provider call propagates as context errors;
8. input Project/Proposal objects and nested slices are not mutated;
9. no provider/model-specific raw schema leaks into candidate types.

## Integration after this task
A later Script generation integration task owns:
- owner-scoped loading of an approved Proposal and Project;
- durable execution through the generic jobs foundation;
- invoking this engine;
- persisting a validated candidate through Script `CreateDraft`;
- feature-specific generation HTTP routes and frontend Generate/Regenerate actions.

A successful generation job must create at most one Script draft and must not mutate an approved Script version.
