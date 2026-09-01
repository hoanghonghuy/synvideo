# Scene Plan V1 Contract

Status: FROZEN for the next Creative Workflow implementation wave.

This contract defines the durable editable Stage 7 Scene Plan resource built from an approved Script and the accepted `SCENE_PLAN_GENERATION_V1` candidate shape.

## Product boundary
Scene Plan sits after Script approval and before expensive media/audio acquisition.

V1 persists and versions scene planning only. It does not generate/fetch media, synthesize voice, render video, publish, or expose Scene Editor asset manipulation.

## Version model
A Project may have many Scene Plan versions.

Each version has:
- `project_id`;
- positive monotonic `version`;
- positive optimistic-concurrency `revision`;
- `status`: `draft | approved | superseded`;
- `source_script_version`;
- `source_proposal_version` copied from the approved Script relationship;
- `content_locale` copied from Project at draft creation;
- ordered editable `scenes`;
- timestamps and optional `approved_at`.

Invariants:
- at most one active `draft` per Project;
- a newly-created draft atomically supersedes only the previous active unapproved draft;
- approved versions are preserved and immutable;
- superseded versions are immutable;
- approval requires the current revision and is atomic;
- editing/regenerating after approval creates a new draft version rather than mutating approved history.

## Source requirements
Internal `CreateDraft` requires an owner-visible **approved Script** version.

The stored `source_script_version` is that approved Script version. The stored `source_proposal_version` must equal the Script's accepted source Proposal relationship.

Draft creation fails when:
- Project/Script is not visible to the owner;
- source Script does not exist or is not `approved`;
- Script/source Proposal relationship is invalid.

Scene Plan source links are immutable after creation.

A later upstream Script approval does not silently mutate an existing Scene Plan. Downstream integration may derive/report that an older Scene Plan is stale by comparing `source_script_version` with the current approved Script.

## Scene shape
A Scene Plan contains 1..500 ordered scenes.

Each scene contains:
- `key`: required unique lowercase slug-like key, 1..64 characters;
- `script_section_key`: required key from the source approved Script;
- `narration`: required approved narration segment, 1..20000 Unicode characters;
- `visual_instruction`: trimmed required 1..5000 Unicode characters;
- `planned_source_type`: `stock | upload | creator_media | generated_image | generated_video`;
- `expected_duration_seconds`: integer 1..3600;
- `caption_intent`: optional trimmed max 3000 Unicode characters;
- `transition_notes`: optional trimmed max 2000 Unicode characters.

Scene order is meaningful.

## Approved Script preservation
Scene Plan editing may change segmentation and planning metadata, but V1 must not become a hidden Script rewrite step.

For the source approved Script:
- every Script section must have one or more scenes;
- scenes for one section are contiguous;
- section groups follow approved Script order;
- after canonical whitespace normalization, concatenated scene narration for each section must equal the approved Script section body exactly.

Create/update validation therefore requires access to the immutable source approved Script. Added, omitted or paraphrased narration is rejected.

Canonical whitespace follows `SCENE_PLAN_GENERATION_V1`: collapse Unicode whitespace runs to one ASCII space and trim ends.

## Persistence operations
Repository/service boundary supports:
- list versions newest-first under owner/project scope;
- get one version under owner/project scope;
- internal `CreateDraft` from an approved Script + validated Scene content;
- replace editable draft content using current `revision`;
- approve a current draft using current `revision`.

Errors follow existing resource conventions:
- non-disclosing not-found for inaccessible resources;
- stale update/approval -> stable stale-revision error;
- non-draft mutation -> immutable error;
- invalid source Script -> stable source-not-approved/source-invalid error;
- invalid scene fields/coverage -> validation error.

## Concurrency
PostgreSQL behavior must prove:
- concurrent `CreateDraft` calls allocate unique monotonic versions;
- at most one active draft remains;
- approved history remains untouched;
- concurrent updates using the same revision allow one success and reject stale competitors;
- approval is revision-checked atomically.

## Migration
TASK-015 owns migration `0007_create_scene_plans.sql` and Scene Plan persistence tables/indexes only.

The migration must not depend on TASK-009's Proposal-generation migration order beyond the existing accepted base schema; migration runner filename tracking allows independent merge order.

## Public HTTP
No new HTTP routes are required in TASK-015. A later Scene Plan integration/workspace task will expose API/UI after this domain/persistence foundation is accepted.

## TDD gates
Required real PostgreSQL/domain coverage includes:
1. create first draft from approved Script;
2. reject draft/superseded/non-owner Script sources;
3. validate Scene shape and Unicode limits;
4. reject narration add/omit/paraphrase and accept whitespace-only segmentation differences;
5. second draft supersedes only active unapproved draft;
6. approved Scene Plan remains immutable;
7. stale revision update/approval;
8. owner isolation;
9. concurrent draft creation and update behavior;
10. source Script/Proposal/locale metadata remains immutable and correct.