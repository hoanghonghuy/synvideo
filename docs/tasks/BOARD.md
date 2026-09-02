# SynVideo Task Board

Current milestone: `F1 — CREATIVE WORKFLOW`

PM plans ahead. AI Developers may start only PM-authorized executable work. Live authorization/lifecycle is on the authoritative GitHub task issue; this board is the queue/order/status mirror. Remote claim ownership is determined separately by canonical task branch / active PR.

Follow `docs/engineering/CONTROL_PLANE_PROTOCOL.md` before making workflow decisions. A stale local checkout or stale board row is never proof that fresher remote work does not exist.

## Control-plane invariant

- `BOARD.md` mirrors queue/order/status; `TASK-XXX.md` owns task scope/acceptance/branch contract; the GitHub task issue owns live execution authorization/lifecycle; canonical remote branch/PR owns claim state.
- Metadata drift must be reconciled, but consumers use the authority hierarchy rather than treating all mirrors as equal vetoes.
- Material scope/branch/acceptance/dependency/frozen-contract conflicts are contract drift and block implementation until PM/Team Lead resolves them.
- Activate `READY` by freezing versioned task/contracts/order on remote `develop` first, then changing the GitHub issue to `READY` last.
- After merge, update/close the issue and reconcile board/task mirrors; stale actionable review instructions must not remain.

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
| TASK-018 | Script durable generation integration | DONE | PR #48 squash `6bc3c86b...`. |
| TASK-019 | Script creator workspace | DONE | PR #47 squash `da01e58c...`. |
| TASK-020 | Scene media binding foundation | DONE | PR #46 squash `b80b8e7b...`. |
| TASK-021 | Scene Plan durable generation + API integration | DONE | PR #50 squash `9d2b5306...`. |
| TASK-022 | Scene Plan creator workspace | DONE | Issue #40 completed; PR #51 accepted head `e0cb568...`, CI #269, TL review `5089868599`, squash `c8d8618...`. |
| TASK-023 | Media Library + Scene Binding API integration | DONE | Issue #41 completed; PR #53 squash `7e3df69...`. |
| TASK-024 | Media Library + scene assignment workspace | DONE | Issue #42 completed; PR #56 accepted head `25db940...`, TL re-review `5090827272`, exact-head CI #279, squash `f0aa549...`. |
| TASK-025 | Provider-neutral visual generation foundation | DONE | Issue #43 completed; PR #49 squash `1c550f316...`. |
| TASK-026 | Live OpenAI image generation adapter | DONE | Issue #44 completed; PR #52 squash `cf4317ad...`. |
| TASK-027 | Provider-neutral TTS + OpenAI speech adapter foundation | DONE | Issue #45 completed; PR #55 accepted head `e00413b...`, exact-head CI #281, squash `a98928f...`. |
| TASK-028 | Secure multi-capability provider runtime and settings | IN_PROGRESS | Issue #54; canonical branch `feature/TASK-028-multicap-provider-runtime` already exists and is the live claim. No PR yet at reconciliation time. |

## Current implementation / review slots

- **TASK-028 — claimed / IN_PROGRESS**: continue only the existing canonical branch/worktree. Do not create a replacement claim or duplicate task.
- Additional implementation slots remain intentionally unallocated until PM identifies independent non-duplicate work with frozen contracts/write surfaces.

## Parallel safety

- TASK-028 owns the provider-runtime/settings evolution declared in `docs/tasks/TASK-028.md`.
- Do not create micro-tasks merely to fill an implementation slot.
- Before planning new work, search open/recent issues, TASK specs, active/recent PRs and canonical branches for duplicate/overlapping product outcomes.

## Next planning path

1. Complete/review TASK-028 through its existing canonical branch/PR workflow.
2. Then plan durable per-scene generated-image acquisition/MediaAsset ingestion and durable narration/TTS orchestration as separate non-overlapping tasks.
3. Follow with stock/captions/music, Scene Editor/composition snapshot, render/export, publishing/channel management and production hardening/E2E.

## Architecture gates

- Core provider capabilities remain provider-neutral; vendor SDK/schema types stay in adapters.
- Image generation may be synchronous/streaming; video generation retains async Start/Poll/OpenResult with opaque external operation identity.
- Paid video orchestration must persist external operation identity before poll/resume to avoid duplicate submissions/cost after worker crash.
- Provider output URLs are not durable SynVideo assets; generated bytes must be ingested into MediaAsset storage with provenance.
- Credentials/ciphertext/base URLs/raw upstream responses never enter durable jobs or public resources.
- TTS input limits return explicit errors; later orchestration owns deterministic chunk/stitch/timing.
- Multi-capability runtime must reuse TASK-017's encrypted owner credential path rather than create image/TTS secret silos.

## Isolation / merge rules

- Each implementation task uses a dedicated worktree; shared/control checkout remains on `develop`.
- Maximum concurrent implementation worktrees normally equals 3.
- Review fixes stay on the original branch/PR.
- New authorized tasks are claimed by atomically creating the absent canonical remote ref from current `origin/develop`.
- Existing canonical branch/PR means claimed even if an issue/mirror still says `READY`.
- Truthful RED → GREEN → REFACTOR is mandatory.
- Team Lead exact-head review + required current-head/current-base green CI is the merge gate.
- Do not self-merge or self-mark DONE.

Allowed statuses: `BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.
