# TASK-011 — Script domain, persistence and approval API

Status: CHANGES_REQUESTED
Milestone: F1 Creative Workflow
Depends on: TASK-006 accepted
Wave: WAVE-F1-C
Branch: `feature/TASK-011-script-persistence`
Base: `develop`
Active PR: #22
Current reviewed head: `6fb2910`

## Current review gate
The Script implementation itself is accepted on reviewed head `6fb2910`:
- Unicode text limits now use rune/character semantics with multibyte boundary regressions;
- approved Proposal source enforcement, version allocation, one-active-draft invariant, immutable approved history, stale revision protection and owner isolation remain sound;
- real PostgreSQL concurrency coverage remains present;
- CI #110 is green.

TASK-010 / PR #21 has since merged into `develop` as `f731f4b9...`, followed by PM control-plane commits. GitHub currently reports PR #22 as mergeable, but the branch still has an old base SHA and has not run CI against the latest combined `develop` state.

Required action on the same dedicated TASK-011 worktree:
1. fetch latest `origin/develop`;
2. rebase or merge latest `develop` into the same branch;
3. preserve both accepted TASK-010 jobs foundation and TASK-011 Script behavior if any shared-file resolution is needed;
4. do not change frozen Script semantics unless the sync genuinely requires it;
5. rerun targeted Script tests + full CI;
6. push the same PR #22 for a final latest-base delta check.

This is a latest-base verification gate, not a new Script product finding.

## Goal
Start Creative Workflow Stage 5–6 by implementing the durable Script resource from an approved Proposal: versioned editable drafts, optimistic revision concurrency, immutable approval, PostgreSQL persistence and owner-isolated read/edit/approve APIs.

This task does not generate Script text with AI and does not create Scene Plans.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/contracts/SCRIPT_V1.md`
- `docs/contracts/AI_PROPOSAL_V1.md`
- `docs/product/CREATIVE_WORKFLOW.md`
- accepted Project/Proposal backend conventions.

## Frozen contract
`docs/contracts/SCRIPT_V1.md` is authoritative. Do not change it from this branch.

## Primary write paths
- `apps/api/internal/script/**`
- `apps/api/internal/postgres/script_repository*.go`
- `apps/api/internal/migrations/sql/0005_create_scripts.sql`
- Script-specific `apps/api/internal/httpserver/scripts*.go`.

## Allowed shared hotspots
- minimal Script service interface/route registration in `apps/api/internal/httpserver/server.go`;
- minimal service composition in `apps/api/cmd/api/main.go`.

## Reserved / do not touch
- accepted `apps/api/internal/jobs/**` and migration `0004` from TASK-010;
- `apps/web/**` — TASK-008;
- Proposal generation integration/job routes — TASK-009;
- provider registry/adapters;
- accepted Proposal schema/contract.

## Scope
- Script domain types/validation matching `SCRIPT_V1`.
- Project-scoped monotonically increasing Script versions.
- Internal `CreateDraft` operation requiring an owner-visible approved Proposal source version.
- Persist `source_proposal_version` and Project `content_locale` snapshot.
- At most one active Script draft; newer draft supersedes only the previous unapproved draft atomically.
- Draft optimistic `revision` update.
- `draft|approved|superseded` statuses.
- Atomic approval with expected revision and durable `approved_at`.
- Newest-first version list and full version get.
- Owner isolation/non-disclosure through Project principal boundary.
- HTTP GET list/version, PUT draft replacement, POST approve exactly as frozen contract.
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
- [ ] Migration `0005_create_scripts.sql` works through current runner on latest `develop`.
- [x] Script source must be an approved owner-visible Proposal.
- [x] Version/draft/approval invariants match `SCRIPT_V1`.
- [x] Owner isolation and concurrency are proven against real PostgreSQL.
- [x] Unicode/multibyte text respects frozen character limits.
- [x] Stable HTTP conflicts include `STALE_REVISION` and `SCRIPT_IMMUTABLE`.
- [x] No AI generation/provider call/Scene Plan/frontend implementation is introduced.
- [ ] Final post-sync CI on latest `develop` is green.
- [x] TDD evidence is truthful.

## Next dependencies
TASK-012 Script generation engine can build independently against frozen Script contracts. Later integration will persist generated candidates through this task's internal `CreateDraft`. Scene Plan work starts only from an approved Script version.

Do not self-merge or self-mark DONE.
