# Visual Generation Provider V1 Contract

Status: FROZEN for planned `TASK-025`.

This contract establishes provider-neutral image/video generation ports and deterministic test fakes before any live media adapter, credential runtime or paid per-scene orchestration is wired.

## Product boundary
TASK-025 provides:
- provider-neutral image generation types/port;
- provider-neutral asynchronous video generation types/port;
- capability-aware registry bindings/resolution for accepted `image_generation` and `video_generation` capabilities;
- deterministic fake implementations for tests;
- safe provider-neutral error categories and metadata.

It does not:
- persist Media Assets;
- submit creator jobs through generic jobs;
- resolve creator credentials;
- bind generated assets to scenes;
- call a real external provider;
- modify frontend/httpserver/main/runtime.

## Why image and video ports are separate
Image generation may complete in one request and return/stream image bytes.
Video generation commonly creates an external long-running operation that must be polled and later downloaded.

Do not collapse both into a universal `Generate() -> []byte` abstraction. Async provider operation identity must remain explicit so later durable orchestration can resume rather than re-submit paid work after process failure.

## Shared identifiers
Reuse accepted:
- `providers.ProviderID`;
- `providers.ModelID`;
- `providers.CapabilityImageGeneration`;
- `providers.CapabilityVideoGeneration`;
- provider/model metadata conventions and safe errors where applicable.

Provider-specific model names/request schemas never leak into core request types.

## Image generation port
Conceptual interface:
```go
type ImageGenerator interface {
    GenerateImage(context.Context, ImageGenerationRequest) (ImageGenerationResponse, error)
}
```

### ImageGenerationRequest
Provider-neutral fields required for first-stage production:
- `Prompt` required bounded Unicode text;
- optional `NegativePrompt` where adapters may ignore only if their declared capability/metadata says unsupported;
- `AspectRatio` using a provider-neutral ratio string/type compatible with accepted Project ratios where possible;
- optional requested output count with a small finite bound;
- optional deterministic seed only as a nullable hint, never assumed supported;
- optional safe reference images represented as caller-provided read/open handles or provider-neutral binary inputs with explicit MIME and finite size, not Media Asset repository types;
- model/provider identity is resolved before calling the port and is not duplicated as raw vendor names inside request.

V1 does not invent provider-specific knobs such as CFG/sampler/steps in core.

### ImageGenerationResponse
One or more generated outputs, each with:
- provider-neutral `GeneratedBinary`/stream opener or bounded binary result abstraction;
- canonical MIME type (`image/png`, `image/jpeg`, `image/webp` as declared/supported);
- optional width/height when safely known;
- optional safe usage metadata;
- optional opaque provider request ID only for diagnostics, never credentials or signed URLs.

Core callers must be able to consume output without knowing whether upstream returned base64 or bytes.

## Video generation port
Video is modeled as an external operation lifecycle.

Conceptual interface:
```go
type VideoGenerator interface {
    StartVideo(context.Context, VideoGenerationRequest) (VideoOperation, error)
    GetVideoOperation(context.Context, string) (VideoOperation, error)
    OpenVideoResult(context.Context, string) (GeneratedBinary, error)
}
```

The operation ID is opaque to core.

### VideoGenerationRequest
Provider-neutral fields:
- required bounded `Prompt`;
- optional safe reference/start image binary input;
- `AspectRatio`;
- optional target duration seconds within a conservative provider-neutral finite bound;
- optional output count only if contract can map deterministically; V1 may restrict to one video per operation;
- no provider credentials/raw endpoints/vendor model string.

### VideoOperation
Fields:
- opaque non-empty `ID`;
- `State`: `queued | running | succeeded | failed`;
- optional integer progress 0..100 only when provider actually supplies meaningful progress;
- optional safe stable failure category/code;
- optional created/updated timestamps if provider supplies them safely;
- optional safe usage metadata.

No raw upstream JSON/error body/signed output URL in domain operation.

### OpenVideoResult
Allowed only for succeeded operation. Returns provider-neutral binary/stream + canonical MIME/optional size/technical metadata.

Later orchestration must persist the opaque operation ID before relying on poll/retry semantics. TASK-025 itself performs no persistence.

## Generated binary abstraction
The abstraction must:
- support streaming consumption without forcing multi-MB/GB video into memory;
- expose canonical MIME and optional known size;
- be closable where resources need closing;
- preserve context cancellation where underlying transport is active;
- not expose provider SDK stream/response types.

Small image base64 responses may be decoded by an adapter before producing this abstraction, but core must enforce finite adapter/request response bounds.

## Reference binary inputs
Reference images supplied to provider ports must have:
- MIME allowlist appropriate to the capability;
- explicit size bound;
- repeatable/openable semantics if adapter transport requires retries inside one HTTP request setup;
- no object key/bucket/owner identifiers in provider types.

Authorization/loading from Media Asset occurs in a future orchestration layer before calling the port.

## Registry evolution
Extend accepted `providers.Registration`/registry in a backward-compatible way so a model declaring:
- text capability requires TextGenerator binding;
- image capability requires ImageGenerator binding;
- video capability requires VideoGenerator binding.

Resolution methods are capability-specific, for example:
- `ResolveImageGenerator`;
- `ResolveVideoGenerator`.

A model may support multiple capabilities with separate bindings.

Existing text registration/resolution behavior and tests must not regress.

## Safe errors
At provider boundary distinguish at minimum:
- provider/model/capability unavailable;
- authentication/config unavailable;
- rate-limited/transient execution failure;
- upstream terminal request rejection/invalid request;
- invalid/malformed provider response;
- failed video operation;
- result unavailable/not ready;
- context cancellation/deadline unchanged.

Errors never contain credentials, authorization headers, full prompt/reference bytes, raw provider bodies or signed URLs.

Do not add unbounded automatic retries; later durable orchestration owns retry policy.

## Deterministic fake providers
Add image/video fakes usable in unit/integration tests with:
- captured deep-cloned requests;
- configurable generated bytes/operations/errors;
- deterministic queued→running→succeeded transitions;
- cancellation behavior;
- no production registration.

## TDD gates
1. image request validation/bounds and deep immutability;
2. image response streaming/close semantics;
3. reference image MIME/size validation;
4. video start returns opaque operation;
5. poll lifecycle maps valid states/progress;
6. result unavailable before success;
7. succeeded result stream;
8. safe error mapping categories/context propagation;
9. registry resolves text/image/video independently and validates declared binding;
10. multi-capability model registration works;
11. legacy text registry behavior remains identical;
12. fakes deterministic/deep-copy requests;
13. no vendor SDK/provider schema/storage/job/http/frontend leakage;
14. race/full backend verification green.

## Scheduling
Contract is frozen and TASK-025 can be activated as soon as one implementation slot is free because its primary write surface is `apps/api/internal/providers/**` plus deterministic fakes/tests. It remains BACKLOG while WAVE-F1-H already occupies the configured three implementation slots.