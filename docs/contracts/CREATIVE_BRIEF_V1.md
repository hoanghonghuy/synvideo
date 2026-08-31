# Creative Brief V1 Contract

Status: FROZEN for the first parallel Creative Brief implementation wave.

This contract lets backend and frontend implement independently. A task may not silently change it in its branch; contract changes require PM/Team Lead coordination on `develop`.

## Product boundary
Creative Brief V1 persists **creator-supplied intent only**. It does not contain AI recommendations, generated proposals, scripts, media, or approval state.

Project-level format fields (`content_format`, `aspect_ratio`, `target_duration_seconds`, `locale`) remain on Project. Do not duplicate them into Creative Brief V1.

## Resource
One Creative Brief exists at most once per Project.

Fields:
- `project_id`: UUID, response only.
- `revision`: positive integer used for optimistic concurrency; response only except as the concurrency token on updates.
- `source_text`: required creator free-text intent, trimmed, 1..20000 characters.
- `target_audience`: optional string, trimmed, max 2000.
- `objective`: optional string, trimmed, max 2000.
- `desired_style`: optional string, trimmed, max 2000.
- `tone`: optional string, trimmed, max 500.
- `distribution_targets`: optional unique array, max 8 values; allowed V1 values are `youtube`, `tiktok`, `instagram`, `other`. These are creator distribution intents, not provider adapter identifiers.
- `call_to_action`: optional string, trimmed, max 2000.
- `must_include`: optional array, max 20 items; each item trimmed, non-empty, max 500 characters.
- `must_avoid`: optional array, max 20 items; each item trimmed, non-empty, max 500 characters.
- `created_at`: UTC timestamp, response only.
- `updated_at`: UTC timestamp, response only.

All fields above are explicitly creator-authored in V1. Future AI-inferred values belong to a proposal/provenance model rather than being mixed into this resource.

## HTTP API
All routes are under `/api/v1` and inherit TASK-002 owner/principal isolation.

### GET `/projects/{project_id}/creative-brief`
Returns the current owner-scoped Creative Brief.

- `200` with the resource when it exists.
- `404` when the project is not visible to the owner or no Creative Brief exists. Do not reveal cross-owner existence.

### PUT `/projects/{project_id}/creative-brief`
Creates the first draft or replaces the editable V1 fields atomically.

Create body:
```json
{
  "source_text": "...",
  "target_audience": "...",
  "objective": "...",
  "desired_style": "...",
  "tone": "...",
  "distribution_targets": ["youtube"],
  "call_to_action": "...",
  "must_include": ["..."],
  "must_avoid": ["..."]
}
```

Update body uses the same fields and additionally supplies the current `revision`:
```json
{
  "revision": 3,
  "source_text": "..."
}
```

Semantics:
- first successful create returns `201` and `revision = 1`;
- successful replacement of an existing brief returns `200` and increments `revision` exactly once;
- updating an existing brief without the current revision is rejected;
- stale revision returns `409` with stable error code `STALE_REVISION`;
- invalid fields return `400` using the repository's standard validation envelope;
- project not visible to the current owner returns `404`;
- request bodies must not accept `project_id`, owner identity, timestamps, or server-controlled revision values other than the update concurrency token.

The backend may reject unknown JSON fields if that matches the accepted API convention; frontend must not depend on sending unknown fields.

## Persistence constraints
- One row/resource per project.
- Deleting/archiving project behavior is not redefined here.
- Owner isolation is enforced through the owner-scoped project relationship/service boundary; no client-supplied owner identifier.
- Revision increment and update must be atomic so two concurrent stale editors cannot silently overwrite each other.

## Frontend behavior expected by the contract
- Load existing brief or show a new-draft state when the project exists but no brief exists.
- Preserve form input through recoverable validation/network errors.
- Show saving and saved state.
- Track the latest returned `revision` after successful save.
- On `STALE_REVISION`, do not blindly retry/overwrite. Show a localized conflict state with an explicit reload action for V1.
- No AI-generation CTA is activated by saving this resource.

## Contract verification
Backend contract tests and frontend client/feature tests must use these exact field names/status semantics. Final frontend acceptance includes a smoke/integration pass against the merged backend implementation.
