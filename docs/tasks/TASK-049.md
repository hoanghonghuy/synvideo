# TASK-049 — Media Ingest Content Validation & Decoder Safety Boundary

Status: BACKLOG
Priority: P1
Milestone: Production Readiness
Issue: #106
Canonical branch when activated: `feature/TASK-049-media-ingest-validation`

## Product outcome
Creator-uploaded media becomes durable SynVideo input only after bounded server-side validation establishes that the content is structurally consistent with a supported media family. Downstream preview/render/transcode paths continue to treat media as untrusted input and execute decoders/probers within bounded resource policy.

## Evidence
Protected `develop` currently accepts these upload MIME families from the client multipart `Content-Type` header: image AVIF/GIF/JPEG/PNG/WebP; video MP4/QuickTime/WebM; audio AAC/FLAC/MPEG/MP4/Ogg/WAV. `uploadMIME` parses and allowlists the declared type, then persists that declared MIME into the durable MediaAsset record before any signature/container validation.

The upload boundary already caps bytes and metadata and the content response sets `X-Content-Type-Options: nosniff`; those controls do not establish ingest integrity for future decoder/transcoder use.

## Scope
- Introduce a bounded server-side media validation/probing boundary before successful durable ingest.
- Validate image signatures and audio/video container structure sufficiently to reject obvious spoofed, malformed, truncated, or unsupported input.
- Reconcile client-declared MIME with server-verified media family/type; client filename/extension and declared MIME are hints, not trust anchors.
- Preserve creator-supplied metadata separately from server-verified media facts where useful.
- Define stable API errors for unsupported type, malformed content, and declared/verified mismatch without parser-internal leakage.
- Bound validation by bytes/time/process resources so probing itself cannot become a denial-of-service vector.
- Require downstream preview/render/transcode integrations to execute untrusted media under a bounded decoder/process policy appropriate to the selected engine.
- Add regression coverage for valid representatives, spoofed MIME, malformed/truncated files, limit interactions, and project isolation.

## Required behavior
1. A supported declared MIME is insufficient by itself to accept durable uploaded media.
2. Obviously spoofed or structurally malformed media is rejected before a successful durable MediaAsset result is returned.
3. Validation does not read or decode unbounded data and has an explicit finite execution budget.
4. Verified media facts are not silently replaced by creator-controlled filename extension or MIME metadata.
5. Parser/prober failures map to stable application errors without exposing sensitive parser output, filesystem paths, command lines, or private media contents.
6. Validation failure leaves no successful durable application asset; any temporary object created during validation converges through the existing lifecycle/cleanup policy.
7. Downstream TASK-036/TASK-037 decoder/transcoder execution inherits the same untrusted-input assumption even for previously validated media.

## Acceptance criteria
- Upload acceptance for each currently supported media family is backed by deterministic server-side signature/container validation.
- MIME/content mismatch and malformed/truncated representatives are rejected before successful durable ingest.
- Validation byte/time/process bounds are explicit, configurable where appropriate, and covered by tests.
- Stable API error semantics distinguish unsupported media from invalid/malformed content without leaking parser internals.
- Server-verified facts can be retained independently from creator-declared metadata when needed by downstream media workflows.
- Valid image/audio/video representatives for the supported V1 families remain ingestible.
- Project isolation and existing upload byte/metadata limits remain intact.
- Existing required `Frontend`, `Backend`, and `Local Infrastructure` CI remains green.

## Non-scope
- Generic HTTP body/connection limits, owned by TASK-043.
- Temporary/intermediate retention and orphan cleanup, owned by TASK-047.
- Choosing the final editor/render engine, owned by TASK-036/TASK-037.
- Antivirus/malware scanning as a general file-security product feature.
- Expanding supported codecs/containers beyond current product requirements solely because a probe library can parse them.

## Dependencies / relations
- TASK-036 editor/preview consumes durable media and must preserve this trust boundary.
- TASK-037 render/export must use bounded decoder/transcoder execution for untrusted creator media.
- TASK-043 remains responsible for HTTP-level resource bounds.
- TASK-047 remains responsible for temporary/intermediate lifecycle convergence.
- TASK-039 may expose privacy-safe validation/decoder rejection diagnostics.

## Activation gate
Remain BACKLOG after planning freeze. Before READY, PM/TL must re-check exact `develop`, dedupe newer ingest-validation work, confirm the active supported media family list, select bounded validation/probing primitives compatible with the chosen deployment topology and TASK-036/TASK-037 media engine direction, freeze initial execution budgets/error mapping, and reconcile implementation WIP capacity. Developer owns implementation only after READY activation.

## TDD focus
Spoofed MIME, malformed/truncated media, valid representatives per media family, validation limits/timeouts, cleanup on validation failure, stable error mapping, and project isolation.
