# AI Proposal Generation Job V1 Contract

Status: FROZEN for `WAVE-F1-E`.

This contract connects the accepted Proposal generation engine, durable jobs foundation, Proposal persistence and creator-facing Proposal workspace without turning provider execution into a blocking HTTP request.

## Product boundary
A generation request captures the creator's current Project + Creative Brief intent, runs asynchronously, persists at most one new Proposal draft for that durable job, and exposes safe progress/result state to the creator.

The feature reuses `JOB_EXECUTION_V1`; it does not create a second queue/lease table or a second worker lifecycle.

Production code must never register or expose the deterministic fake provider as a live creator option.

## Stable job identity and HTTP idempotency
The client supplies a UUID `request_id` for each explicit Generate/Regenerate intent.

That `request_id` is also the durable `jobs.id`.

- Repeating the same `request_id` for the same owner/project/provider/model returns the already-created generation job rather than enqueuing another job.
- Reusing a `request_id` with conflicting selection data returns `409 GENERATION_REQUEST_CONFLICT`.
- A fresh Generate/Regenerate action uses a fresh `request_id`.
- The server owns owner/project identity from the authenticated route context; the request body never contains owner identity.

This HTTP idempotency prevents duplicate jobs after ambiguous network retries. It is separate from the Proposal-persistence idempotency requirement below.

## Provider catalog
`GET /api/v1/ai/text-generation-options`

Returns only registered models supporting the accepted text-generation capability. Response shape:

```json
{
  "providers": [
    {
      "id": "provider-id",
      "display_name": "Provider",
      "models": [
        {
          "id": "model-id",
          "display_name": "Model"
        }
      ]
    }
  ]
}
```

Requirements:
- deterministic ordering;
- no credentials, base URLs, secret metadata or raw provider configuration;
- providers without a text-generation model are omitted;
- an empty array is valid and means no production text provider is configured;
- fake/test registrations may exist only in test composition.

## Create generation request
`POST /api/v1/projects/{project_id}/creative-proposal-generations`

Body:

```json
{
  "request_id": "uuid",
  "provider_id": "provider-id",
  "model_id": "model-id"
}
```

Success is `202 Accepted` and returns the capability-specific job view described below.

Before enqueue the service must:
1. verify the Project is visible to the current principal;
2. load the current Creative Brief under the same owner scope;
3. resolve the selected text-generation capability through the accepted provider registry;
4. snapshot the Project + Creative Brief fields required by `AI_PROPOSAL_GENERATION_V1`;
5. enqueue one generic durable job.

Stable pre-enqueue failures:
- invisible Project: existing non-disclosing `404` behavior;
- no current Creative Brief: `409 CREATIVE_BRIEF_REQUIRED`;
- invalid/unregistered provider/model/capability selection: `400 GENERATION_PROVIDER_UNAVAILABLE`;
- conflicting reuse of `request_id`: `409 GENERATION_REQUEST_CONFLICT`.

No provider network call occurs in this HTTP request.

## Generic job envelope
Stable job kind:

`creative_proposal_generation_v1`

Generic durable fields:
- `id = request_id`;
- owner from principal;
- project from route;
- `max_attempts = 3` in V1;
- optional generic dedupe key may mirror the request ID but correctness must not depend on the dedupe key alone.

The generic job payload is a JSON object with schema version `ai_proposal_generation_job_v1` and contains a request-time snapshot:

```json
{
  "schema_version": "ai_proposal_generation_job_v1",
  "provider_id": "provider-id",
  "model_id": "model-id",
  "project": {
    "id": "uuid",
    "content_format": "short",
    "aspect_ratio": "9:16",
    "target_duration_seconds": 60,
    "locale": "vi"
  },
  "brief": {
    "revision": 3,
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
}
```

Why snapshot instead of reloading "current" data in the worker:
- a queued job must keep the exact intent that was accepted when the creator clicked Generate;
- later Creative Brief edits do not silently change an already-enqueued generation;
- `source_brief_revision` remains deterministic.

Credentials/API keys/tokens are forbidden in the durable payload.

## Handler behavior
The registered handler:
1. strictly decodes the versioned payload;
2. invokes the accepted `proposalgeneration.Engine` using the snapshotted Project/Brief plus provider/model IDs;
3. persists only a fully validated candidate;
4. returns a safe JSON result object only after idempotent Proposal persistence succeeds.

No raw provider output is persisted as Proposal content or generic job result.

## Exactly-once Proposal persistence per durable job
A successful durable job must create **at most one** Proposal version even if the worker crashes or loses its lease after database commit but before generic `MarkSuccess`.

V1 persistence rule:
- migration `0006` adds nullable internal `source_generation_job_id uuid` metadata to `creative_proposals`;
- it references the durable job where practical and has a uniqueness constraint/index for non-null values;
- this field is internal persistence metadata and is not added to public `AI_PROPOSAL_V1` responses;
- generation persistence uses an idempotent internal operation keyed by `(owner, project, job_id)`;
- first call creates the draft using the accepted Proposal version/supersede transaction and stores `source_generation_job_id` atomically;
- a retry with the same job ID returns the already-created Proposal rather than allocating another version;
- a different job ID is a genuinely new Generate/Regenerate request and may create a new draft.

The database uniqueness guarantee is mandatory; in-memory checks alone are insufficient.

This specifically covers the failure window:
`CreateDraft COMMIT -> worker crash/lease loss -> job reclaimed -> handler runs again`.

## Generic job result
On success the generic `jobs.result` object is:

```json
{
  "proposal_version": 4
}
```

The Proposal version is server-derived from idempotent persistence, never provider-controlled.

## Capability-specific status API
`GET /api/v1/projects/{project_id}/creative-proposal-generations/{job_id}`

Owner/project isolation uses generic `GetByIDForProject` semantics; inaccessible/cross-owner jobs are non-disclosing `404`.

Response exposes only feature-safe fields:

```json
{
  "id": "uuid",
  "state": "queued",
  "attempt": 0,
  "max_attempts": 3,
  "error_code": null,
  "proposal_version": null,
  "created_at": "...",
  "updated_at": "...",
  "started_at": null,
  "finished_at": null
}
```

- lifecycle is exactly `queued|running|succeeded|failed` from `JOB_EXECUTION_V1`;
- `proposal_version` appears only for a valid succeeded result;
- raw payload, lease token/deadline, provider response, internal errors and credentials are never exposed.

## Error/retry semantics
Handler mapping in V1:
- `GENERATION_PROVIDER_UNAVAILABLE`: retryable within generic max attempts;
- `GENERATION_PROVIDER_FAILED`: retryable within generic max attempts;
- `GENERATION_INVALID_OUTPUT`: terminal for that explicit generation request; creator may explicitly Regenerate with a new `request_id`;
- transient persistence failures: retryable with safe `GENERATION_PERSISTENCE_FAILED`;
- standard context cancellation/deadline/lease-loss behavior remains owned by the generic executor and must not be converted into a fake feature success.

Presentation-safe error codes never contain raw provider response/error text.

## Frontend behavior
The accepted Proposal workspace gains Generate/Regenerate without changing Proposal edit/approval semantics.

Requirements:
- provider/model uses the safe catalog endpoint, never raw credential inputs;
- if no text-generation option exists, generation is disabled with localized configuration guidance; fake provider is never shown as production AI;
- Generate/Regenerate is disabled while Proposal edits are dirty; creator must save/discard first so a newly-generated draft cannot silently supersede unsaved local work;
- one click creates a fresh `request_id`, POSTs once, then tracks the returned durable job;
- queued/running/succeeded/failed states are visible and localized;
- current Proposal content remains mounted/preserved while generation is pending or fails;
- successful generation opens the returned Proposal version and refreshes version history;
- Regenerate creates a new draft and preserves approved history according to `AI_PROPOSAL_V1`;
- refresh/navigation recovery preserves the active job ID in route/session state so a durable job can be resumed rather than visually forgotten;
- an explicit retry after terminal failure is a new generation request with a new `request_id`.

## TDD / integration gates
Required verification includes:
1. request-time Project/Brief snapshot uses the current owner-scoped Brief revision;
2. same `request_id` retry returns the same job; conflicting reuse is rejected;
3. no credentials in HTTP body, job payload/result or presentation errors;
4. handler persists nothing on generation failure/invalid output;
5. crash/reclaim after Proposal commit cannot create a second Proposal for the same job;
6. distinct regenerate jobs preserve approved history and one-active-draft invariants;
7. owner/project job status isolation;
8. fake provider exists only in deterministic test composition;
9. UI dirty-state protection and pending/failure state preservation;
10. page refresh can resume a non-terminal generation job;
11. full PostgreSQL integration plus end-to-end local smoke through Proposal approval.

## Live-provider boundary
TASK-009 is implementable and mergeable with an empty production provider registry as long as product-visible generation is disabled when no live provider is configured and deterministic fake providers remain test-only.

AI Proposal is not production-complete until a separately accepted live provider + secure credential/BYOK runtime path registers at least one production text-generation option.