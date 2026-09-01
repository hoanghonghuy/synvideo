# Media Asset + Object Storage V1 Contract

Status: FROZEN for the next Creative Workflow implementation wave.

This contract establishes the durable Stage 8 media object boundary before scene-level acquisition/generation workflows are added.

## Product boundary
V1 provides:
- owner/project-scoped durable media metadata;
- a vendor-neutral object storage port;
- an S3-compatible storage adapter suitable for local MinIO and future cloud S3-compatible services;
- deterministic integrity/size/type metadata.

V1 does **not** expose upload HTTP routes, generate media through AI providers, bind assets to Scene Plan scenes, synthesize audio, render video, or publish.

Those are follow-on orchestration/product tasks built on this foundation.

## Media Asset resource
Each asset has:
- `id` UUID;
- `owner_id` internal persistence scope only;
- `project_id` UUID;
- `kind`: `image | video | audio | document | other`;
- `origin`: `upload | creator_media | stock | generated_image | generated_video | generated_audio | system`;
- `object_key`: opaque server-controlled storage key;
- `mime_type`: trimmed required max 255;
- `byte_size`: non-negative integer;
- `sha256`: lowercase 64-hex digest of stored bytes;
- optional original filename, max 500 Unicode characters;
- optional metadata JSON object for safe non-secret technical attributes;
- `created_at` and `updated_at`.

`object_key` is never accepted from an untrusted future client as an ownership selector. It is generated/validated server-side.

## Object storage port
Core media code depends on a small provider-neutral interface, not AWS/MinIO SDK types.

Required operations:
- put/stream an object under a server-supplied key;
- stat/head object metadata;
- open/read object content as a stream;
- delete object;
- distinguish not-found from transport/storage failure.

The interface must preserve context cancellation/deadline.

No public bucket names, credentials, endpoint secrets or SDK-specific response structs leak into media domain types.

## S3-compatible adapter
TASK-016 implements one adapter behind the port with configurable:
- endpoint/base URL where required;
- region where required;
- bucket;
- access key/secret via injected server-side configuration/credential source;
- path-style/compatible mode when required for local MinIO;
- bounded operation timeouts through context/client configuration.

Security/invariants:
- credentials never appear in returned errors/domain metadata/loggable request objects;
- adapter never trusts arbitrary external object keys to cross project ownership boundaries;
- errors are mapped to stable storage categories;
- object bodies are always closed;
- no unbounded internal retry loop; durable orchestration owns retry policy when future jobs use storage.

## Persistence transaction model
Asset metadata is stored in PostgreSQL separately from object bytes.

Creation flow used by this foundation's service:
1. validate owner-visible Project;
2. generate canonical asset UUID/object key;
3. stream bytes to object storage while calculating SHA-256 and byte size;
4. persist metadata only after storage succeeds;
5. if DB persistence fails after object write, perform best-effort compensating delete and return a storage/persistence-safe error;
6. never return a successful asset resource if either object write or metadata persistence failed.

This is not a distributed transaction. Tests must make the failure window explicit and prove compensation is attempted.

Deletion flow:
- verify owner/project scope through metadata first;
- remove object and metadata with clearly documented ordering;
- failures must not disclose cross-owner asset existence.

V1 may choose object-first or DB-first deletion, but tests must prove recoverable/error-safe semantics and no cross-owner access.

## Object key convention
Canonical keys are server generated and deterministic from ownership identity, for example:
`projects/{project_id}/assets/{asset_id}` plus a safe optional extension.

Exact formatting may vary, but:
- no `..` traversal;
- no user-supplied absolute paths/URLs;
- asset/project identity must be represented in a server-controlled way;
- object key uniqueness is enforced.

## Local development
The accepted technical baseline requires S3-compatible local development.

TASK-016 may add MinIO (or an equivalent S3-compatible local service) to Docker Compose and deterministic integration configuration.

Tests must not require public internet access.

## Migration
TASK-016 owns migration `0008_create_media_assets.sql` only.

It references Projects but does not reference Scene Plan tables, so TASK-015 and TASK-016 remain merge-order independent.

Required database constraints/indexes include:
- primary UUID identity;
- Project FK/cascade behavior consistent with accepted Project lifecycle;
- allowed kind/origin values;
- non-negative byte size;
- SHA-256 format;
- unique object key;
- owner/project query indexes as appropriate.

## Repository/service operations
Foundation supports at least:
- create/store an asset from a reader/stream under owner/project scope;
- get asset metadata by owner/project/id;
- list project assets newest-first with a bounded result contract;
- open asset content only after owner/project metadata authorization;
- delete asset under owner/project scope.

No generic public HTTP API is required in TASK-016.

## Deterministic verification
Required coverage:
1. S3-compatible put/stat/open/delete mapping against a deterministic local test service or MinIO integration;
2. context cancellation propagation;
3. credential/error safety;
4. bounded/scoped object keys and traversal rejection;
5. SHA-256 and byte-size metadata correctness;
6. owner/project non-disclosure;
7. object-success + DB-failure compensation path;
8. DB metadata never created when object write fails;
9. deletion/error semantics;
10. real PostgreSQL metadata constraints;
11. no Scene Plan/Proposal/Script/httpserver/frontend/provider-generation scope leakage.