# Scene Plan Generation V1 Contract

Status: FROZEN for `WAVE-F1-E`.

This contract defines provider-neutral generation of an editable Scene Plan candidate from an already-approved Script, before any expensive media generation occurs.

## Product boundary
Scene Plan is Creative Workflow Stage 7. It transforms approved narration into ordered production scenes/segments and planning metadata.

This task does not persist Scene Plan versions, generate/acquire media, synthesize voice, render video, expose HTTP routes or modify frontend/editor state.

## Input
The generation engine receives already-authorized, already-loaded data:

### Project context
- `id`;
- `content_format`;
- `aspect_ratio`;
- `target_duration_seconds`;
- `locale`.

### Approved Script context
- `version`;
- `source_proposal_version`;
- ordered Script sections with stable `key`, optional heading and approved body text;
- optional Script `estimated_duration_seconds`;
- optional Script notes.

### Source Proposal production direction
The supplied Proposal version must match the Script's `source_proposal_version` and contributes production guidance only:
- `visual_direction`;
- `voice_direction`;
- `music_direction`;
- `caption_direction`;
- `warnings` and `research_gaps` where still relevant to planning.

### Provider selection
Stable `provider_id` and `model_id` for the accepted text-generation capability.

No credentials/API keys are part of the generation request type.

## Output candidate
The engine returns:
- `source_script_version`, copied from the input Script version;
- `source_proposal_version`, copied from the input Script/source Proposal relationship;
- ordered `scenes`.

No Scene Plan persistence version/revision/status/timestamps or approval fields are allocated by this engine.

## Scene shape
A candidate contains 1..500 scenes.

Each scene contains:
- `key`: required stable lowercase slug-like unique key, 1..64 characters;
- `script_section_key`: required Script section key and must exist in the input Script;
- `narration`: required approved narration segment, 1..20000 Unicode characters;
- `visual_instruction`: required production instruction, trimmed 1..5000 Unicode characters;
- `planned_source_type`: one of `stock`, `upload`, `creator_media`, `generated_image`, `generated_video`;
- `expected_duration_seconds`: required positive integer 1..3600;
- `caption_intent`: optional trimmed max 3000 characters;
- `transition_notes`: optional trimmed max 2000 characters.

Scene ordering is meaningful and preserved exactly as returned after validation.

## Approved Script preservation
Scene planning may split approved Script sections into multiple scenes, but it must not silently rewrite, add or omit approved narration.

Validation rule:
- scenes belonging to one Script section must be contiguous;
- every Script section must have at least one scene;
- scene groups must follow the original Script section order;
- after canonical whitespace normalization, concatenating all scene `narration` values for a Script section must equal the canonicalized approved Script section body exactly.

Canonical whitespace normalization collapses Unicode whitespace runs to a single ASCII space and trims the ends.

This permits segmentation/newline differences but prevents paraphrase/content drift after Script approval.

## Prompt behavior
Prompt/template version is explicitly `scene_plan_v1`.

The prompt must:
- identify Script text as approved narration that must be preserved rather than rewritten;
- preserve Project locale, aspect ratio, content format and duration intent;
- use Proposal production directions as visual/audio/caption planning context, not as permission to alter Script narration;
- allow long-form Scripts to produce many scenes without forcing short-video assumptions;
- request scene-level visual instructions and planned source types only, not actual media generation;
- surface warnings/research gaps so scene instructions do not invent unsupported factual visuals;
- explain the exact structured JSON output shape.

## Structured output
Provider output is strict JSON.

The engine must:
- reject malformed/trailing JSON;
- reject unknown fields where practical;
- validate scene/cardinality/enums/Unicode limits;
- validate Script section references/order/coverage and narration preservation;
- never repair missing required scenes or narration by inventing content in application code.

## Provider boundary
Use only the accepted provider-neutral `providers.TextGenerator` / registry capability. No vendor SDK import is allowed in the Scene Plan generation package.

Context cancellation/deadline during an in-flight provider call propagates unchanged.

## Stable errors
At minimum:
- `GENERATION_PROVIDER_UNAVAILABLE`;
- `GENERATION_PROVIDER_FAILED`;
- `GENERATION_INVALID_OUTPUT`;
- standard `context.Canceled` / `context.DeadlineExceeded`.

Presentation-safe errors must not expose raw provider output, raw secret-bearing errors or credentials.

## Input immutability
The engine treats Project, Script, Proposal and nested section/slice input as immutable snapshots. Tests must prove nested input data is unchanged after success and failure paths.

## Deterministic verification
CI uses the accepted deterministic fake provider only.

Required coverage:
1. valid scene JSON produces a validated candidate with exact source Script/Proposal versions;
2. prompt includes approved Script text, Project format/locale/aspect/duration and Proposal production direction;
3. long-form input is not constrained to short-form scene counts/pacing;
4. malformed/trailing/unknown JSON is rejected;
5. invalid scene keys, cardinality, enum, duration and Unicode limits are rejected;
6. unknown Script section references are rejected;
7. section order or non-contiguous grouping drift is rejected;
8. omitted/added/paraphrased narration fails canonical coverage validation;
9. provider resolution/execution failures map safely;
10. in-flight cancellation/deadline propagates;
11. Project/Script/Proposal inputs and nested slices are not mutated;
12. no provider-specific raw schema leaks into candidate types.

## Integration after this task
Later Scene Plan persistence/integration work will:
- load an approved Script and matching source Proposal under owner scope;
- invoke this engine through durable jobs where appropriate;
- persist/edit/version the Scene Plan;
- mark downstream assets stale when approved upstream Script changes;
- feed Scene-level media/audio generation and the Scene Editor.
