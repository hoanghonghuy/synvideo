# Scene Plan Durable Generation + API V1 Contract

Status: FROZEN for planned `TASK-021`.

This contract turns accepted Scene Plan domain/generation foundations into a durable creator-facing Stage 7 backend capability.

## Product boundary
TASK-021 integrates:
- accepted `SCENE_PLAN_GENERATION_V1` engine;
- accepted `SCENE_PLAN_V1` persistence;
- generic durable jobs;
- accepted owner-scoped text-provider runtime;
- Scene Plan resource HTTP API;
- Scene Plan generation HTTP API;
- application runtime composition.

It does not implement frontend workspace, media acquisition/generation, Scene Media Binding UI, audio, render or publish.

## Source selection
A new explicit generation request uses the highest-version owner-visible **approved Script** for the Project at request time.

The Script is authoritative for:
- `source_script_version`;
- exact approved section narration;
- `source_proposal_version`.

The matching source Proposal must exist, be owner-visible, have exactly the Script's `source_proposal_version`, and be approved. Its visual/voice/music/caption directions are planning context only.

If no approved Script exists, reject with `409 SCRIPT_APPROVAL_REQUIRED`.
If the Script/source Proposal relationship is invalid, reject safely with `409 SCENE_PLAN_SOURCE_INVALID`.

## Request-time snapshot
The durable job payload snapshots, at enqueue time:
- Project ID, content format, aspect ratio, target duration and content locale;
- full approved Script version/sections/estimated duration/notes;
- matching approved Proposal production-direction fields required by `SCENE_PLAN_GENERATION_V1`;
- provider/model selection.

The worker must not reload a newer Script/Proposal and silently change accepted intent.

Credentials/API keys never enter durable payload/result.

## HTTP resource API
Expose accepted Scene Plan resource operations:
- `GET /api/v1/projects/{project_id}/scene-plans` — newest-first history;
- `GET /api/v1/projects/{project_id}/scene-plans/{version}`;
- `PUT /api/v1/projects/{project_id}/scene-plans/{version}` — current draft + optimistic revision only;
- `POST /api/v1/projects/{project_id}/scene-plans/{version}/approve` — current draft + revision.

Public Scene Plan JSON follows `SCENE_PLAN_V1`. Internal generation job IDs are never exposed through the Scene Plan resource.

## Generation API
### Create
`POST /api/v1/projects/{project_id}/scene-plan-generations`

Body:
```json
{
  "request_id": "uuid",
  "provider_id": "provider-id",
  "model_id": "model-id"
}
```

Returns `202` with safe feature-specific job status.

`request_id` is the generic durable job ID.

### Status
`GET /api/v1/projects/{project_id}/scene-plan-generations/{job_id}`

Safe response fields only:
- `id`;
- `state`;
- `attempt`;
- `max_attempts`;
- optional stable `error_code`;
- optional `scene_plan_version` after success;
- created/updated timestamps.

Never return raw durable payload, lease token/deadline, provider raw response, provider operation details or credentials.

## Request-id replay/idempotency
Replay semantics follow accepted Proposal/Script generation rules:
1. lookup owner/project/job ID before current source/provider checks;
2. if an existing Scene Plan generation job has the same owner/project/provider/model selection, return it;
3. conflicting reuse returns `409 GENERATION_REQUEST_CONFLICT`;
4. ambiguous HTTP retry cannot enqueue a second job;
5. duplicate-enqueue races must re-read and compare the winner deterministically.

## Job contract
Job kind: `scene_plan_generation_v1`.
Payload schema version: `scene_plan_generation_job_v1`.

Handler:
1. strictly decode payload and reject unknown/trailing fields;
2. resolve current owner credential/provider/model only at worker execution;
3. run accepted Scene Plan generation engine against the snapshotted Project/Script/Proposal;
4. persist validated candidate idempotently by generation job ID;
5. return generic job result:
```json
{ "scene_plan_version": 3 }
```

## DB-level exactly-once persistence
TASK-021 owns migration exactly:
`0012_add_scene_plan_generation_idempotency.sql`.

Add internal nullable `source_generation_job_id uuid` to Scene Plan persistence with uniqueness for non-null values.

The field is internal only (`json:"-"` or equivalent).

Repository/service adds an internal idempotent create operation by `(owner, project, generation_job_id)`.

Critical crash window:
`Scene Plan draft COMMIT -> worker crash/lease loss -> generic MarkSuccess not committed`.
A reclaimed handler must return the already-created Scene Plan version for that job rather than create another version.

Uniqueness must be enforced in PostgreSQL, not only process memory.

## Generation and version semantics
- first successful generation creates a new Scene Plan draft;
- a new generation atomically supersedes only the previous active unapproved Scene Plan draft;
- approved Scene Plans remain immutable/history-preserved;
- Generate/Regenerate after approval creates a new draft version;
- source Script/Proposal links are request-time snapshot links and immutable.

## Provider behavior
Reuse accepted TASK-017 owner text runtime. Do not create another secret/settings path.

POST validates selected provider/model for the current owner but never calls the upstream provider.
Worker resolves the current owner credential at execution.
Existing request-ID replay returns the durable original job even if credentials/provider availability changed after enqueue.

## Error semantics
At minimum:
- `SCRIPT_APPROVAL_REQUIRED` — request cannot start;
- `SCENE_PLAN_SOURCE_INVALID` — source relationship invalid;
- `GENERATION_REQUEST_CONFLICT` — request ID reused inconsistently;
- `GENERATION_PROVIDER_UNAVAILABLE` — retryable;
- `GENERATION_PROVIDER_FAILED` — retryable;
- `GENERATION_INVALID_OUTPUT` — terminal for explicit request;
- `GENERATION_PERSISTENCE_FAILED` — retryable where appropriate;
- strict invalid payload — terminal `GENERATION_INVALID_PAYLOAD`;
- standard context/lease behavior remains generic executor-owned.

Errors are presentation-safe and secret-free.

## Runtime composition
Register Scene Plan job kind alongside Proposal and Script job kinds in the single accepted generic executor.

Graceful shutdown/lease heartbeat/reclaim behavior remains generic. TASK-021 must not start a second executor loop.

## TDD gates
Required deterministic/real-PostgreSQL evidence:
1. requires highest approved Script;
2. exact request-time Script + matching Proposal snapshot;
3. rejects invalid Script/source Proposal relationship;
4. no provider call in HTTP POST;
5. replay before current credential/source validation;
6. same/conflicting duplicate-enqueue race;
7. strict payload decoding;
8. owner isolation/non-disclosure;
9. provider safe error mapping and local credential smoke;
10. DB generation-job uniqueness;
11. crash-after-draft-before-MarkSuccess retry creates exactly one Scene Plan version;
12. approved Scene Plan history is preserved;
13. public Scene Plan/job JSON contains no internal generation ID/secret/raw payload;
14. executor registers Proposal + Script + Scene Plan kinds together without regression;
15. race tests and full `make verify` green.

## Scheduling gate
Contract is frozen now, but TASK-021 remains BACKLOG/BLOCKED until TASK-018 releases the shared `main.go` / jobs registry / `httpserver` backend hotspot and its accepted Script-job integration pattern is available for revalidation.