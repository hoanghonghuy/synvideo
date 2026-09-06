# Stock Media operations

TASK-033 exposes a project-scoped stock-media flow backed by a provider adapter. V1 enables Pexels when the API process receives `SYNVIDEO_PEXELS_API_KEY`.

## Credential setup

Set `SYNVIDEO_PEXELS_API_KEY` only in the API/server environment. Do not expose it through Vite, browser configuration, logs, screenshots, or project metadata. If the key is absent, the application still starts; stock-media requests return the explicit `STOCK_MEDIA_PROVIDER_UNAVAILABLE` error while upload/generated media flows continue to work.

A real provider credential is not required by ordinary unit/CI tests. Provider tests use deterministic local HTTP fixtures and must not contact Pexels.

## Runtime flow

1. The browser searches through `/api/v1/projects/{project_id}/stock-media/search` with an explicit provider, query, image/video kind, orientation, page and bounded page size.
2. Search results are remote candidates only. They expose provider identity, preview, source page, creator, license summary/reference, attribution text and whether acquisition is currently supported.
3. A user explicitly requests acquisition through `/api/v1/projects/{project_id}/stock-media/acquisitions` using only provider key, provider result ID and media kind.
4. The API re-resolves that exact provider item at acquisition time. Browser-supplied provenance/download URLs are not trusted.
5. The resolved bytes are copied into SynVideo object storage through the existing bounded MediaAsset storage path and persisted with `origin=stock` plus truthful provenance metadata.
6. Project-scoped provider/result identity is deduplicated. Retries return the existing durable MediaAsset rather than creating uncontrolled duplicates.
7. Only after durable acquisition can the asset be assigned to a scene through the existing Scene Media binding/history API.

## Failure and recovery semantics

- Provider rate limiting: `429 STOCK_MEDIA_RATE_LIMITED`; `Retry-After` is forwarded when supplied by the provider.
- Source removed/unavailable: `410 STOCK_MEDIA_SOURCE_UNAVAILABLE`; no silent substitute is selected.
- Provider credential rejected: `503 STOCK_MEDIA_PROVIDER_AUTH_FAILED`.
- Provider not configured: `503 STOCK_MEDIA_PROVIDER_UNAVAILABLE`.
- Durable asset exceeds configured media limit: `413 STOCK_MEDIA_TOO_LARGE`.
- Object storage failure: `502 STOCK_MEDIA_STORAGE_FAILED`.
- Other upstream failures: `502 STOCK_MEDIA_PROVIDER_FAILED`.

The UI must keep existing project MediaAssets and scene bindings intact across provider failures. Retrying search/acquisition is explicit; provider fallback is never silent in V1.

## Persistence and migrations

Migration `0019_add_stock_media_identity.sql` adds a partial unique index for active stock MediaAssets using owner, project, media kind, provider key and provider result ID. Provenance is stored in MediaAsset JSON metadata (`stock_provider`, `stock_result_id`, source page, creator, license and attribution fields) so canonical scene bindings continue to reference only SynVideo MediaAsset IDs rather than provider-specific IDs.
