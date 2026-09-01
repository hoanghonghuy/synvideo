# TASK-017 — Secure BYOK text provider settings and owner-scoped runtime

Status: READY
Milestone: F1 Creative Workflow
Depends on: TASK-005, TASK-009 and TASK-013 accepted; frozen `BYOK_TEXT_PROVIDER_RUNTIME_V1`
Wave: WAVE-F1-G
Branch: `feature/TASK-017-byok-provider-runtime`
Base: `develop`

## Goal
Make AI Proposal generation genuinely usable with live creator-provided API credentials by adding secure owner-scoped BYOK settings, encrypted-at-rest credential persistence, safe provider/model settings UI, and runtime resolution of the accepted OpenAI-compatible adapter.

This task closes the current gap where TASK-009 is production-safe with an empty live provider catalog but creators cannot yet configure a real provider.

## Authoritative contract
`docs/contracts/BYOK_TEXT_PROVIDER_RUNTIME_V1.md`

Read first:
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/decisions/0005-technical-baseline.md`
- `docs/contracts/OPENAI_COMPAT_TEXT_PROVIDER_V1.md`
- `docs/contracts/AI_PROPOSAL_JOB_V1.md`
- `docs/contracts/BYOK_TEXT_PROVIDER_RUNTIME_V1.md`
- accepted TASK-005 provider registry/types;
- accepted TASK-009 Proposal generation service/handler/UI;
- accepted TASK-013 `providers/openaicompat` adapter.

## Primary ownership
- new cohesive owner provider-settings/runtime package under `apps/api/internal/**`;
- migration **`0009_create_text_provider_settings.sql`**;
- provider-settings PostgreSQL repository;
- credential encryption boundary;
- deployment provider-definition configuration loader;
- targeted owner-scoped runtime integration into TASK-009 Proposal generation;
- `apps/api/internal/httpserver/**` routes/handlers needed for provider settings and owner-scoped generation options;
- `apps/api/cmd/api/main.go` runtime composition;
- creator-facing AI Provider settings workspace under `apps/web/**`;
- task-specific backend/frontend/PostgreSQL/security tests.

## Mandatory isolation
Do not touch:
- `apps/api/internal/sceneplan/**` or migration `0007` / TASK-015 paths;
- `apps/api/internal/mediaasset/**`, storage adapter or migration `0008` / TASK-016 paths;
- Script generation integration/workspace beyond generic reusable provider-runtime interfaces;
- render/publish/media generation;
- deterministic fake provider production registration.

TASK-015 and TASK-016 are concurrently in review-fix mode and their write surfaces must remain isolated.

## Scope
### Secure persisted owner settings
- one owner/provider setting with optimistic `revision`;
- enabled/disabled state;
- enabled internal model IDs;
- encrypted API key only, never plaintext database storage;
- create/update/rotate/preserve/delete credential lifecycle;
- owner isolation and stale revision protection;
- migration `0009_create_text_provider_settings.sql` only.

### Encryption boundary
- standard authenticated encryption (AES-256-GCM or equivalent accepted standard-library primitive);
- master key only from server-side environment/config;
- fresh cryptographic nonce on every encryption;
- AAD binds owner/provider/key version;
- tamper-safe failure;
- no plaintext/ciphertext/key leaks in presentation errors/loggable values;
- missing/invalid master key disables BYOK safely without forcing plaintext fallback.

### Deployment provider definitions
- deterministic non-secret catalog from validated server configuration;
- stable provider/model IDs, display names, base URL, external model mapping and safe bounds;
- creator cannot submit arbitrary base URLs/external model IDs in V1;
- reuse TASK-013 validation/adapter; no second OpenAI-compatible implementation.

### HTTP API
Implement frozen routes:
- `GET /api/v1/ai/provider-settings`;
- `PUT /api/v1/ai/provider-settings/{provider_id}`;
- `DELETE /api/v1/ai/provider-settings/{provider_id}?revision=...`;
- make existing `GET /api/v1/ai/text-generation-options` principal-aware and owner-scoped.

All responses are safe views only: no API key fragments, ciphertext, nonce, master key, base URL or external model ID.

### Owner-scoped runtime
- cohesive runtime/resolver lists owner-enabled models and resolves an owner/provider/model to accepted provider-neutral `TextGenerator`;
- per-owner secret must never be installed into the global static registry;
- Proposal generation creation validates through owner runtime;
- Proposal generation worker resolves current owner secret from `job.owner_id` at execution and invokes TASK-013 adapter;
- durable payload/result remain credential-free;
- existing `request_id` replay/conflict semantics stay intact even if current credential configuration changed after initial enqueue.

### Creator UI
- localized AI Provider settings page/workspace;
- configure API key, provider enable state and enabled models;
- existing stored key is represented only as `has_api_key`, never masked/revealed;
- update may preserve existing key without re-entry;
- explicit key replacement and explicit delete confirmation;
- clear secret input from component state after save;
- never put secret into route/localStorage/sessionStorage;
- refresh Proposal generation options after setting changes;
- loading/saved/stale/validation/network states follow existing UX conventions.

## TDD plan
Truthful RED → GREEN → REFACTOR must cover at least:
1. encryption/decryption round-trip with different random nonce per write;
2. AAD owner/provider binding and tamper rejection;
3. plaintext sentinel absent from DB row/HTTP JSON/errors;
4. invalid/missing deployment master key fails BYOK closed;
5. provider-definition validation, duplicate IDs and no fake registration;
6. first configure requires API key; update may preserve; rotate replaces; delete removes;
7. stale same-revision concurrent update gives one success and stale competitors;
8. owner A cannot list/update/delete/resolve owner B credential;
9. enabled model selection validated against deployment catalog;
10. owner generation options contain only configured+enabled selected models;
11. Proposal generation local `httptest` smoke proves owner A key reaches fake upstream and owner B key does not;
12. queued job has no secret and resolves credential at worker execution;
13. credential missing/disabled at execution maps to safe retryable provider-unavailable behavior;
14. existing request-ID replay returns durable job before current provider/credential availability can invalidate it;
15. frontend secret input clears after save and is never client-persisted;
16. settings UI stale/error/delete confirmations regressions;
17. real PostgreSQL, `go test -race ./...`, frontend tests/build and full `make verify` green with no real provider network.

## Acceptance criteria
- [ ] Frozen `BYOK_TEXT_PROVIDER_RUNTIME_V1` implemented without drift.
- [ ] Migration is exactly `0009_create_text_provider_settings.sql` and independent of TASK-015/016 new tables.
- [ ] API keys are encrypted at rest and never returned after submission.
- [ ] Owner isolation holds at repository/service/runtime/HTTP boundaries.
- [ ] No arbitrary creator base URL in V1; no SSRF surface introduced by settings API.
- [ ] TASK-013 adapter is reused rather than duplicated.
- [ ] Proposal generation options and worker execution are truly owner-scoped.
- [ ] Durable jobs remain secret-free and request-ID replay behavior is preserved.
- [ ] Creator can configure/disable/rotate/delete a live provider through localized UI.
- [ ] No Scene Plan/media/Script/render/publish scope leakage.
- [ ] TDD/security evidence is truthful and full CI is green.

## Worktree
Atomically create the absent remote branch ref `feature/TASK-017-byok-provider-runtime` from the latest `origin/develop`, then use a dedicated TASK-017 worktree. The shared/control checkout stays on `develop`.

Before claiming the branch, confirm it does not already exist remotely. Do not use a plain same-base push as an exclusive lock.

Do not self-merge or self-mark DONE.

## Follow-on
After TASK-017 is accepted, the next shared runtime task should integrate durable Script generation using TASK-012 + generic jobs + this same owner-scoped text-provider runtime. Do not build a second credential/settings path for Script.