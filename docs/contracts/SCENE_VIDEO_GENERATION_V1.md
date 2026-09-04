# SCENE_VIDEO_GENERATION_V1

Status: ACCEPTED — implemented by TASK-032 / PR #89
Applies to: TASK-032

## Purpose
Define the provider-neutral contract for durable per-scene AI video generation without binding SynVideo's domain model to a vendor/model that can be renamed or deprecated.

## Product invariant
For one immutable approved-scene snapshot, SynVideo must create at most one logical paid upstream generation attempt unless the user explicitly starts a new alternative. Worker/browser restarts and retries must resume the same durable provider operation rather than blindly submit again.

## Request snapshot
A generation request durably snapshots, at minimum:
- project / scene-plan version / scene key;
- approved scene intent/prompt inputs used for the generation;
- provider adapter identifier and selected model/config identifier as opaque configuration values;
- optional durable reference MediaAsset identities;
- requested aspect ratio/duration/quality controls supported by the selected adapter;
- actor/owner identity and request idempotency identity.

The snapshot is immutable for the lifetime of the logical generation. Editing the scene later does not mutate an in-flight job.

## Provider operation lifecycle
Adapters expose two conceptual phases:
1. `Submit`: create the upstream generation and return a provider-owned opaque operation/task identifier.
2. `Retrieve`: query the existing operation by that identifier until a terminal state is reached.

The external operation identity must be persisted durably before subsequent poll/download work is considered recoverable.

### Ambiguous submit outcome
If network/timeout failure occurs after a submit may have reached the provider but before SynVideo safely persisted the external operation identity, the job must enter an explicit ambiguous/manual-safe recovery state or use a provider-supported idempotency/recovery mechanism. It must not automatically issue another paid submit merely because the client did not receive the first response.

## Internal states
Provider states map into safe internal lifecycle states such as:
- queued/submitting;
- running/polling;
- succeeded-acquiring;
- succeeded;
- retryable failure;
- terminal failure;
- canceled when supported;
- ambiguous submit/recovery required.

Raw provider payloads/errors must not leak credentials, secrets, signed URLs, or sensitive request metadata to public API responses.

## Polling and retry
- Polling survives process restarts and always reuses the persisted external operation identity.
- Polling uses bounded backoff with jitter and honors provider rate-limit/retry guidance where available.
- Non-terminal transport/provider errors may be retried without creating a new upstream generation.
- Terminal provider failure remains attached to the same logical generation and may be exposed through safe normalized error semantics.

## Output acquisition
A provider-completed video is not considered a durable SynVideo result until the output has been acquired into SynVideo-managed MediaAsset/object storage with provenance sufficient to trace:
- logical generation;
- provider adapter + opaque model/config identifier;
- external operation identity;
- source scene snapshot;
- acquisition time and relevant output metadata.

Temporary provider URLs must not become the durable scene-media reference.

## Scene assignment / alternatives
Generated video MediaAssets are alternatives. Assignment/replacement of the active scene visual is an explicit idempotent action through the existing scene-media binding semantics. Generation completion must not silently replace an existing selected visual unless the frozen product contract explicitly says so.

## Adapter constraints
The domain contract must not hard-code today's model names. Initial V1 adapter candidate: Runway, because its current official API exposes durable asynchronous task IDs and retrieve/poll semantics. Google Veo 3.1 is a validated reference for the same long-running-operation pattern and a possible follow-on adapter.

## Required TDD / integration evidence
Tests must demonstrate at least:
1. immutable source snapshot;
2. idempotent logical submit under normal retry;
3. external operation ID persisted and reused after worker restart;
4. polling/retrieve never creates a second paid operation;
5. ambiguous submit outcome does not blindly resubmit;
6. provider terminal/retryable status mapping;
7. completed output acquired into managed MediaAsset storage;
8. refresh/restart recovery from durable state;
9. explicit scene assignment and history/idempotency;
10. owner/project isolation and safe error/provenance exposure.

## Provider research checkpoint — 2026-09-03
- Runway Dev official SDK/API documents generation as asynchronous tasks returning an `id`, retrievable via `GET /v1/tasks/{id}`, with terminal statuses and polling/backoff guidance.
- Google Gemini API Veo 3.1 documents long-running video operations with an operation name retrieved/polled until completion; older Veo 3 model IDs are deprecated, validating the decision not to freeze model names into SynVideo's domain contract.

Official references:
- https://docs.dev.runwayml.com/api-details/sdks/
- https://docs.dev.runwayml.com/api-details/api_changelog/
- https://ai.google.dev/gemini-api/docs/veo

## Acceptance record — 2026-09-04
TASK-032 is DONE. PR #89 passed exact-head required CI and was squash-merged to protected `develop` as `7b3569bd09d8a35ad7a57363eaccfbf4eb6d545c`. Downstream planning may treat this contract/implementation boundary as accepted unless a later explicit amendment supersedes it.
