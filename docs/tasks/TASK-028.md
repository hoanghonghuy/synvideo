# TASK-028 — Secure multi-capability provider runtime and settings

Status: IN_PROGRESS
Milestone: F1 Creative Workflow
Owner role: AI Developer
PR target: develop
Branch: `feature/TASK-028-multicap-provider-runtime`
Depends on: TASK-017, TASK-024, TASK-025, TASK-026 and TASK-027 — all complete.
Issue: #54
Claim state: canonical remote branch exists; continue that branch/worktree only.

## Goal
Evolve the accepted text-only BYOK owner runtime into one secure capability-aware provider configuration/runtime so the same owner credential and safe deployment catalog can expose text, image and TTS models/voices without duplicating secret storage or creating vendor-specific domain paths.

## Why
TASK-026 and TASK-027 provide live adapters, but production creator flows cannot safely resolve those adapters per owner until the accepted TASK-017 BYOK/runtime model is generalized. Durable per-scene generation must not invent a second credential path.

## Read first
- `AGENTS.md`
- `docs/engineering/CONTROL_PLANE_PROTOCOL.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/contracts/BYOK_TEXT_PROVIDER_RUNTIME_V1.md`
- `docs/contracts/VISUAL_GENERATION_PROVIDER_V1.md`
- `docs/contracts/OPENAI_IMAGE_PROVIDER_V1.md`
- `docs/contracts/TTS_PROVIDER_V1.md`
- `docs/contracts/MULTI_CAPABILITY_PROVIDER_RUNTIME_V1.md`
- accepted TASK-017 implementation and migrations.

## Frozen integration contract
`docs/contracts/MULTI_CAPABILITY_PROVIDER_RUNTIME_V1.md`

The contract preserves TASK-017 security/backward compatibility while adding owner-scoped text/image/TTS settings, safe option surfaces and runtime resolution. Implementation must not weaken the frozen guarantees.

## Parallel safety
### Primary write paths
- owner provider settings/runtime/catalog packages under `apps/api/internal/**`;
- narrowly scoped provider-settings HTTP/runtime composition;
- AI Provider settings frontend workspace and task-specific tests.

### Allowed shared integration files
- `apps/api/cmd/api/main.go` only for final runtime composition;
- existing provider-settings routes/types only through backwards-compatible evolution;
- localized settings route/components as required.

### Reserved / do not touch
- durable generation jobs, Media Asset ingestion/binding, captions/render/publish;
- vendor adapters except minimal construction through their public configuration/factory boundary.

## Scope
- generalize deployment provider definitions from text-only to capability-aware model/voice catalog entries;
- reuse one encrypted owner/provider credential when an upstream credential legitimately spans capabilities;
- persist owner enablement/selection for safe internal text/image/TTS model IDs and TTS voice IDs without exposing external IDs;
- capability-aware owner runtime resolvers for accepted TextGenerator, ImageGenerator and SpeechSynthesizer ports;
- safe owner-scoped generation-option endpoints/views for image and TTS while keeping existing text options compatible;
- extend creator AI Provider settings UI to configure capability/model/voice enablement without ever returning stored secret material;
- preserve credential rotate/preserve/delete, optimistic revision, owner isolation and fail-closed encryption behavior.

## Out of scope
- video-provider activation/research implementation;
- per-scene image/TTS generation jobs;
- Media Asset persistence or scene binding;
- audio chunk/stitch/timing;
- captions/music/editor/render/publish.

## Required behavior
- existing text BYOK users migrate without credential loss or forced plaintext re-entry;
- no API key/ciphertext/base URL/external model/voice identity appears in public responses or durable jobs;
- provider/model/voice selection is validated against deployment-controlled catalog definitions;
- disabled capability/model/voice cannot resolve at runtime;
- owner A can never resolve owner B credentials/settings;
- invalid capability catalog fails startup rather than silently falling back;
- text Proposal/Script/Scene Plan generation remains backward-compatible;
- image/TTS options expose only safe internal IDs/display metadata needed by future creator flows.

## TDD plan
1. migration/backward-compatibility tests from existing text settings;
2. encryption/owner-isolation regressions remain green;
3. capability-aware catalog validation and duplicate-ID rejection;
4. image/TTS model + voice selection validation;
5. owner runtime resolves correct adapter with owner-specific secret and rejects disabled selections;
6. safe option payloads contain no secret/external-ID sentinel;
7. existing text generation E2E remains green;
8. settings UI preserves/rotates/deletes secret and edits capability selections without client persistence;
9. race/full backend + frontend verification.

## Acceptance criteria
- [x] PM/TL froze `MULTI_CAPABILITY_PROVIDER_RUNTIME_V1` before READY.
- [ ] Existing TASK-017 text configuration/data continues to work after migration.
- [ ] One encrypted owner/provider credential can safely serve multiple enabled capabilities where supported.
- [ ] Text/image/TTS runtime resolution is owner-scoped and provider-neutral.
- [ ] Public settings/options never leak credentials, ciphertext or external IDs.
- [ ] No durable generation/media orchestration scope leaks into this task.
- [ ] Full security/backward-compatibility/TDD evidence and CI are green.

## Constraints
- never weaken AES-GCM/AAD/fail-closed behavior from TASK-017;
- no arbitrary creator-supplied provider base URL in this version;
- capability metadata stays provider-neutral; vendor schema remains in adapters;
- schema changes must have an explicit backward migration/data-preservation story.

## Verification
- focused provider settings/runtime tests;
- real PostgreSQL migration/integration coverage;
- `go test -race ./...` and full API verify;
- frontend verify for settings workspace;
- secret sentinel checks across DB/API/error/loggable metadata;
- fresh PR CI.

## Claim / execution state
This task was activated only after its dependencies/contracts were satisfied. The canonical remote branch `feature/TASK-028-multicap-provider-runtime` now exists and is the live claim. It must **not** be claimed again or replaced merely because a local checkout/BOARD mirror is stale or because no PR exists yet.

Continue only the existing canonical branch/worktree. Before further implementation or delivery, re-check the live issue and current remote contract under `CONTROL_PLANE_PROTOCOL.md`. Do not self-mark DONE or self-merge.
