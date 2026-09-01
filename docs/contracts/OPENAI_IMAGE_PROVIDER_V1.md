# OpenAI Image Generation Adapter V1 Contract

Status: FROZEN for planned `TASK-026`.

This contract implements the first live image-generation adapter behind accepted/planned provider-neutral `VISUAL_GENERATION_PROVIDER_V1` image port.

## Research gate / why image-only
As of 2026-09-01, OpenAI's official API docs mark the Sora video API as deprecated and scheduled for shutdown on 2026-09-24. TASK-026 therefore intentionally does **not** build an OpenAI video adapter that would become obsolete almost immediately.

Current OpenAI image models expose image generation through the Images API, while future video work will select an actively supported provider/API through a separate revalidation gate.

## Product boundary
TASK-026 provides:
- live HTTP OpenAI image-generation adapter;
- injected adapter config/secret/model mapping;
- safe request translation and generated-image decoding;
- bounded HTTP/response/base64 handling;
- cancellation/error/usage mapping;
- local `httptest` verification.

It does not:
- modify owner provider settings/runtime;
- register itself in `main.go`;
- persist jobs/Media Assets;
- bind scenes;
- implement video/TTS;
- touch frontend/httpserver.

## Primary package
New isolated adapter package such as:
`apps/api/internal/providers/openaiimage/**`.

Do not place provider-specific request/response structs in core `providers` package.

## Configuration
Adapter config includes:
- stable internal ProviderID supplied by caller/deployment layer;
- injected API key/secret;
- base URL defaulting to the canonical OpenAI API root but replaceable for deterministic local tests;
- stable internal ModelID -> external OpenAI image model string mapping;
- bounded HTTP timeout;
- maximum response JSON bytes;
- maximum decoded image bytes per output;
- maximum number of outputs allowed by this adapter config.

Validation:
- API key exact string required; do not trim secret mistakes silently;
- HTTPS required for non-loopback/non-test base URLs;
- reject user-info/query/fragment and unsafe URL forms;
- model external ID non-empty/canonical;
- all byte/count/time bounds finite and positive where configured.

No creator-supplied arbitrary base URL in this task.

## HTTP behavior
Use direct HTTP/infrastructure transport rather than importing an OpenAI SDK into core contracts.

The adapter uses the currently supported OpenAI Images generation endpoint (`/v1/images/generations` relative to canonical API root) and external model mapping supplied by config.

Tests target a local fake upstream and must not require public OpenAI network/API keys.

## Request mapping
Map provider-neutral request conservatively:
- prompt;
- external model ID;
- requested output count within adapter/model bounds;
- aspect ratio to a supported OpenAI size mapping only when deterministic;
- reference image inputs are **not** part of TASK-026 generation V1 unless the accepted ImageGenerator request can map them to this endpoint without switching to a different edit API.

Unsupported optional generic hints (negative prompt, seed, etc.) must not be silently reinterpreted. The adapter either:
- declares the option unsupported and returns a stable unsupported-parameter error when the caller explicitly requests it; or
- maps it only when the OpenAI endpoint has equivalent documented semantics.

No vendor quality/style knobs are added to core request types for this adapter.

## Output mapping
Prefer response mode that returns image data within the authenticated API response rather than trusting arbitrary remote/signed URLs when supported by the selected OpenAI image model/API.

For each returned image:
- strictly decode expected JSON shape;
- bounded base64 decode or bounded binary extraction;
- validate non-empty output and adapter-supported canonical image MIME;
- return provider-neutral generated binary/result;
- map safe usage/request metadata when present.

Do not expose revised prompt/raw provider response/signed URL unless a later explicit provenance contract accepts a sanitized field.

## Resource safety
- bounded request body/reference handling inherited from core input validation;
- bounded response reader before JSON decode;
- bounded decoded base64 image bytes;
- response body always closed;
- no unbounded retry loop;
- context cancellation/deadline propagates to HTTP;
- reject output count/aggregate decoded bytes that exceed configured safe limits.

## Redirect/SSRF safety
HTTP client must not follow redirects to unvalidated hosts by default. If redirects are required by a documented OpenAI endpoint in future, each redirect target must be revalidated and authorization must never be forwarded cross-origin.

Images generation V1 should not need arbitrary output URL fetching.

## Error mapping
Map without leaking raw upstream body or Authorization:
- `401/403` -> provider authentication/config unavailable;
- `429` -> safe rate-limited/transient category;
- `5xx` -> transient provider execution failure;
- supported `4xx` request rejection -> terminal provider request error;
- malformed/trailing/oversized success response -> invalid provider response;
- invalid base64/empty image -> invalid provider response;
- context cancellation/deadline unchanged.

Raw provider error bodies may contain user prompt/account detail and are never returned verbatim.

## Secret safety
Tests explicitly prove:
- expected bearer/API authorization reaches the local fake upstream;
- API key never appears in returned errors;
- Authorization never appears in debug metadata;
- raw response body containing a sentinel secret is not surfaced;
- config/registration metadata is safe to log only after secret omission.

## Registration factory
Expose a deterministic adapter-local factory/registration helper that can bind configured models to `CapabilityImageGeneration` and the provider-neutral ImageGenerator interface.

TASK-026 does not install it into production runtime. Future multi-capability owner runtime owns credential resolution/registration.

## TDD gates
1. config/base URL/model/bound validation;
2. exact secret preservation and auth header reaches fake upstream;
3. prompt/model/count/size mapping;
4. unsupported generic hint is explicit, not silently changed;
5. valid bounded image response decoded into provider-neutral output;
6. multiple-output/aggregate bound;
7. malformed/trailing/oversized JSON and invalid base64 rejected;
8. 401/403/429/4xx/5xx safe mapping;
9. cancellation/deadline propagation;
10. response body close and redirect safety;
11. secret/raw-body sentinel never leaks;
12. deterministic registration factory advertises image capability only;
13. no main/http/jobs/storage/frontend/video scope leakage;
14. full backend/race verification green.

## Scheduling
TASK-026 depends on accepted TASK-025. Keep BACKLOG until the visual provider-neutral port/registry shape is merged and revalidate official OpenAI Images API details at activation time because media APIs evolve quickly.