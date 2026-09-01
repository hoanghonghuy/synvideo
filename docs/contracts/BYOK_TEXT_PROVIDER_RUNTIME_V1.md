# BYOK Text Provider Runtime V1 Contract

Status: FROZEN for `WAVE-F1-G`.

This contract turns the accepted OpenAI-compatible provider adapter and durable Proposal-generation flow into a creator-usable live AI capability without putting provider credentials into request bodies, job payloads, logs, browser storage or public provider metadata.

## Product boundary
V1 provides:
- owner-scoped BYOK API-key lifecycle;
- encrypted-at-rest provider credentials;
- deployment-managed OpenAI-compatible provider/model definitions;
- creator-facing provider settings;
- owner-scoped live text-generation options;
- runtime resolution of the accepted TASK-013 adapter for Proposal generation jobs.

V1 does not:
- accept arbitrary creator-supplied provider base URLs;
- implement OAuth provider accounts;
- add image/video/audio providers;
- add Script/Scene Plan generation UI;
- put credentials into durable jobs;
- expose provider credentials after save.

Arbitrary custom endpoints are deliberately deferred because an untrusted URL creates SSRF/DNS-rebinding requirements that should not be smuggled into the first BYOK task.

## Deployment provider definitions
The server loads a non-secret provider-definition catalog at startup from validated deployment configuration (for example a JSON environment/config value).

Each definition contains:
- stable internal `provider_id`;
- creator-facing display name;
- OpenAI-compatible base URL;
- one or more models with stable internal `model_id`, display name and external/upstream model ID;
- optional bounded timeout/response-size overrides.

Requirements:
- provider/model IDs use the accepted `providers` identity rules;
- definitions are deterministic and duplicate IDs fail startup/config loading safely;
- creator HTTP input never supplies or overrides base URL/external model ID;
- base URLs reuse TASK-013 validation and redirect protections;
- production definitions never register the deterministic fake provider;
- a stable internal provider/model ID must not be repurposed to a semantically different upstream target while durable jobs may still reference it.

## Owner-scoped persisted settings
Migration `0009_create_text_provider_settings.sql` stores one row per `(owner_id, provider_id)`.

Durable fields include at least:
- `owner_id uuid`;
- `provider_id text`;
- positive `revision`;
- `enabled boolean`;
- `enabled_model_ids jsonb` array of stable internal model IDs;
- encrypted API-key ciphertext;
- AEAD nonce;
- encryption key version;
- `created_at`, `updated_at`.

The API key is never stored as plaintext in PostgreSQL.

`provider_id`/model selections must correspond to the deployment definition catalog at service validation time. Database constraints enforce structural invariants where practical; application validation owns catalog membership.

## Credential encryption
V1 uses an authenticated encryption boundary backed by a deployment secret, with a standard-library primitive such as AES-256-GCM.

Requirements:
- the active master key comes only from server-side configuration/environment and is never committed;
- key material must be exactly validated before use;
- every write uses a fresh cryptographically random nonce;
- authenticated additional data binds at least owner ID, provider ID and encryption-key version so ciphertext cannot be transplanted between owners/providers;
- tampered ciphertext/nonce/AAD fails closed;
- raw encryption/decryption errors and key material never reach presentation errors;
- decrypted API keys are never persisted in caches, provider metadata, job payloads/results or browser-visible responses;
- decrypted secret lifetime in process memory is minimized and scoped to provider execution/runtime resolution.

If the deployment has no valid credential-encryption key, the application may continue serving non-BYOK product surfaces, but BYOK settings/runtime must be unavailable with a stable safe configuration error. It must never silently fall back to plaintext storage.

## API-key semantics
API keys are opaque secrets.

Validation:
- required on first configuration;
- bounded to a documented safe size (V1 maximum 8192 bytes);
- empty or leading/trailing-whitespace-only mistakes are rejected rather than silently transformed;
- an update may omit `api_key` to preserve the existing encrypted credential;
- an update that supplies `api_key` rotates/re-encrypts the credential and increments revision;
- responses expose only `has_api_key: true|false`, never a masked secret substring.

Deletion removes the owner/provider credential row. A stale delete/update must not overwrite/remove a newer setting revision.

## Provider settings HTTP API
All routes use the existing authenticated principal/owner boundary.

### List safe settings
`GET /api/v1/ai/provider-settings`

Returns deployment-defined providers/models plus owner-safe state, for example:

```json
{
  "providers": [
    {
      "id": "openai",
      "display_name": "OpenAI",
      "configured": true,
      "enabled": true,
      "has_api_key": true,
      "revision": 2,
      "models": [
        {"id":"gpt-5-mini","display_name":"GPT-5 mini","enabled":true}
      ]
    }
  ]
}
```

The response does not expose API keys, ciphertext, nonce, encryption metadata, base URLs or external model IDs.

### Create/update one setting
`PUT /api/v1/ai/provider-settings/{provider_id}`

Body:

```json
{
  "revision": 2,
  "enabled": true,
  "enabled_model_ids": ["gpt-5-mini"],
  "api_key": "optional-on-update"
}
```

Rules:
- first create requires no revision and requires `api_key`;
- update requires current revision;
- stale revision -> stable `409 STALE_REVISION`-style error following existing resource conventions;
- unknown provider/model -> validation error;
- `enabled=true` requires an encrypted credential and at least one enabled text model;
- response is the safe settings view only.

### Delete one setting
`DELETE /api/v1/ai/provider-settings/{provider_id}?revision={current_revision}`

Rules:
- requires the current revision;
- stale revision is rejected;
- owner isolation is non-disclosing;
- successful deletion removes the credential and immediately removes that provider from owner generation options.

## Owner-scoped generation runtime
Introduce a cohesive owner-scoped text-provider runtime/resolver rather than mutating the existing global registry with per-owner secrets.

It provides at least:
- list enabled text-generation options for an owner;
- validate/resolve an owner + provider + model selection;
- obtain a provider-neutral `TextGenerator` bound to the selected deployment definition and that owner's credential.

TASK-009 integration changes:
- `GET /api/v1/ai/text-generation-options` becomes principal-aware and returns only the current owner's enabled/configured text models;
- Proposal generation creation validates selection against the owner-scoped runtime;
- the durable job still snapshots only stable provider/model IDs and Project/Brief intent; no secret/ciphertext is added;
- at worker execution the handler resolves the current owner's credential using durable `job.owner_id` and stable provider/model IDs, then invokes the accepted Proposal generation engine;
- deleting/disabling/rotating a credential affects subsequent execution/resolution; a credential unavailable at execution maps to safe retryable `GENERATION_PROVIDER_UNAVAILABLE` behavior under the accepted job contract;
- same-`request_id` replay semantics remain unchanged and must not be broken by current credential/config changes.

The existing generic `providers.Registry` remains useful for static/test registrations; per-owner credentials must not be globally registered or shared across owners.

## Creator settings UI
Add a localized AI Provider settings workspace reachable from the application UI.

Required behavior:
- list deployment-supported providers/models;
- show configured/enabled state without revealing any portion of the stored key;
- first configuration accepts an API key and model selection;
- update can enable/disable models/provider without re-entering the key;
- replacing the key uses an explicit new secret input which is cleared from component state after success;
- deleting configuration requires explicit confirmation;
- stale/save/network errors are localized and do not echo submitted secrets;
- secret input uses appropriate password/autocomplete behavior and is never stored in route query, localStorage or sessionStorage;
- after settings change, Proposal generation options refresh and accurately reflect owner availability.

## Error and logging safety
Stable presentation errors must not contain:
- submitted API key;
- encrypted credential/nonce;
- master encryption key;
- Authorization header;
- upstream error body;
- deployment base URL/external model IDs where they are not already public product metadata.

HTTP/server/provider logs must not serialize request bodies for credential-setting routes. Tests must inspect relevant error strings/JSON and prove sentinel secrets are absent.

## Concurrency and owner isolation
Required behavior:
- owner A cannot read/update/delete/use owner B settings;
- same provider may be independently configured by many owners;
- concurrent same-revision updates allow one success and stale competitors;
- deleting/rotating one owner's key cannot affect another owner's adapter/runtime;
- generation option listing is owner-specific and deterministic.

## Migration/order boundary
TASK-017 owns only `0009_create_text_provider_settings.sql`.

The migration depends on no TASK-015/TASK-016 tables and can merge independently of migrations `0007`/`0008` under the accepted filename-tracking migration runner.

## Deterministic verification
Required coverage includes:
1. encryption round-trip with random nonce and owner/provider AAD binding;
2. tampered ciphertext/nonce/AAD fails closed;
3. plaintext secret is absent from database rows, JSON responses and returned/loggable errors;
4. create/update/rotate/preserve/delete credential semantics and stale revisions;
5. owner isolation for list/update/delete/runtime resolution;
6. deployment definition validation and no fake production registration;
7. generation options expose only configured/enabled owner models;
8. Proposal generation job execution uses the correct owner's credential through local `httptest` upstream;
9. replay of an existing request ID does not start failing merely because current credential/provider configuration changed;
10. credential unavailable at worker execution maps safely to retryable provider-unavailable semantics with no secret leak;
11. settings UI secret state is cleared after save and never persisted client-side;
12. real PostgreSQL integration, race tests and full frontend/backend CI are green with no public internet dependency.

## Follow-on boundary
After TASK-017 is accepted, AI Proposal becomes live-provider usable for configured creators. The next shared-runtime task may integrate durable Script generation using the same owner-scoped provider runtime rather than inventing a second credential path.