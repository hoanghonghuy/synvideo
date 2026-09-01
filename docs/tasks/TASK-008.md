# TASK-008 — AI Proposal frontend workspace

Status: DONE
Milestone: F1 Creative Workflow
Depends on: TASK-004 accepted
Wave: WAVE-F1-B / completed in WAVE-F1-D transition
Branch: `feature/TASK-008-ai-proposal-web`
Base: `develop`
Accepted PR: #20
Accepted head: `fe7c368`
Squash merge: `36418b8eaccf9c65bb133dac2f3f28c9f3a40da0`

## Acceptance record
Team Lead accepted the final implementation after multiple TDD review rounds:
- Proposal history/current-version workspace matches frozen `AI_PROPOSAL_V1` semantics;
- draft edit/save uses current optimistic revision and stores the returned revision;
- approved/superseded versions are read-only;
- approval is explicit and only looks durable after server success;
- dirty version switching cannot silently discard edits;
- stale/reload GET failures preserve local form values, dirty state and coherent selected-version state;
- mutation failures are isolated from version-load retry state;
- Project-success + Proposal-list failure renders localized recoverable error/retry instead of false empty history;
- all visible copy is i18n-backed;
- real backend smoke covers list/get/edit/stale/approve;
- final CI #122 is green.

## Goal
Build the creator-facing AI Proposal workspace for viewing Proposal history, editing a draft, resolving stale edits and explicitly approving a version before script generation.

## Contract
`docs/contracts/AI_PROPOSAL_V1.md` is frozen and authoritative.

TASK-006 provides the accepted persistence/API. Generation/Regenerate remains outside this task and belongs to TASK-009.

## Delivered scope
- Route/workspace for a Project's Proposal history/current version.
- Empty/no-proposal state without pretending generation already exists.
- Version list/switching with clear `draft|approved|superseded` state.
- Full frozen V1 content rendering.
- Draft editing and PUT save with current revision.
- Dirty/saving/saved/error/stale state handling.
- Approved/superseded versions read-only.
- Explicit approval action with confirmation and current revision.
- Stale edit/approval handling without automatic overwrite/retry.
- Destructive version switching protection while dirty.
- Localized recoverable list/version error states.
- i18n for visible strings and accepted frontend accessibility/responsive conventions.

## Out of scope
- Generate/Regenerate AI action.
- provider/model selection or credentials.
- targeted AI revise.
- script/scenes/media/editor.

## TDD coverage
1. empty state when Proposal list is empty;
2. version history renders newest/current status accurately;
3. draft fields are editable and PUT sends current revision;
4. successful save stores returned revision and clears dirty state;
5. validation/network failure preserves values + dirty state;
6. stale save shows conflict without auto-overwrite;
7. approved/superseded versions are read-only;
8. explicit approval sends current revision and renders approved state;
9. dirty version switch cannot silently discard edits;
10. Project navigation reaches the production Proposal view;
11. failed reload/version GET preserves dirty state and coherent selection while surfacing a visible error;
12. failed mutation never exposes version-load retry or stale failed-version routing;
13. Proposal list load failure surfaces error/retry instead of false empty state.

## Acceptance criteria
- [x] Frontend matches `AI_PROPOSAL_V1.md` field/status semantics.
- [x] Version history/read-only states are clear.
- [x] Draft save/dirty/stale behavior is regression-tested.
- [x] Recoverable version/reload failures do not silently clear dirty state or leave selection/content inconsistent.
- [x] Proposal version/list load failures surface localized visible error/retry states and are not rendered as authoritative empty history.
- [x] Mutation failures preserve edits and never expose destructive load retry by mistake.
- [x] Approval is explicit and durable-looking only after server success.
- [x] No fake generation success or vendor/provider UI is introduced.
- [x] Visible copy goes through i18n.
- [x] Frontend lint/typecheck/test/build green.
- [x] Real backend smoke covers list/get/edit/stale/approve.
- [x] TDD evidence is truthful.

## Next dependency
TASK-009 may now consume this accepted frontend surface when PM freezes the Proposal generation-job integration contract against accepted TASK-010.
