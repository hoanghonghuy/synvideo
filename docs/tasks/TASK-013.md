# TASK-013 — Live OpenAI-compatible text provider adapter foundation

Status: DONE
Milestone: F1 Creative Workflow
Depends on: TASK-005 accepted; frozen `OPENAI_COMPAT_TEXT_PROVIDER_V1`
Wave: WAVE-F1-E
Branch: `feature/TASK-013-openai-compatible-provider`
Base: `develop`
Accepted PR: #29
Squash merge: `177e78fc27a8af2990c2263b689e3d41a1785499`

## Acceptance record
Team Lead logical APPROVE on head `9a3d0ca8` with CI #142 green.

Accepted behavior:
- isolated `apps/api/internal/providers/openaicompat/**` scope;
- OpenAI-compatible chat-completions adapter implements accepted `providers.TextGenerator`;
- stable internal provider/model IDs map to configured external model IDs;
- credential is supplied only at request time and never exposed through metadata/presentation errors;
- redirects are disabled to prevent credential forwarding;
- timeout/response size are bounded and there is no internal retry loop;
- auth/rate-limit/5xx/malformed responses are safely classified;
- context cancellation/deadline propagates;
- deterministic `httptest` coverage, race/full CI and no real CI network.

## Goal
Implement the first live-network text-generation adapter behind the accepted provider-neutral capability using the frozen OpenAI-compatible HTTP contract.

This task is infrastructure/provider-boundary work. It intentionally does not own creator BYOK settings or application runtime wiring so it can run in parallel with TASK-009 without touching the same composition files.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/contracts/OPENAI_COMPAT_TEXT_PROVIDER_V1.md`
- accepted `apps/api/internal/providers/**` contracts/tests.

## Frozen contract
`OPENAI_COMPAT_TEXT_PROVIDER_V1.md` is authoritative.

## Primary ownership
- `apps/api/internal/providers/openaicompat/**`;
- adapter-local test fixtures/helpers only.

## Reserved / do not touch
- `apps/api/cmd/api/main.go` and runtime server composition — TASK-009/current PM wave;
- `apps/api/internal/httpserver/**`;
- `apps/web/**`;
- generic jobs/persistence/migrations;
- Proposal/Script/Scene Plan packages;
- secure owner credential storage/settings UI (future BYOK runtime task).

## Scope
- OpenAI-compatible chat-completions HTTP client implementing accepted `providers.TextGenerator`;
- stable internal provider/model IDs mapped to configurable external model identifiers;
- bearer credential supplied only through injected adapter configuration/secret source;
- deterministic `providers.Registration` factory for configured models;
- context-bound requests and safe timeout/response-size limits;
- strict success-response extraction and usage mapping where supported;
- safe error classification for auth/config, rate-limit/transient server errors, malformed responses and context cancellation;
- secret-free errors/metadata;
- local `httptest` coverage; no external network in CI.

## TDD plan
Truthful RED -> GREEN -> REFACTOR covers at least:
1. request method/path/model/messages mapping;
2. bearer auth arrives at fake upstream;
3. stable internal provider/model IDs are returned even when external model ID differs;
4. valid assistant text response succeeds;
5. empty/malformed/unknown response fails safely;
6. `401/403`, `429`, `5xx` classifications;
7. in-flight cancel/deadline propagation;
8. bounded response body rejection;
9. API key never appears in error strings/metadata;
10. registration factory capability/model metadata correctness;
11. duplicate/invalid config rejection;
12. no vendor SDK/core-domain leakage.

## Acceptance criteria
- [x] Implements frozen OpenAI-compatible provider contract.
- [x] Uses only accepted provider-neutral interfaces outside the adapter package.
- [x] No credential can escape through request domain types or safe errors.
- [x] No unbounded internal retry; durable retry remains a higher-layer concern.
- [x] Context cancellation/deadline and resource bounds are proven.
- [x] Deterministic local upstream tests, gofmt/vet/race/tests/build/full CI green.
- [x] No main/httpserver/web/jobs/persistence scope leakage.
- [x] TDD evidence truthful.

## Follow-on
A later secure BYOK/runtime task will store/manage creator credentials, expose provider settings and register this adapter into production runtime. TASK-009 may safely merge before that with an empty production provider catalog.

## Worktree cleanup
The TASK-013 implementation worktree should be removed after confirming the merged commit before this developer claims another task.
