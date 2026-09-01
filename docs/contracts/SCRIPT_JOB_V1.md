# Script Durable Generation Job V1 Contract

Status: FROZEN for `WAVE-F1-H`.

This contract turns the accepted Script generation engine, Script persistence model, generic durable jobs engine, and owner-scoped BYOK text runtime into a creator-usable asynchronous Script generation capability.

## Product boundary
V1 covers:
- durable Generate / Regenerate requests for Script;
- request-time snapshot of the exact approved Proposal and Project intent;
- owner-scoped provider/model selection using the accepted TASK-017 runtime;
- worker-side Script generation using the accepted `scriptgeneration` engine;
- crash-safe idempotent Script draft persistence;
- feature-specific generation status endpoints.

V1 does not implement the Script creator UI; that is `SCRIPT_WORKSPACE_V1` / TASK-019.

## HTTP API

### Create generation request
`POST /api/v1/projects/{project_id}/script-generations`

Body:
```json
{
  "request_id": "uuid",
  "provider_id": "provider-id",
  "model_id": "model-id"
}
```

Rules:
- authenticated owner only;
- `request_id` is also the durable `jobs.id`;
- response is `202 Accepted` with the safe feature-specific job view;
- the first request requires an owner-visible approved Creative Proposal; choose the highest approved Proposal version visible at request time;
- no approved Proposal -> `409 APPROVED_PROPOSAL_REQUIRED`;
- invalid/unavailable owner provider/model selection -> `400 GENERATION_PROVIDER_UNAVAILABLE`;
- the POST path must not call the provider.

### Request-id replay
Replay semantics are authoritative and must survive ambiguous client retries:
- same `request_id` + same owner/project/provider/model returns the already-created job;
- replay lookup happens before checking current Proposal/provider/credential state;
- a newer approved Proposal, disabled provider, rotated/deleted credential, or other current configuration change must not invalidate a valid replay;
- reuse of the same `request_id` for a different owner/project/provider/model -> `409 GENERATION_REQUEST_CONFLICT`;
- duplicate-enqueue races must re-read the winner and apply the same identity comparison.

### Get generation status
`GET /api/v1/projects/{project_id}/script-generations/{job_id}`

Safe response fields only:
```json
{
  "id": "uuid",
  "state": "queued|running|succeeded|failed",
  "attempt": 1,
  "max_attempts": 3,
  "error_code": "optional-safe-code",
  "script_version": 4,
  "created_at": "...",
  "updated_at": "..."
}
```

Never expose raw job payload/result, provider response, lease token/deadline, credentials, ciphertext, deployment base URL, or external model ID.

## Durable job
Job kind: `script_generation_v1`.

Payload schema marker: `script_generation_job_v1`.

The payload snapshots the request-time generation intent:
- Project: id, content format, aspect ratio, target duration, content locale;
- exact approved Proposal version and all fields required by `SCRIPT_GENERATION_V1`;
- provider ID and model ID;
- schema marker.

The worker must not reload the latest Proposal and silently change the creator-approved source. Durable payloads contain no credential/API key/ciphertext.

Unknown payload fields, malformed JSON, wrong schema marker, invalid IDs, or structurally invalid snapshots fail terminally as `GENERATION_INVALID_PAYLOAD`.

## Worker execution
The handler:
1. strictly decodes the durable payload;
2. resolves the current owner credential from `job.owner_id` through the accepted TASK-017 owner-scoped runtime;
3. calls the accepted provider-neutral `scriptgeneration` engine with the snapshotted Project + approved Proposal and provider/model IDs;
4. persists only a validated Script candidate;
5. returns `{ "script_version": N }` as the generic job result.

Provider credentials are resolved only at execution. Disabled/deleted/unavailable credentials map to retryable `GENERATION_PROVIDER_UNAVAILABLE` without exposing secrets.

## Crash-safe Script persistence
Migration `0010_add_script_generation_idempotency.sql` adds nullable internal `source_generation_job_id uuid` to Script persistence with database uniqueness for non-null values.

`source_generation_job_id` is internal only and must not appear in public Script JSON.

Provide an internal generation persistence operation keyed by `(owner, project, generation_job_id)`:
- first execution atomically creates the Script draft and stores `source_generation_job_id` in the same Script version/create/supersede transaction;
- existing approved Script versions remain immutable;
- a new generated draft supersedes only the prior active unapproved draft according to the accepted Script contract;
- retry after `CreateDraft COMMIT` but before generic `MarkSuccess` returns the already-created Script instead of creating another version;
- DB uniqueness is mandatory; in-memory checks are insufficient.

The source Proposal version and Project content locale are preserved exactly as required by `SCRIPT_V1` / `SCRIPT_GENERATION_V1`.

## Error semantics
- `GENERATION_PROVIDER_UNAVAILABLE`: retryable;
- `GENERATION_PROVIDER_FAILED`: retryable;
- `GENERATION_INVALID_OUTPUT`: terminal for the explicit generation request;
- `GENERATION_INVALID_PAYLOAD`: terminal;
- transient database/persistence failure: retryable `GENERATION_PERSISTENCE_FAILED`;
- context cancellation, lease ownership and stale lease token behavior remain owned by `JOB_EXECUTION_V1`.

Errors and logs never include credentials, Authorization headers, raw upstream bodies, encrypted credential material, or durable raw payloads.

## Runtime composition
TASK-018 registers `script_generation_v1` in the existing generic jobs registry/executor and wires the feature service/HTTP endpoints into the API runtime.

It reuses the existing owner-scoped text runtime from TASK-017. It must not create a second credential/provider registration path.

## Deterministic verification
Required coverage includes:
1. first request snapshots the highest approved Proposal and exact Project fields;
2. no approved Proposal is rejected before enqueue;
3. same request ID replay succeeds despite later Proposal/provider/credential changes;
4. conflicting request ID reuse returns `GENERATION_REQUEST_CONFLICT`;
5. duplicate-enqueue race covers both same-selection and conflicting-selection winners;
6. job payload/result remain secret-free;
7. strict payload decoding rejects unknown/trailing/malformed content;
8. local `httptest` upstream proves the job uses the correct owner's current credential;
9. provider unavailable/failed/invalid-output mappings are correct;
10. crash after Script draft commit but before job success does not create a second Script version on retry;
11. real PostgreSQL concurrency proves the generation-job uniqueness boundary;
12. generated Script public JSON never exposes `source_generation_job_id`;
13. `go test -race ./...` and full verification remain green with no public-network dependency.

## Ownership boundary
TASK-018 may own:
- `apps/api/internal/scriptgenerationjob/**` or cohesive equivalent;
- minimal Script repository/domain internal generation-idempotency extension;
- PostgreSQL Script repository tests/integration;
- migration `0010_add_script_generation_idempotency.sql`;
- Script-generation-specific HTTP handler/tests;
- minimal `apps/api/internal/httpserver/server.go` and `apps/api/cmd/api/main.go` composition.

TASK-018 must not modify `apps/web/**`, `sceneplan/**`, `sceneplangeneration/**`, `mediaasset/**`, `scenemedia/**`, render/publish code, or provider credential settings semantics.