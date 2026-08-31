# TASK-005 — AI provider capability and text-generation contracts

Status: BLOCKED
Milestone: F1 Creative Workflow
Depends on: TASK-001 accepted
Wave gate: open with WAVE-F1-A after TASK-002 acceptance
Branch: `feature/TASK-005-ai-provider-contracts`
Base: `develop`
Wave: WAVE-F1-A

## Goal
Establish a provider-neutral AI boundary that the future AI Proposal workflow can depend on without coupling SynVideo domain/application logic to OpenAI, Gemini, Anthropic or any other provider.

## Why
The next creative stages need AI text generation, but provider lock-in must not leak into core domain semantics. This task intentionally creates the boundary and tests first, not live vendor adapters.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/engineering/ARCHITECTURE_PRINCIPLES.md`
- `docs/decisions/0002-provider-abstraction.md`
- relevant provider section of `docs/research/OPEN_SOURCE_REFERENCES.md` only if needed for interface research

Do not recursively load unrelated docs.

## Integration contract
Provides an internal provider-neutral capability/registry contract for future AI Proposal work.

This task does not expose a public HTTP API and does not implement a real external provider adapter.

## Parallel safety
### Primary write paths
- `apps/api/internal/providers/**` or the equivalent dedicated provider-boundary package chosen by the accepted Go structure.
- Provider-contract-specific tests in that package.

### Allowed shared integration files
- None by default.
- A minimal package-level documentation/reference update if required.

### Reserved / do not touch
- Creative Brief backend/domain/migrations owned by TASK-003.
- `apps/web/**` owned by TASK-004.
- HTTP router/server composition, project persistence and Creative Brief API wiring.
- Global config/env/secret plumbing for real vendors.

The task should be merge-order independent from TASK-003/TASK-004 because it has no runtime integration with them in this wave.

## Scope
- Stable provider identity type separate from model identity.
- Capability representation that can express at least text generation now and extend later to image generation, video generation, TTS, transcription and music without redesigning core provider identity.
- Provider/model metadata contract sufficient for later capability discovery (for example provider id, model id, display metadata, supported capabilities).
- A typed text-generation boundary appropriate for future Creative Proposal generation.
- Request/response types that carry provider-neutral inputs/outputs and usage metadata without embedding one vendor's raw schema into domain code.
- Registry/catalog behavior to register and resolve available provider/model implementations by stable ids/capabilities.
- Deterministic fakes/stubs for tests and future application-service tests.
- Explicit error categories for unsupported capability/model, provider unavailable and provider execution failure without exposing secrets/vendor raw payloads as core errors.

## Out of scope
- Live OpenAI/Gemini/Anthropic/etc. adapters.
- BYOK credential persistence/encryption UI or API.
- HTTP endpoints for provider management.
- Billing/credits/cost charging.
- AI Proposal prompt/schema implementation.
- Image/video/TTS/transcription/music execution interfaces beyond capability extensibility required by this task.
- Database schema unless PM explicitly approves a later persistence task.

## Required behavior
- Core/application code can depend on provider-neutral interfaces/types without importing a vendor SDK.
- Provider and model identity are explicit and not represented by free-form magic strings scattered across callers.
- Registry rejects duplicate conflicting registrations deterministically.
- Resolution can distinguish unknown provider/model from known-but-unsupported capability.
- Context cancellation is part of the execution boundary.
- Provider errors are wrapped/mapped into stable internal categories while retaining safe diagnostic cause internally.
- No API keys/secrets are accepted, logged or persisted by this task.
- Design must allow a later adapter to report token/usage metadata without making billing assumptions mandatory.

## TDD plan
Start with RED unit/contract tests before implementation, including at minimum:
1. successful provider/model registration and resolution by text capability;
2. duplicate/conflicting registration rejection;
3. unknown provider/model resolution error;
4. unsupported capability error distinct from unknown provider;
5. fake text provider receives provider-neutral request and returns deterministic response/usage;
6. context cancellation propagates through text generation boundary;
7. provider execution error maps to stable internal category without leaking a secret-bearing raw message to presentation-facing error text.

No live network calls in CI.

## Acceptance criteria
- [ ] Provider/model/capability types are provider-neutral and documented in code.
- [ ] Typed text-generation boundary exists and is context-aware.
- [ ] Registry/catalog behavior is deterministic and covered by tests.
- [ ] Stable error categories exist for core failure modes.
- [ ] Deterministic fake implementation is available for future application tests.
- [ ] No vendor SDK, API key persistence, public HTTP API or billing scope is introduced.
- [ ] Package remains isolated from TASK-003/TASK-004 write surfaces.
- [ ] PR contains TDD evidence and backend verification is green.

## Open-source research
Consult only if needed for interface/capability ideas. Do not copy code without license verification. This task should normally be implemented from SynVideo's own product/architecture requirements.

## Verification
At minimum:
- targeted provider package tests;
- race-safe/concurrency tests if registry implementation is mutable/concurrent;
- `gofmt`;
- `go vet ./...`;
- `go test ./...`;
- backend build;
- `git diff --check` or equivalent.

## Delivery
PR to `develop` from `feature/TASK-005-ai-provider-contracts` must include:
- interface/capability design summary;
- TDD evidence: RED/GREEN/REFACTOR;
- error/registry behavior verification;
- confirmation that no vendor SDK/live provider/secret persistence was introduced;
- commands/results and follow-up findings.

Do not self-merge or mark DONE.
