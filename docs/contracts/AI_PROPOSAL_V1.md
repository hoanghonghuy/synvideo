# AI Proposal V1 Contract

Status: FROZEN for `WAVE-F1-B`.

This contract lets Proposal persistence/API, generation-engine and frontend work proceed independently. Task branches may not silently change it; contract changes require PM/Team Lead coordination on `develop`.

## Product boundary
AI Proposal is editorial direction produced from the current Project + Creative Brief before script/media generation.

It is **not** a script, scene plan, media asset or render instruction set. Creator-authored Creative Brief facts remain separate and are referenced through `source_brief_revision` rather than copied back as creator facts.

## Version and approval invariants
- A Project may have many Proposal versions.
- `version` is a positive, monotonically increasing integer scoped to one Project.
- Each Proposal version has a mutable `revision` only while status is `draft`.
- Status values in V1: `draft`, `approved`, `superseded`.
- A Project has at most one active `draft` Proposal version at a time.
- An approved Proposal is immutable. Editing/regenerating after approval creates a new draft version; approval history must never be silently rewritten.
- Creating a newer draft **must** mark the previous active unapproved draft `superseded` in the same transaction; approved versions remain approved.
- Approval requires the caller's current `revision` and must be atomic with the status transition.
- Every Proposal records `source_brief_revision`. A later integration layer may report an approved/draft Proposal as upstream-stale when the Creative Brief revision has advanced; do not mutate the Proposal silently.

## Editable proposal content
The persisted/editor-visible content is:
- `title_options`: required array, 1..5 trimmed non-empty strings, each max 300 chars.
- `hook_options`: required array, 1..5 trimmed non-empty strings, each max 1000 chars.
- `audience_summary`: required trimmed string, 1..2000 chars.
- `objective_summary`: required trimmed string, 1..2000 chars.
- `narrative_angle`: required trimmed string, 1..4000 chars.
- `estimated_duration_seconds`: optional integer, 1..43200.
- `format_rationale`: optional trimmed string, max 2000 chars. This explains a recommendation; it does not replace Project `content_format`/`aspect_ratio`.
- `structure`: required ordered array, 1..50 items. Each item contains:
  - `key`: stable within this Proposal version, lowercase slug-like string, 1..64 chars;
  - `title`: trimmed string, 1..300 chars;
  - `purpose`: trimmed string, 1..2000 chars.
- `visual_direction`: optional trimmed string, max 5000 chars.
- `voice_direction`: optional trimmed string, max 3000 chars.
- `music_direction`: optional trimmed string, max 3000 chars.
- `caption_direction`: optional trimmed string, max 3000 chars.
- `call_to_action`: optional trimmed string, max 2000 chars.
- `research_gaps`: optional array, max 20 items; each trimmed non-empty string max 1000 chars.
- `warnings`: optional array, max 20 items; each trimmed non-empty string max 1000 chars.

Server-controlled fields:
- `project_id`: UUID.
- `version`: positive integer.
- `revision`: positive integer optimistic-concurrency token for a draft.
- `status`: `draft|approved|superseded`.
- `source_brief_revision`: positive integer.
- `created_at`, `updated_at`: UTC timestamps.
- `approved_at`: UTC timestamp or null.

Provider/model identifiers and token usage are generation provenance, not editorial content. They must not be required to edit/read/approve a Proposal.

## HTTP API owned by Proposal persistence task
All routes inherit owner/principal isolation from Project. Cross-owner existence must not be disclosed.

### GET `/api/v1/projects/{project_id}/creative-proposals`
Returns Proposal version summaries newest first. At minimum each item includes `version`, `revision`, `status`, `source_brief_revision`, `created_at`, `updated_at`, `approved_at`.

### GET `/api/v1/projects/{project_id}/creative-proposals/{version}`
Returns one full Proposal version.
- `200` when visible.
- `404` when project/proposal is not visible or version does not exist.

### PUT `/api/v1/projects/{project_id}/creative-proposals/{version}`
Replaces editable content of a **draft** version atomically.
- Body contains current `revision` plus all editable content fields.
- Success returns `200` and increments `revision` exactly once.
- Stale revision returns `409` code `STALE_REVISION`.
- Approved/superseded version mutation returns `409` code `PROPOSAL_IMMUTABLE`.
- Invalid content returns `400` standard validation envelope.

### POST `/api/v1/projects/{project_id}/creative-proposals/{version}/approve`
Body:
```json
{ "revision": 3 }
```
Semantics:
- Atomically approves the current draft revision and sets `approved_at`.
- Success returns `200` full approved Proposal.
- Stale revision: `409 STALE_REVISION`.
- Any non-draft version: `409 PROPOSAL_IMMUTABLE`.

## Internal create-draft service contract
Generation integration needs a service/repository operation to create a new Proposal draft from validated structured content plus `source_brief_revision`.

This operation is **internal application behavior in WAVE-F1-B**, not a public manual-create HTTP endpoint.

Creation semantics:
- allocate next Project-scoped `version` atomically;
- start `revision = 1`, `status = draft`;
- if an active draft exists, mark it `superseded` in the same transaction before the new draft becomes active;
- never supersede/modify an approved version;
- enforce at most one active draft per Project under concurrent creation;
- return the created full Proposal.

## Frontend behavior expected by this contract
- Show version history and status clearly.
- Show an empty/no-proposal state when none exists; generation CTA is added by the later integration task, not faked in TASK-008.
- Draft versions are editable and save with revision concurrency.
- Approved/superseded versions are read-only.
- Approval requires explicit user action and must not be inferred from navigation.
- Dirty/saving/saved/error/stale states remain distinguishable.
- Do not discard unsaved draft edits on recoverable failures.
- Switching versions while dirty must require explicit discard/reload behavior or be prevented until the user resolves dirty state.
- All visible copy through i18n.

## Explicitly outside this contract wave
- live vendor adapters/BYOK credentials;
- generation HTTP endpoint and frontend Generate/Regenerate CTA;
- targeted subsection AI revision;
- script generation;
- assets/media.

These are integrated after the three WAVE-F1-B tasks merge; Proposal is not declared product-complete until that integration passes.
