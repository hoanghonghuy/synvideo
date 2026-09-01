# OpenAI-Compatible Text Provider V1 Contract

Status: FROZEN for `WAVE-F1-E`.

This contract adds the first live-network text-generation adapter behind the accepted `providers.TextGenerator` capability without leaking vendor SDKs or credentials into Proposal/Script generation domains.

## Boundary
The adapter lives at the provider/infrastructure boundary only.

It does not:
- expose HTTP routes or frontend settings;
- persist user credentials;
- change Proposal/Script contracts;
- register deterministic fake providers in production;
- import an OpenAI vendor SDK into core/domain packages.

A later secure BYOK/runtime task owns creator credential storage/settings and final application registration.

## Protocol
V1 targets OpenAI-compatible chat-completions HTTP APIs.

Configuration supplied to the adapter/factory:
- stable internal `provider_id`;
- display name;
- base URL;
- secret API key supplied through an injected secret/credential source, never through `TextGenerationRequest`;
- one or more model registrations mapping stable internal `model_id` to provider/external model identifier;
- request timeout / bounded response-size settings with safe defaults.

The adapter sends a context-bound `POST` to the compatible chat-completions endpoint using:
- bearer authorization from the injected secret source;
- selected external model identifier;
- accepted `providers.TextMessage` role/content sequence.

No credential is copied into provider metadata, model metadata, generation request objects, returned usage metadata or presentation errors.

## Response mapping
Success requires a valid response with at least one usable textual assistant choice.

The adapter maps the first accepted text choice into `providers.TextGenerationResponse` using the original stable internal provider/model IDs.

Usage metadata may be populated only from non-secret provider usage fields supported by the accepted provider contract.

Malformed/empty/incompatible success payloads map to a safe provider execution error; raw response bodies are not surfaced to presentation layers.

## Error safety
At minimum:
- context cancellation/deadline propagates unchanged;
- authentication/configuration-style failures such as `401/403` map to `providers.ErrProviderUnavailable` or an equivalent safe unavailable classification;
- rate-limit and transient `5xx` failures map to a safe execution/unavailable classification usable by higher-level retry policy;
- unsupported/malformed provider success payloads map to `providers.ErrProviderExecution`;
- raw upstream error bodies, authorization headers, API keys and full request dumps are never embedded in returned errors.

Error wrapping may retain safe HTTP status/category context for diagnostics but must remain secret-free.

## Resource bounds
V1 must protect the process from unreasonable upstream responses:
- bounded HTTP client timeout;
- context-aware request cancellation;
- bounded response-body read size with a documented default;
- response body always closed;
- no unbounded retry loop inside the adapter. Durable job retry remains owned by job orchestration.

## Registration factory
The package exposes a deterministic way to build accepted `providers.Registration` metadata/bindings for configured models.

Requirements:
- stable internal provider/model IDs are validated through existing provider types;
- each configured text model advertises `CapabilityTextGeneration`;
- duplicate configured stable IDs are rejected;
- external model names/base URL/API key remain adapter configuration and do not leak into generation candidate/domain types.

## Deterministic verification
Tests use `httptest` or an equivalent local fake upstream; CI makes no real external network call.

Required coverage:
1. request method/path/model/messages are mapped correctly;
2. bearer credential reaches the fake upstream but never appears in returned error strings/metadata;
3. valid response maps to stable internal provider/model response IDs and text;
4. malformed/empty success payload is rejected safely;
5. `401/403`, `429`, `5xx` and invalid JSON are classified safely;
6. in-flight cancellation/deadline stops the request and propagates context errors;
7. response-size limit is enforced;
8. configured registration lists only declared text models;
9. no vendor SDK import or Proposal/Script/jobs/httpserver/frontend scope leakage.

## Integration after this task
A later BYOK/runtime task will provide owner-scoped secure credential management, creator-facing provider settings and application registration of one or more configured live adapters. This adapter task remains independently testable and production-capable at the provider boundary.