# TASK-008 — AI Proposal frontend workspace

Status: CHANGES_REQUESTED
Milestone: F1 Creative Workflow
Depends on: TASK-004 accepted
Wave: WAVE-F1-B / continuing in WAVE-F1-C
Branch: `feature/TASK-008-ai-proposal-web`
Base: `develop`
Active PR: #20

## Current review gate
Team Lead review on PR #20 found two recoverable-load state blockers:
- `loadVersion()` clears dirty/stale/saved/pending state and changes selected version before GET succeeds; a failed explicit reload after stale conflict can leave edited form values visible while parent state incorrectly says clean, allowing a later version switch to discard them without warning;
- when Project + Proposal list load but the selected/newest Proposal GET fails, proposal-level error UI is not rendered because it sits inside the `selectedProposal` branch; failed user-initiated version switching can also make the highlighted selected version diverge from the Proposal content actually displayed.

Required TDD fixes on the same PR/branch:
1. stale/dirty local edits -> explicit reload -> GET failure must preserve local values and dirty/recoverable state;
2. list has Proposal -> version GET fails must show a localized visible error/retry state;
3. failed version switch must keep selection/content coherent and must not claim a version was loaded when GET failed;
4. sync latest `develop`, continue only in the dedicated TASK-008 worktree, rerun targeted frontend tests and full CI.

## Goal
Build the creator-facing AI Proposal workspace for viewing Proposal history, editing a draft, resolving stale edits and explicitly approving a version before script generation.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/contracts/AI_PROPOSAL_V1.md`
- `docs/product/CREATIVE_WORKFLOW.md`
- accepted Project/Creative Brief frontend conventions

## Contract
`docs/contracts/AI_PROPOSAL_V1.md` is frozen. Develop against deterministic transport mocks first.

TASK-006 provides the real persistence/API and is accepted on `develop`; final backend smoke must use that implementation.

Generation CTA is intentionally outside this task and is added by TASK-009. Do not fake successful AI generation in the UI.

## Primary write paths
- `apps/web/src/features/creative-proposal/**`
- Proposal-specific views/tests under accepted frontend conventions.

## Allowed shared hotspots
- minimal route registration;
- minimal Project/Creative Brief navigation link into Proposal;
- i18n resource additions.

## Reserved / do not touch
- `apps/api/**` — accepted Proposal backend/generation plus TASK-010/TASK-011 work.
- unrelated global visual redesign/shared architecture refactors.
- Creative Brief behavior except minimal navigation entry.

## Scope
- Route/workspace for a Project's Proposal history/current version.
- Empty/no-proposal state without pretending generation already exists.
- Version list/switching with clear `draft|approved|superseded` state.
- Full frozen V1 content rendering.
- Draft editing and PUT save with current revision.
- Dirty/saving/saved/error/stale state handling.
- Approved/superseded versions read-only.
- Explicit approval action with confirmation and current revision.
- Stale edit/approval handling without automatic overwrite/retry.
- Prevent or explicitly confirm destructive version switching while dirty.
- i18n for all visible strings; responsive/keyboard/accessibility basics.

## Out of scope
- Generate/Regenerate AI action.
- provider/model selection or credentials.
- targeted AI revise.
- script/scenes/media/editor.

## TDD plan
Start RED for at least:
1. empty state when Proposal list is empty;
2. version history renders newest/current status accurately;
3. draft fields are editable and PUT sends current revision;
4. successful save stores returned revision and clears dirty state;
5. validation/network failure preserves values + dirty state;
6. stale save shows conflict without auto-overwrite;
7. approved/superseded versions are read-only;
8. explicit approval sends current revision and renders approved state;
9. dirty version switch cannot silently discard edits;
10. real Project/Creative Brief navigation reaches the production Proposal view;
11. failed reload/version GET preserves dirty state and coherent selection while surfacing a visible error.

## Acceptance criteria
- [ ] Frontend matches `AI_PROPOSAL_V1.md` field/status semantics.
- [ ] Version history/read-only states are clear.
- [ ] Draft save/dirty/stale behavior is regression-tested.
- [ ] Recoverable version/reload failures do not silently clear dirty state or leave selection/content inconsistent.
- [ ] Proposal-level GET failures surface a localized visible error/retry state.
- [ ] Approval is explicit and durable-looking only after server success.
- [ ] No fake generation success or vendor/provider UI is introduced.
- [ ] Visible copy goes through i18n.
- [ ] Frontend lint/typecheck/test/build green.
- [ ] Real backend smoke covers list/get/edit/stale/approve.
- [ ] TDD evidence is truthful.

## Verification
At minimum:
- targeted feature/client/component tests;
- `npm run lint:web`, `npm run typecheck:web`, `npm run test:web`, `npm run build:web`;
- final smoke against merged TASK-006 backend;
- full repository verification and `git diff --check`;
- PR CI on latest `develop` after the review-fix push.

## Merge order
Independent from TASK-010 and TASK-011 by primary write surface. Final acceptance uses merged TASK-006 backend. TASK-009 remains blocked until this task is accepted.

Do not self-merge or self-mark DONE.
