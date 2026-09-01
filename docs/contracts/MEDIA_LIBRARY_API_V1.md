# Media Library + Scene Binding API V1 Contract

Status: FROZEN for planned `TASK-023`.

This contract makes accepted Media Asset storage and Scene Media Binding foundations usable through a safe creator-facing backend API and production/local runtime composition.

## Product boundary
TASK-023 exposes:
- project Media Asset upload/list/metadata/content/delete APIs;
- accepted S3-compatible storage adapter through application config/runtime;
- current/history primary-visual Scene Media Binding APIs;
- friendly asset-in-use behavior;
- owner/project-safe binary serving suitable for image/video preview/download.

It does not implement frontend workspace, AI image/video generation, stock search, voice/audio generation, Scene Editor, render or publish.

## Runtime storage configuration
Extend server config with explicit non-secret/secret storage fields, for example:
- `SYNVIDEO_MEDIA_STORAGE_ENDPOINT`;
- `SYNVIDEO_MEDIA_STORAGE_REGION`;
- `SYNVIDEO_MEDIA_STORAGE_BUCKET`;
- `SYNVIDEO_MEDIA_STORAGE_ACCESS_KEY_ID`;
- `SYNVIDEO_MEDIA_STORAGE_SECRET_ACCESS_KEY`;
- `SYNVIDEO_MEDIA_STORAGE_PATH_STYLE`;
- bounded operation timeout.

Exact env names may follow current config naming conventions but must be documented/tested.

Requirements:
- when Media API is enabled, invalid or incomplete explicitly supplied storage configuration fails startup rather than creating a half-working API;
- credentials never appear in logs/errors/public JSON;
- use accepted `mediaasset/s3storage` adapter; do not implement a second S3 client;
- local Compose/SeaweedFS-compatible configuration remains deterministic and public internet is not required for CI/integration;
- bucket ensure/create may be development/test-only; production startup must not assume permission to create buckets unless explicitly configured.

## Public Media Asset JSON
Safe metadata fields only:
- `id`;
- `project_id`;
- `kind`;
- `origin`;
- `mime_type`;
- `byte_size`;
- `sha256`;
- optional original filename;
- safe technical `metadata` object;
- timestamps.

Never expose `owner_id`, object key, bucket, endpoint, storage credentials or provider secrets.

## Upload API
`POST /api/v1/projects/{project_id}/media-assets`

Use streaming `multipart/form-data` or an equivalently bounded streaming request contract. V1 supports creator upload only; server assigns `origin=upload`.

Required fields:
- one file body;
- optional safe metadata JSON object if supported.

The client cannot choose:
- owner/project identity beyond authorized route;
- asset UUID;
- object key;
- storage bucket;
- arbitrary `origin`;
- checksum claimed as authoritative.

Server derives kind from an allowlisted MIME mapping and/or validated request metadata. At minimum supported creator visual/audio MIME families are explicit; unsupported types return a validation error rather than being silently stored as a misleading kind.

Security/resource requirements:
- configurable max upload bytes with a sane finite default;
- reject before/until exceeding the bound without buffering the whole file in memory;
- filename treated as display metadata only, never a filesystem/object-key path;
- reject multipart ambiguity/multiple unexpected file parts;
- content type is validated/canonicalized rather than trusting an unsafe arbitrary header string;
- context cancellation stops storage upload;
- storage-success + DB-failure compensation semantics remain accepted Media Asset behavior.

Success: `201` safe Media Asset metadata.

## List/get metadata
- `GET /api/v1/projects/{project_id}/media-assets?limit=...`
- `GET /api/v1/projects/{project_id}/media-assets/{asset_id}`

Owner/project scoped, deterministic newest-first list with accepted bounded limit.

Future filtering/search may extend the API; TASK-023 need not invent database search over arbitrary metadata.

## Content endpoint
`GET /api/v1/projects/{project_id}/media-assets/{asset_id}/content`

Authorization happens through owner/project metadata before object access.

V1 content serving requirements:
- set safe `Content-Type` from stored validated metadata;
- set `Content-Length` when known;
- set a non-executable/download-safe `Content-Disposition` policy appropriate to preview vs original filename;
- `X-Content-Type-Options: nosniff`;
- stream object; do not read full asset into server memory;
- object/storage not-found maps to safe feature error without exposing object keys.

### Byte ranges
Image/video preview and later editor playback require efficient seek behavior. V1 therefore supports one standard single `Range: bytes=...` request and `206 Partial Content` for stored objects where the adapter can provide range reads.

TASK-023 may minimally extend the provider-neutral ObjectStorage port with a range-open operation and implement it in the accepted S3 adapter. It must not leak MinIO/AWS response types into domain/HTTP code.

Reject malformed/multiple/unsatisfiable ranges safely (`416` where appropriate). Full GET remains supported.

## Delete API
`DELETE /api/v1/projects/{project_id}/media-assets/{asset_id}`

Owner/project scoped.

If the asset is referenced by active **or historical** Scene Media Binding rows, return stable `409 MEDIA_ASSET_IN_USE`; do not delete binding history or leave dangling references.

For an unreferenced asset, reuse accepted Media Asset deletion semantics. Storage/persistence partial failure must return a safe actionable error; do not claim success while durable metadata/object state is inconsistent.

## Scene Media Binding API
Built over accepted TASK-020 service/repository contract.

### Current assignments for plan
`GET /api/v1/projects/{project_id}/scene-plans/{scene_plan_version}/media-bindings`

Return entries in Scene Plan scene order and include unbound scenes explicitly.

Safe entry includes:
- `scene_key`;
- `role=primary_visual`;
- optional active binding metadata;
- optional safe selected Media Asset metadata sufficient for preview.

### Assign/replace primary visual
`PUT /api/v1/projects/{project_id}/scene-plans/{scene_plan_version}/scenes/{scene_key}/primary-visual`

Body:
```json
{ "asset_id": "uuid" }
```

Uses accepted atomic assignment semantics:
- approved plan only;
- image/video same owner/project;
- same active asset is idempotent;
- replacement preserves old history and increments binding version once.

### History
`GET /api/v1/projects/{project_id}/scene-plans/{scene_plan_version}/scenes/{scene_key}/primary-visual/history`

Return deterministic newest/version-desc history with safe asset metadata/provenance, not storage internals.

## Error categories
At minimum stable presentation codes:
- `MEDIA_ASSET_NOT_FOUND`;
- `MEDIA_ASSET_INVALID`;
- `MEDIA_ASSET_TOO_LARGE`;
- `MEDIA_ASSET_UNSUPPORTED_TYPE`;
- `MEDIA_ASSET_STORAGE_FAILED`;
- `MEDIA_ASSET_PERSISTENCE_FAILED`;
- `MEDIA_ASSET_IN_USE`;
- accepted Scene Media Binding not-found/plan-not-approved/scene-not-found/asset-kind errors;
- generic unauthenticated/non-disclosing owner mismatch.

No raw SQL, object key, bucket/endpoint, SDK response or credential leaks.

## HTTP/runtime TDD gates
1. runtime uses accepted S3 adapter/config and fails explicit invalid config safely;
2. storage secrets absent from logs/errors/public response fixtures;
3. bounded streaming upload success + SHA/size server-derived;
4. oversize rejected without full buffering;
5. unsafe/unsupported MIME and multipart ambiguity rejected;
6. cross-owner/project list/get/content/delete non-disclosing;
7. full content stream safe headers;
8. valid single byte range returns correct `206`; malformed/unsatisfiable ranges safe;
9. object key never public;
10. active/historical bound asset delete maps to `MEDIA_ASSET_IN_USE`;
11. unreferenced delete executes accepted storage/metadata behavior;
12. current scene binding list preserves approved Scene Plan order/unbound scenes;
13. assign same asset idempotent, replace preserves history;
14. no frontend/generation/audio/render scope leakage;
15. real PostgreSQL + local S3-compatible integration + race/full verification green.

## Scheduling gate
Contract is frozen now. TASK-023 remains BACKLOG/BLOCKED until TASK-020 is accepted and the shared backend composition surface is released by TASK-018/TASK-021 scheduling. Before activation, revalidate exact ObjectStorage range extension against accepted TASK-020/021 heads.