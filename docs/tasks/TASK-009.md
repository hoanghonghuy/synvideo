# TASK-009 — AI Proposal generation integration

Status: BLOCKED
Milestone: F1 Creative Workflow
Depends on: TASK-006, TASK-007 and TASK-008 accepted
Wave: post-WAVE-F1-B integration
Branch: `feature/TASK-009-ai-proposal-integration`
Base: `develop`

## Goal
Connect the accepted Proposal persistence/API, provider-neutral generation engine and Proposal frontend into the complete Stage 3–4 flow: generate/regenerate a Proposal from the current Creative Brief, persist a new draft version, review/edit it and explicitly approve it.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/contracts/AI_PROPOSAL_V1.md`
- `docs/contracts/AI_PROPOSAL_GENERATION_V1.md`
- accepted TASK-006/007/008 implementations

## Scope
- Owner-scoped generation application service loads Project + current Creative Brief.
- Select provider/model through accepted provider registry/capability boundary.
- Call accepted Proposal generation engine.
- Persist candidate through accepted Proposal `CreateDraft` operation.
- Expose `POST /api/v1/projects/{project_id}/creative-proposals/generate` (request contains stable `provider_id` and `model_id`; no credentials).
- Return created Proposal draft (`201`) and stable mapped generation errors.
- Frontend Generate/Regenerate action with provider/model ids supplied by the current runtime/configured capability surface available at implementation time; if product-visible selection is not yet available, do not invent credentials UI—use an explicitly documented configured/default capability path or escalate PM decision.
- Regenerate creates a new draft version and does not mutate approved history.
- End-to-end flow: Creative Brief -> generate -> edit/save -> regenerate/version history -> approve.
- Full error/loading/retry/stale behavior and real local integration smoke.

## TDD plan
Start RED around integration seams:
1. generation loads the current owner-scoped Creative Brief revision;
2. valid engine candidate persists as next draft version;
3. regenerate never mutates an approved version;
4. no Creative Brief -> stable user-facing error;
5. provider unavailable/failure/invalid output does not create a Proposal row;
6. concurrent generation cannot allocate duplicate Project versions;
7. frontend Generate success opens the returned draft/version;
8. frontend failure preserves current reviewed Proposal state;
9. complete local smoke through approval.

## Acceptance criteria
- [ ] Stage 3–4 is usable end-to-end in local/dev configuration without fake successful UI state.
- [ ] Generation failure is transactional: no partial Proposal version.
- [ ] Approved Proposal history is preserved.
- [ ] No API keys/secrets are sent in generation request payloads.
- [ ] Provider-specific SDK/schema remains outside domain/frontend Proposal semantics.
- [ ] TDD evidence, full CI and real integration smoke are green.

## Blocked reason
Requires the three independently developed WAVE-F1-B outputs to be accepted first. Do not claim until PM moves this task to READY.

Do not self-merge or self-mark DONE.
