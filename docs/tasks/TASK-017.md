# TASK-017 — Secure BYOK text provider settings and owner-scoped runtime

Status: DONE
Milestone: F1 Creative Workflow
Depends on: TASK-005, TASK-009 and TASK-013 accepted; frozen `BYOK_TEXT_PROVIDER_RUNTIME_V1`
Wave: WAVE-F1-G
Branch: `feature/TASK-017-byok-provider-runtime`
Base: `develop`
Accepted PR: #35
Accepted head: `8dd10cae4b7e665719d5087f75fa4df1845b73e1`
Logical Team Lead APPROVE review: `5078306924`
CI: #177 green
Merge commit: `6fbfdbc032225de1d4b1a3f89bf00a103935bbe7`

## Goal
Make AI Proposal generation genuinely usable with live creator-provided API credentials by adding secure owner-scoped BYOK settings, encrypted-at-rest credential persistence, safe provider/model settings UI, and runtime resolution of the accepted OpenAI-compatible adapter.

This closes the gap where TASK-009 was production-safe but creators could not yet configure a real live provider.

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
- cohesive owner provider-settings/runtime package under `apps/api/internal/**`;
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
Did not reopen accepted TASK-015 Scene Plan or TASK-016 media foundations, and did not introduce Script/render/publish implementation scope. Production does not register the deterministic fake provider.

## Accepted scope
### Secure persisted owner settings
- one owner/provider setting with optimistic `revision`;
- enabled/disabled state and enabled internal model IDs;
- encrypted API key only, never plaintext database storage;
- create/update/rotate/preserve/delete credential lifecycle;
- owner isolation and stale revision protection;
- migration `0009_create_text_provider_settings.sql` only.

### Encryption boundary
- AES-256-GCM authenticated encryption;
- master key only from server-side environment/config;
- fresh cryptographic nonce on each encryption;
- AAD binds owner/provider/key version;
- tamper-safe failure;
- no plaintext/ciphertext/key leaks in presentation errors/loggable values;
- missing/invalid master key disables BYOK safely without plaintext fallback.

### Deployment provider definitions
- deterministic non-secret catalog from validated server configuration;
- stable provider/model IDs, display names, base URL, external model mapping and safe bounds;
- creator cannot submit arbitrary base URLs/external model IDs in V1;
- TASK-013 URL validation and adapter are reused;
- negative timeout/max-response-size and non-canonical external model IDs fail catalog validation;
- explicitly supplied invalid `SYNVIDEO_TEXT_PROVIDER_DEFINITIONS` fails startup rather than silently disabling BYOK.

### HTTP API
Accepted routes:
- `GET /api/v1/ai/provider-settings`;
- `PUT /api/v1/ai/provider-settings/{provider_id}`;
- `DELETE /api/v1/ai/provider-settings/{provider_id}?revision=...`;
- principal-aware owner-scoped `GET /api/v1/ai/text-generation-options`.

Responses expose safe views only: no API key fragments, ciphertext, nonce, master key, base URL or external model ID.

### Owner-scoped runtime
- lists owner-enabled models and resolves owner/provider/model to provider-neutral `TextGenerator`;
- per-owner secrets are not installed into the global static registry;
- Proposal generation creation validates through owner runtime;
- Proposal generation worker resolves the current owner secret from `job.owner_id` at execution and invokes TASK-013 adapter;
- durable payload/result remain credential-free;
- request-ID replay behavior remains durable across later credential/config changes.

### Creator UI
- localized AI Provider settings workspace;
- configure API key, provider enable state and enabled models;
- stored key represented only through `has_api_key`;
- existing key can be preserved without re-entry;
- explicit replacement/delete flows;
- secret input cleared from component state after successful save;
- no secret persistence in route/localStorage/sessionStorage;
- Proposal generation options reflect owner settings;
- loading/saved/stale/validation/network states covered.

## TDD / verification evidence
Accepted coverage includes:
1. encryption/decryption round-trip with random nonces;
2. AAD owner/provider binding and tamper rejection;
3. plaintext sentinel absent from DB row/HTTP JSON/errors;
4. missing/invalid master key fails BYOK closed;
5. provider-definition validation, duplicate IDs and no fake production registration;
6. first configure/update/rotate/preserve/delete semantics;
7. real PostgreSQL concurrent same-revision updates with exactly one winner;
8. owner isolation for settings and runtime resolution;
9. enabled model selection validation;
10. owner-specific generation options;
11. Proposal generation job E2E through local `httptest` upstream proving Owner A/B Authorization isolation;
12. durable job payload/result remain secret-free;
13. credential unavailable at execution maps to safe retryable provider-unavailable behavior;
14. request-ID replay remains durable;
15. frontend secret clearing and no client persistence;
16. settings UI stale/error/delete regressions;
17. catalog runtime-equivalent bounds validation and explicit invalid-definition startup fail-fast;
18. CI #177 green on accepted head.

## Acceptance criteria
- [x] Frozen `BYOK_TEXT_PROVIDER_RUNTIME_V1` implemented without blocking drift.
- [x] Migration is exactly `0009_create_text_provider_settings.sql` and independent of TASK-015/016 new tables.
- [x] API keys are encrypted at rest and never returned after submission.
- [x] Owner isolation holds at repository/service/runtime/HTTP boundaries.
- [x] No arbitrary creator base URL in V1; no settings-API SSRF surface introduced.
- [x] TASK-013 adapter is reused rather than duplicated.
- [x] Proposal generation options and worker execution are truly owner-scoped.
- [x] Durable jobs remain secret-free and request-ID replay behavior is preserved.
- [x] Creator can configure/disable/rotate/delete a live provider through localized UI.
- [x] No Scene Plan/media/Script/render/publish scope leakage.
- [x] TDD/security evidence and full CI are green.

## Team Lead acceptance
Final review accepted head `8dd10cae4b7e665719d5087f75fa4df1845b73e1` after all prior security/runtime gates and the final deployment configuration gate were closed. Logical APPROVE review `5078306924`; squash merge commit `6fbfdbc032225de1d4b1a3f89bf00a103935bbe7`.

## Worktree
Merged worktree should be cleaned before the developer claims another task. Shared control checkout remains on `develop`.

## Follow-on
Script and Scene Plan durable generation must reuse this owner-scoped provider runtime rather than creating a second credential/settings path.