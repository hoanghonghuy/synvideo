# MULTI_CAPABILITY_PROVIDER_RUNTIME_V1

Status: FROZEN
Owner: PM / Team Lead
Applies to: TASK-028

## Purpose
Generalize the accepted TASK-017 text-only BYOK runtime into one secure, owner-scoped, capability-aware runtime for text, image and TTS while preserving existing text behavior and credential security.

## Compatibility guarantees
- Existing TASK-017 encrypted provider settings/data must migrate without credential loss or plaintext re-entry.
- Existing text generation APIs and Proposal/Script/Scene Plan behavior remain backward-compatible.
- AES-GCM, AAD binding, owner isolation, optimistic revision handling and fail-closed secret behavior must not be weakened.

## Deployment catalog
- Provider capabilities, internal model IDs and internal voice IDs are deployment-controlled.
- Creator input must never introduce arbitrary provider base URLs, external model IDs or external voice IDs.
- Invalid/duplicate capability, model or voice definitions fail startup; no silent fallback.
- Public metadata may expose only safe internal IDs and display metadata required for selection.

## Owner settings
- One encrypted owner/provider credential may serve multiple capabilities only when the upstream credential legitimately spans them.
- Owner settings may persist capability/model/voice enablement and selection using internal IDs only.
- Rotate/preserve/delete semantics for secrets remain compatible with TASK-017.
- Stored API keys, ciphertext, base URLs and external provider identities must never be returned to the client.

## Runtime resolution
The owner runtime must resolve provider-neutral ports by owner + capability + internal selection:
- text -> `TextGenerator`
- image -> `ImageGenerator`
- TTS -> `SpeechSynthesizer`

Resolution must fail closed when:
- owner/provider setting does not exist;
- credential is unavailable/decryption fails;
- capability/model/voice is disabled or not in the deployment catalog;
- requested selection belongs to another owner or invalid provider binding.

No cross-owner credential or configuration reuse is permitted.

## Safe option APIs
Capability-aware settings/options responses may expose:
- internal provider/model/voice IDs;
- display names;
- safe capability metadata;
- locale/language/style metadata where needed for TTS selection.

They must never expose:
- API keys or secret material;
- ciphertext/nonces/AAD internals;
- provider base URLs;
- external model/voice IDs;
- adapter-specific configuration not required by the creator UI.

## Migration and data integrity
- Schema changes require an explicit forward migration from existing TASK-017 data.
- Existing text configuration must remain usable after migration.
- Migration must be deterministic, idempotent under normal migration tooling and covered against real PostgreSQL.
- No destructive rewrite of encrypted credentials is allowed merely to add image/TTS capability metadata.

## HTTP / UI behavior
- Existing text provider settings endpoints remain backward-compatible or evolve additively.
- Image/TTS option/settings endpoints are owner-scoped and return provider-neutral safe metadata.
- UI must support capability/model/voice enablement without persisting secret material client-side.
- Error responses must not leak credentials, ciphertext, base URLs or external identities.

## Out of scope
- durable generation jobs;
- Media Asset ingestion/persistence/binding;
- per-scene image/TTS orchestration;
- TTS chunking/stitching/timing;
- captions/music/editor/render/publish;
- video-provider activation.

## Required tests / TDD gate
Behavior changes follow RED -> GREEN -> REFACTOR where applicable. Acceptance requires evidence for:
1. migration/backward compatibility from TASK-017 text settings;
2. encryption, owner isolation and fail-closed regressions;
3. deployment catalog validation and duplicate-ID rejection;
4. image/TTS model and voice selection validation;
5. disabled selections fail before adapter use;
6. correct owner-specific secret reaches runtime adapter only at request time;
7. safe option/settings payloads contain no secret/external-ID sentinel;
8. existing text generation E2E/regressions remain green;
9. settings UI secret-preserve/rotate/delete and capability-selection flows;
10. race/full API/frontend verification and fresh PR CI.

## Acceptance gate
TASK-028 may be marked READY only when this contract is present on `develop` and TASK-017, TASK-024, TASK-025, TASK-026 and TASK-027 prerequisites are complete. Implementation must use a feature branch and PR into `develop`.