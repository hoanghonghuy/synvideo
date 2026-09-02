# SynVideo Task Board

Current milestone: `F1 — CREATIVE WORKFLOW`

PM plans ahead. AI Developers may start only tasks explicitly marked `READY`; review fixes stay on the original branch/PR. Parallel work follows `docs/engineering/PARALLEL_WORK_PROTOCOL.md` and normally uses at most 3 implementation worktrees.

| ID | Task | Status | Spec / Notes |
|---|---|---|---|
| TASK-001 | Technical foundation and runnable project skeleton | DONE | Accepted PR #3. |
| TASK-002 | Project domain and persistence foundation | DONE | Accepted PR #8. |
| TASK-003 | Creative Brief backend and persistence | DONE | Accepted PR #9. |
| TASK-004 | Creative Brief frontend workspace | DONE | Accepted PR #10. |
| TASK-005 | AI provider capability and text-generation contracts | DONE | Accepted PR #11. |
| TASK-006 | AI Proposal domain, persistence and approval API | DONE | Accepted PR #17. |
| TASK-007 | AI Proposal generation engine | DONE | Accepted PR #16. |
| TASK-008 | AI Proposal frontend workspace | DONE | Accepted PR #20. |
| TASK-009 | AI Proposal generation job integration | DONE | Accepted PR #28. |
| TASK-010 | Durable job execution foundation | DONE | Accepted PR #21. |
| TASK-011 | Script domain, persistence and approval API | DONE | Accepted PR #22. |
| TASK-012 | Script generation engine | DONE | Accepted PR #24. |
| TASK-013 | Live OpenAI-compatible text provider adapter foundation | DONE | Accepted PR #29. |
| TASK-014 | Scene Plan generation engine | DONE | Accepted PR #27. |
| TASK-015 | Scene Plan domain and persistence foundation | DONE | Accepted PR #32. |
| TASK-016 | Media Asset + S3-compatible storage foundation | DONE | Accepted PR #33. |
| TASK-017 | Secure BYOK text provider settings and owner-scoped runtime | DONE | Accepted PR #35. |
| TASK-018 | Script durable generation integration | DONE | PR #48 accepted; squash `6bc3c86b...`. |
| TASK-019 | Script creator workspace | DONE | PR #47 accepted; squash `da01e58c...`. |
| TASK-020 | Scene media binding foundation | DONE | PR #46 accepted; squash `b80b8e7b...`. |
| TASK-021 | Scene Plan durable generation + API integration | DONE | Issue #39 completed; PR #50 squash `9d2b5306...`. |
| TASK-022 | Scene Plan creator workspace | CHANGES_REQUESTED | Issue #40 / PR #51. Head `ee33676...`; all behavior accepted. Only latest-`develop` sync + fresh CI remain. TL re-review `5089547573`. |
| TASK-023 | Media Library + Scene Binding API integration | DONE | Issue #41 completed; PR #53 accepted head `f997905...`, CI #262, TL review `5089424360`, squash `7e3df69...`. |
| TASK-024 | Media Library + scene assignment workspace | BACKLOG | Issue #42. TASK-023 dependency is satisfied; activate immediately after TASK-022 releases frontend router/locale workspace surfaces. |
| TASK-025 | Provider-neutral visual generation foundation | DONE | Issue #43 completed; PR #49 squash `1c550f316...`. |
| TASK-026 | Live OpenAI image generation adapter | DONE | Issue #44 completed; PR #52 accepted head `c396337...`, CI #260, TL review `5089423041`, squash `cf4317ad...`. |
| TASK-027 | Provider-neutral TTS + OpenAI speech adapter foundation | READY | Issue #45. Slot released; official speech API/model revalidated 2026-09-02. Claim only from latest `origin/develop`. |
| TASK-028 | Secure multi-capability provider runtime and settings | BACKLOG | Issue #54 / `docs/tasks/TASK-028.md`. Depends on accepted TASK-027; dedicated runtime contract must be frozen before READY and shared frontend settings hotspot must be free. |

## Current implementation / review slots

- **Dev A — TASK-022 `CHANGES_REQUESTED`**: continue only PR #51. Functional review is complete; rebase/sync to latest `origin/develop`, rerun frontend verify, push and wait for fresh CI. Do not add behavior.
- **Dev B — TASK-027 `READY`**: may claim `feature/TASK-027-tts-provider-foundation` from latest `origin/develop`. Owns minimum provider-core TTS extension + isolated OpenAI speech adapter/tests only.
- **Dev C — intentionally unallocated**: TASK-024 becomes READY immediately after TASK-022 merges. Do not manufacture a conflicting task merely to fill this slot.

## Current review gate — TASK-022 / PR #51

All functional blockers are accepted on head `ee33676...`, including the final in-flight edit race regression. The only remaining gate is integration freshness:
1. sync/rebase onto the latest `origin/develop` after TASK-023/TASK-026 and PM control-plane commits;
2. preserve the accepted Scene Plan behavior without scope expansion;
3. run `npm --prefix apps/web run verify`;
4. fresh PR CI on the post-sync head must be green;
5. if no new behavioral diff appears, next TL pass is merge-only verification.

Issue #40 is authoritative.

## Parallel safety

- TASK-022 and TASK-024 both use frontend router/locale/project-workspace integration; do not overlap them.
- TASK-027 owns a minimum core provider TTS extension plus `providers/openaitts/**`; do not activate TASK-028 provider-runtime evolution until TASK-027 is accepted.
- TASK-028 will reuse TASK-017 encrypted credentials/runtime and must preserve existing text behavior; it also touches the AI Provider settings UI, so schedule it after TASK-024 releases shared frontend surfaces.
- Do not create micro-tasks merely to fill an implementation slot.

## Next activation path

1. TASK-027 may start now from current `develop`.
2. Rebase/verify/merge TASK-022.
3. Immediately after TASK-022 merges, promote TASK-024 to READY because TASK-023 is already DONE.
4. Accept TASK-027, then freeze `MULTI_CAPABILITY_PROVIDER_RUNTIME_V1` and promote TASK-028 when the settings/frontend hotspot is free.
5. After TASK-028, plan durable per-scene generated-image acquisition/MediaAsset ingestion and durable narration/TTS orchestration as separate tasks; do not jump directly to render/publish.
6. Follow with stock/captions/music, Scene Editor/composition snapshot, render/export, publishing/channel management and production hardening/E2E.

## Product checkpoint

Creative Brief, Proposal and Script are creator-usable end to end. Stage 7 Scene Plan backend is complete and its creator workspace has passed functional review pending only current-base integration verification. Media Library/Scene Binding APIs and the first live image adapter are merged. The next live capability lane is TTS, while the Media workspace will activate as soon as the Scene Plan frontend lane is released.

## Architecture gates

- Core provider capabilities remain provider-neutral; vendor SDK/schema types stay at adapters.
- Image generation may be synchronous/streaming; video generation retains async Start/Poll/OpenResult with opaque external operation identity.
- Paid video orchestration must persist external operation identity before poll/resume to avoid duplicate submissions/cost after worker crash.
- Provider output URLs are not durable SynVideo assets; generated bytes must be ingested into MediaAsset storage with provenance.
- Credentials/ciphertext/base URLs/raw upstream responses never enter durable jobs or public resources.
- TTS input limits must return explicit errors; later orchestration owns deterministic chunk/stitch/timing.
- Multi-capability runtime must reuse TASK-017's encrypted owner credential path rather than create image/TTS secret silos.

## Isolation / merge rules

- Each implementation task uses a dedicated worktree; shared/control checkout remains on `develop`.
- Maximum concurrent implementation worktrees normally equals 3.
- Review fixes stay on the original branch/PR.
- READY tasks are claimed by atomically creating the absent remote ref from latest `origin/develop`.
- Truthful RED → GREEN → REFACTOR is mandatory.
- Team Lead review + current-base green CI is the merge gate.
- Merged worktrees are cleaned before another claim.
- Do not self-merge or self-mark DONE.

Allowed statuses: `BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.