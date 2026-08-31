# Script V1 Contract

Status: FROZEN for the next F1 parallel wave.

Script is the durable, editable text/narration artifact created from an approved AI Proposal before Scene Plan generation.

## Product boundary
- A Script belongs to one Project.
- A Script version references exactly one approved Proposal version through `source_proposal_version`.
- A Script is not a Scene Plan, media asset, caption track or render instruction set.
- Long-form Projects must remain supported; do not force one short-video script shape.

## Version and approval invariants
- A Project may have many Script versions.
- `version` is a positive monotonically increasing integer scoped to one Project.
- Each version has mutable `revision` only while `status = draft`.
- V1 statuses: `draft|approved|superseded`.
- A Project has at most one active draft Script at a time.
- New draft creation supersedes only the previous active unapproved draft in the same transaction.
- Approved/superseded versions are immutable.
- Approval requires the caller's current `revision`, is atomic and records `approved_at`.
- Editing/regenerating after approval creates a new draft version; approved history is never rewritten.
- Script creation requires an owner-visible **approved** source Proposal version. Draft/superseded Proposal versions are not valid Script sources.

## Editable Script content
Persist/edit:
- `sections`: required ordered array, 1..200 items.
  - `key`: required lowercase slug-like stable key within the Script version, 1..64 chars, unique.
  - `heading`: optional trimmed string, max 300 chars.
  - `body`: required trimmed string, 1..20000 chars.
- `estimated_duration_seconds`: optional integer, 1..43200.
- `notes`: optional trimmed string, max 10000 chars.

The section model is intentionally neutral to short/long form. Section count and body limits must permit long-form scripts.

## Server-controlled fields
- `project_id`: UUID.
- `version`: positive Project-scoped integer.
- `revision`: positive optimistic concurrency token.
- `status`: `draft|approved|superseded`.
- `source_proposal_version`: positive integer.
- `content_locale`: copied from the Project content locale when the Script draft is created and preserved for that version.
- `created_at`, `updated_at`: UTC timestamps.
- `approved_at`: UTC timestamp or null.

Provider/model identifiers and token usage are generation provenance, not Script editorial content.

## HTTP API owned by Script persistence task
All routes inherit owner/principal isolation and cross-owner non-disclosure.

### GET `/api/v1/projects/{project_id}/scripts`
Returns version summaries newest first. At minimum: `version`, `revision`, `status`, `source_proposal_version`, `content_locale`, timestamps.

### GET `/api/v1/projects/{project_id}/scripts/{version}`
Returns one full Script version. Invisible/missing project or version returns `404`.

### PUT `/api/v1/projects/{project_id}/scripts/{version}`
Replaces all editable fields of a draft.
- Body contains current `revision` plus all editable fields.
- Success `200`, revision increments exactly once.
- stale revision -> `409 STALE_REVISION`.
- approved/superseded -> `409 SCRIPT_IMMUTABLE`.
- invalid content -> `400` validation envelope.

### POST `/api/v1/projects/{project_id}/scripts/{version}/approve`
Body: `{ "revision": <positive integer> }`.
- current draft revision -> `200` full approved Script and durable `approved_at`.
- stale revision -> `409 STALE_REVISION`.
- any non-draft -> `409 SCRIPT_IMMUTABLE`.

## Internal create-draft contract
Generation integration later uses an internal `CreateDraft` operation; no public manual-create HTTP endpoint in this task.

CreateDraft must:
- verify owner-visible Project;
- verify `source_proposal_version` exists and is `approved`;
- allocate the next Script version atomically;
- copy current Project content locale into `content_locale`;
- start `revision = 1`, `status = draft`;
- supersede a previous active Script draft atomically;
- preserve all approved Script versions;
- enforce at most one active draft under concurrent creation.

## Downstream staleness
Scene Plan work later references an approved Script version. If a newer Script version is approved, existing downstream Scene Plans may be marked stale by a later integration task; do not silently mutate them from this contract.

## Verification expectations
Required coverage includes domain validation, owner isolation, approved-source enforcement, version allocation, concurrent draft creation, stale update/approval, immutable approved history, newest-first listing, real PostgreSQL constraints/migrations and HTTP error semantics.
