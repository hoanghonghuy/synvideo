# TASK-008 — AI Proposal frontend workspace

Status: CHANGES_REQUESTED
Milestone: F1 Creative Workflow
Depends on: TASK-004 accepted
Wave: WAVE-F1-B / continuing in WAVE-F1-C
Branch: `feature/TASK-008-ai-proposal-web`
Base: `develop`
Active PR: #20
Current reviewed head: `41a6b7f`

## Current review gate
The earlier dirty/version-load blockers and mutation-vs-load retry regression are fixed correctly; CI #112 is green.

One recoverable workspace-load blocker remains:
- `loadWorkspace()` assigns `project` before calling `listCreativeProposals()`;
- if Project GET succeeds but Proposal list GET fails, `project` remains non-null while `summaries` remains empty;
- the template only renders the top-level load error when `loadErrorCode && !project`, so this failure is falsely rendered as the authoritative empty Proposal state with no retry.

Required TDD fix on the same PR/worktree:
1. Project GET succeeds -> Proposal list GET fails;
2. render localized visible error + retry;
3. do not render the empty Proposal state for that failure;
4. retry remains failure-safe and does not regress dirty/version handling;
5. rerun targeted frontend tests and full CI.

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
- `apps/api/**` — accepted Proposal backend/generation plus downstream backend work.
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
11. failed reload/version GET preserves dirty state and coherent selection while surfacing a visible error;
12. failed mutation never exposes version-load retry or stale failed-version routing;
13. Proposal list load failure surfaces error/retry instead of false empty state.

## Acceptance criteria
- [ ] Frontend matches `AI_PROPOSAL_V1.md` field/status semantics.
- [ ] Version history/read-only states are clear.
- [ ] Draft save/dirty/stale behavior is regression-tested.
- [ ] Recoverable version/reload failures do not silently clear dirty state or leave selection/content inconsistent.
- [ ] Proposal version/list load failures surface localized visible error/retry states and are not rendered as authoritative empty history.
- [ ] Mutation failures preserve edits and never expose destructive load retry by mistake.
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
- PR CI after the review-fix push.

## Merge order
Independent by primary write surface. TASK-009 remains blocked until this task is accepted.

Do not self-merge or self-mark DONE.
