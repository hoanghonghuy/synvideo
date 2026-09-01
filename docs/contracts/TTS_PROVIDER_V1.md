# TTS Provider Foundation V1 Contract

Status: FROZEN for planned `TASK-027`.

This contract establishes provider-neutral text-to-speech capability plus the first isolated live OpenAI speech adapter. It intentionally stops before scene-level durable audio orchestration, Media Asset persistence and creator voice UI.

## Product boundary
TASK-027 provides:
- provider-neutral speech synthesis request/response/voice types and port;
- capability-specific registry binding/resolution for accepted `providers.CapabilityTTS`;
- deterministic TTS fake;
- isolated live OpenAI `/v1/audio/speech` adapter behind the port;
- safe configured stable internal model/voice mappings;
- bounded streaming, cancellation and secret-safe error handling.

It does not:
- chunk/stitch long Scene narration;
- persist audio Media Assets;
- create durable jobs;
- bind audio to scenes;
- perform transcription/caption timing;
- modify owner credential/settings UI/runtime;
- touch frontend/httpserver/main/render/publish.

## Provider-neutral types
### Voice identity
Define stable internal `VoiceID` with conservative validation similar to ProviderID/ModelID.

`VoiceMetadata` safe fields:
- `id` stable internal voice ID;
- display name;
- optional locale/language hints;
- optional style/gender-like descriptors only when provider supplies safe factual metadata and product actually needs them;
- no provider secret/raw voice object/clone token.

Do not encode a provider-specific external voice name in public core metadata.

### SpeechSynthesisRequest
Required:
- `Text`: exact narration text, non-empty, bounded Unicode;
- `VoiceID`: stable selected voice.

Optional provider-neutral hints only when semantically portable:
- content locale/language tag;
- output audio format from a small common enum such as `mp3|wav`;
- speed/rate as a narrowly bounded normalized value only if accepted across first adapters.

V1 does not add arbitrary vendor instructions/emotion JSON to core.

The request is treated as immutable and adapters must not trim/rewrite narration except protocol-required encoding. Leading/trailing/Unicode whitespace in approved narration is preserved as supplied.

### SpeechSynthesisResponse
Contains:
- provider-neutral closable/streaming generated audio body;
- canonical MIME type;
- optional known byte size;
- selected safe model/voice metadata;
- optional safe usage/request metadata.

No provider SDK response, signed URL or credential.

## Long-input semantics
A provider adapter has an explicit finite input limit derived from its accepted config/current upstream capability.

If request text exceeds the adapter limit:
- return stable `SPEECH_INPUT_TOO_LONG`/provider request validation error;
- never truncate;
- never silently summarize/rewrite;
- never automatically split inside the adapter.

A later scene-audio orchestration task owns deterministic chunking at safe textual boundaries, synthesis of every chunk, ordered stitching and timing while preserving the entire approved narration.

## Registry evolution
Build on the accepted multi-capability registration shape after TASK-025.

A model declaring `CapabilityTTS` requires a `SpeechSynthesizer` binding. Add capability-specific resolver such as `ResolveSpeechSynthesizer`.

Models may support text/image/video/TTS combinations without one binding satisfying another capability.

Existing text/image/video resolution remains unchanged.

## Deterministic fake
Fake synthesizer supports:
- configured voices;
- captured deep-cloned exact text request;
- deterministic audio bytes/MIME;
- configurable errors;
- cancellation;
- proof that long-input validation never truncates.

No production fake registration.

# OpenAI Speech Adapter V1

## Current upstream boundary
Use the currently supported OpenAI speech generation endpoint `/v1/audio/speech` through direct HTTP/infrastructure transport.

External model IDs and external voice strings are injected through adapter config and mapped from stable internal IDs. Do not hard-code a creator-visible dependency on one current model/voice alias.

Implementation tests use local `httptest`; CI never calls public OpenAI.

## Adapter configuration
Includes:
- stable ProviderID;
- internal ModelID -> external model mapping;
- internal VoiceID -> external OpenAI voice mapping;
- exact API key;
- canonical/test base URL;
- allowed output format(s);
- maximum input Unicode/rune/byte bound matching the chosen upstream model contract conservatively;
- HTTP timeout;
- maximum response audio bytes.

Validation:
- exact API key required and never silently trimmed;
- HTTPS except validated loopback test target;
- base URL rejects user-info/query/fragment/unsafe redirects;
- mappings non-empty/unique/canonical;
- input/response/time bounds finite positive.

## Request translation
Map only accepted semantics:
- exact text -> upstream input;
- external model;
- external voice;
- configured/requested supported response format;
- speed only if core request includes it and upstream equivalence is documented.

Do not invent unsupported emotion/instruction behavior.

## Streaming/resource behavior
Prefer streaming upstream audio directly into provider-neutral response rather than buffering the full audio response.

Requirements:
- check HTTP status/content type before exposing success;
- wrap body with a hard maximum byte reader that fails safely if exceeded;
- returned body close always closes upstream response;
- context cancellation/deadline interrupts read;
- no unbounded retry;
- redirects blocked/revalidated; Authorization never forwarded to another origin.

If upstream requires response buffering for a specific format, enforce the same finite maximum and document/test it.

## Safe errors
At minimum:
- invalid internal model/voice -> provider unavailable/unsupported;
- text too long -> terminal speech input validation;
- `401/403` -> authentication/config unavailable;
- `429` -> rate-limited/transient;
- `5xx` -> transient execution failure;
- other request `4xx` -> safe terminal provider rejection where appropriate;
- unexpected MIME/empty body/oversize -> invalid provider response;
- context cancellation/deadline unchanged.

Never return raw upstream error body, prompt/narration sentinel, API key or Authorization header.

## Secret and narration privacy tests
Tests prove:
- exact expected credential reaches only local fake upstream;
- API key absent from errors/metadata;
- raw upstream secret sentinel absent from errors;
- request narration is not included in generic adapter errors/loggable metadata;
- over-limit narration returns validation without making network request;
- narration bytes/text are transmitted exactly when valid, not trimmed/truncated.

## TDD gates
1. VoiceID/voice metadata validation and clone safety;
2. exact request immutability/text preservation;
3. TTS registry binding/resolution + legacy capability regressions;
4. fake deterministic audio/error/cancel behavior;
5. OpenAI adapter config/base URL/secret/model/voice validation;
6. exact model/voice/text request mapping;
7. streaming audio success + close/cancel;
8. supported MIME/format mapping;
9. input too long rejects before network and never truncates;
10. response byte bound;
11. 401/403/429/4xx/5xx + malformed success safe mapping;
12. redirect/credential safety;
13. no persistent Media/job/frontend/runtime/transcription scope leakage;
14. race/full backend verification green.

## Scheduling
TASK-027 depends on accepted TASK-025 registry evolution. After TASK-025 merges, TASK-026 and TASK-027 can normally run in parallel because their live adapter packages are isolated; TASK-027 only owns the minimum TTS-specific registry extension agreed by the frozen provider contract.