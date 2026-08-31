# TASK-004 — Creative Brief frontend workspace

Status: BLOCKED
Milestone: F1 Creative Workflow
Depends on: TASK-002 accepted
Integration gate: TASK-003 backend merged before final acceptance
Branch: `feature/TASK-004-creative-brief-web`
Base: `develop`
Wave: WAVE-F1-A

## Goal
Build the creator-facing Creative Brief V1 workspace in Vue so a user can enter, edit, save, reopen and resolve stale-edit conflicts for creator intent before any AI generation begins.

## Why
Frontend can be implemented in parallel with backend because PM has frozen the API/resource contract. This reduces delivery time without letting either developer invent the shared contract.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/contracts/CREATIVE_BRIEF_V1.md`
- `docs/product/CREATIVE_WORKFLOW.md`
- accepted TASK-002 project routes/client/i18n conventions after it merges

Do not recursively load unrelated docs.

## Integration contract
Consumes frozen `docs/contracts/CREATIVE_BRIEF_V1.md`.

Implement client/feature tests against this contract using deterministic transport mocks while TASK-003 is in parallel. Do not change the contract from the frontend branch.

## Parallel safety
### Primary write paths
- `apps/web/src/features/creative-brief/**`
- A Creative-Brief-specific view under the accepted frontend view/feature convention.
- Creative-Brief-specific test files.

### Allowed shared integration files
- Minimal route registration needed to navigate from an accepted Project detail/workspace route to Creative Brief.
- Minimal i18n resource registration/additions using the accepted locale structure.
- Existing API client composition only where necessary to register the feature client.

### Reserved / do not touch
- `apps/api/**` — owned by TASK-003/TASK-005 in this wave.
- Project persistence/domain behavior from TASK-002.
- Global visual redesign, unrelated dashboard routes or shared architecture refactors.

TASK-004 may reach review in parallel with TASK-003, but final acceptance requires rebase on the merged backend task and a real integration/smoke pass against the actual API.

## Scope
- Creative Brief form/workspace for all frozen V1 fields.
- Load existing brief or initialize a new draft when none exists.
- Explicit save behavior with dirty/saving/saved/error states.
- Keep user input after recoverable validation/network errors.
- Track current server `revision` after save.
- Handle `STALE_REVISION` as a localized conflict state; do not auto-overwrite. V1 provides explicit reload/latest-data action.
- Navigate from the project workspace/detail into Creative Brief without turning this task into the final editor/dashboard.
- All user-visible copy through i18n; Vietnamese active locale.
- Responsive, keyboard-usable, semantically accessible form basics.

## Out of scope
- Backend/migration changes.
- AI Proposal generation CTA behavior beyond a disabled/not-yet-available boundary if the existing UX needs one.
- Autosave/background merge conflict resolution.
- File/image/video upload.
- Script/scenes/editor/media.
- Final brand/design-system overhaul.

## Required behavior
- Fields and validation messages map to the frozen contract.
- New brief save uses create semantics; subsequent save includes latest revision.
- Successful save replaces local dirty state and stores returned revision.
- Validation/network errors preserve the user's unsaved input.
- Stale revision never silently retries with overwrite semantics.
- Reload action after stale conflict fetches latest persisted brief and clearly replaces the local stale version only after explicit user action.
- Loading, empty/new, saving, success, validation, network error and stale-conflict states are distinguishable.
- No hard-coded visible Vietnamese text in components.

## TDD plan
Start with RED feature/component/client tests, including at minimum:
1. new-draft rendering from GET 404/no brief semantics;
2. field editing and PUT create payload;
3. existing brief update sends current revision and stores incremented revision;
4. validation/network failure preserves entered values;
5. stale revision shows conflict state and does not retry automatically;
6. reload-latest flow after stale conflict;
7. route/open flow from an existing project.

Use contract-shaped transport mocks; snapshots alone are insufficient.

## Acceptance criteria
- [ ] Creator can open a project and reach the Creative Brief workspace.
- [ ] All V1 fields can be edited with clear validation/help where needed.
- [ ] Save/reopen behavior follows the frozen API contract.
- [ ] Dirty/saving/saved/error/stale states are implemented and tested.
- [ ] Stale edits cannot silently overwrite newer server state.
- [ ] Visible copy is localized through i18n.
- [ ] Frontend lint/typecheck/test/build remain green.
- [ ] After TASK-003 merges, branch is rebased as needed and real backend integration smoke passes before acceptance.
- [ ] PR contains TDD evidence.

## Open-source research
None required; this is product UI/application behavior rather than an editor/media subsystem.

## Verification
At minimum:
- targeted Creative Brief client/feature/component tests;
- `npm run lint:web`;
- `npm run typecheck:web`;
- `npm run test:web`;
- `npm run build:web`;
- final smoke against merged TASK-003 backend;
- `git diff --check` or equivalent.

## Delivery
PR to `develop` from `feature/TASK-004-creative-brief-web` must include:
- UX/state summary;
- TDD evidence: RED/GREEN/REFACTOR;
- contract-client behavior tested;
- frontend verification;
- final backend integration result after TASK-003 merges;
- any contract issue surfaced without silently changing the frontend expectation.

Do not self-merge or mark DONE.
