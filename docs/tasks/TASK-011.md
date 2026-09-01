# TASK-011 — Script domain, persistence and approval API

Status: DONE
Milestone: F1 Creative Workflow
Depends on: TASK-006 accepted
Wave: WAVE-F1-C / completed in WAVE-F1-D transition
Branch: `feature/TASK-011-script-persistence`
Base: `develop`
Accepted PR: #22
Accepted head: `8652f951`
Squash merge: `87c9849e4b0a8ec913ff5df93e2ea26a519f108e`

## Acceptance record
Team Lead accepted the synced final implementation after TDD review:
- Script creation requires an owner-visible approved Proposal source;
- Project-scoped Script versions are monotonic with at most one active draft;
- newer drafts supersede only prior active unapproved drafts;
- approved/superseded Script history is immutable;
- optimistic revision update/approval returns stable stale/immutable conflicts;
- Project content locale is snapshotted into Script;
- owner isolation/non-disclosure and concurrent CreateDraft behavior are proven against real PostgreSQL;
- heading/body/notes limits use rune/character semantics with multibyte boundary regressions;
- migration `0005_create_scripts.sql` works through the current migration runner;
- latest-develop sync preserves accepted TASK-010 durable jobs behavior;
- final CI #123 is green;
- no AI generation/provider/Scene Plan/frontend scope leakage.

## Goal
Provide the durable Script resource for Creative Workflow Stage 5–6: versioned editable drafts sourced from approved Proposal versions, explicit immutable approval, PostgreSQL persistence and owner-isolated read/edit/approve APIs.

## Contract
`docs/contracts/SCRIPT_V1.md` is frozen and authoritative.

## Delivered scope
- Script domain types/validation matching `SCRIPT_V1`.
- Internal `CreateDraft` requiring an approved owner-visible Proposal version.
- `source_proposal_version` and Project `content_locale` snapshot persistence.
- Project-scoped monotonic Script versions.
- At most one active Script draft with atomic prior-draft supersede.
- Draft optimistic `revision` update.
- `draft|approved|superseded` statuses.
- Atomic approval with expected revision and durable `approved_at`.
- Newest-first version list and full version get.
- Owner isolation/non-disclosure through the Project principal boundary.
- HTTP GET list/version, PUT draft replacement and POST approve.
- Real PostgreSQL integration/concurrency coverage.

## Important invariants
- Script cannot be created from draft/superseded/non-visible Proposal.
- Approved Script content cannot be mutated.
- New draft creation never rewrites approved Script history.
- Concurrent draft creation cannot allocate duplicate versions or leave multiple active drafts.
- stale edit/approval cannot silently win.
- long-form Script sections are supported.
- text length validation uses character/rune semantics, not UTF-8 byte length.

## TDD coverage
1. approved Proposal -> first Script draft v1/revision1/draft;
2. non-approved Proposal -> rejected without Script creation;
3. second draft -> monotonic version + prior active draft superseded;
4. approved Script remains immutable and preserved;
5. current revision update increments exactly once;
6. stale update/approval -> conflict;
7. concurrent CreateDraft -> unique versions + one active draft;
8. owner isolation;
9. newest-first list;
10. frozen section/length/cardinality validation;
11. Unicode text boundaries for heading/body/notes.

## Acceptance criteria
- [x] Migration `0005_create_scripts.sql` works through current runner on accepted `develop`.
- [x] Script source must be an approved owner-visible Proposal.
- [x] Version/draft/approval invariants match `SCRIPT_V1`.
- [x] Owner isolation and concurrency are proven against real PostgreSQL.
- [x] Unicode/multibyte text respects frozen character limits.
- [x] Stable HTTP conflicts include `STALE_REVISION` and `SCRIPT_IMMUTABLE`.
- [x] No AI generation/provider call/Scene Plan/frontend implementation is introduced.
- [x] Final post-sync CI is green.
- [x] TDD evidence is truthful.

## Next dependencies
TASK-012 Script generation engine builds independently against the frozen Script contracts. Later Script generation integration will persist validated candidates through this task's internal `CreateDraft`; Scene Plan work starts only from an approved Script version.
