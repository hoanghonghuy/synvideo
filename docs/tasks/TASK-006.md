# TASK-006 — AI Proposal domain, persistence and approval API

Status: READY
Milestone: F1 Creative Workflow
Depends on: TASK-003 accepted
Wave: WAVE-F1-B
Branch: `feature/TASK-006-ai-proposal-persistence`
Base: `develop`

## Goal
Implement the durable AI Proposal resource: Project-scoped versions, editable draft revision concurrency, immutable approval, PostgreSQL persistence and owner-isolated HTTP read/edit/approve APIs.

## Read first
- `AGENTS.md`
- `docs/engineering/TDD_PROTOCOL.md`
- `docs/engineering/PARALLEL_WORK_PROTOCOL.md`
- `docs/contracts/AI_PROPOSAL_V1.md`
- `docs/contracts/CREATIVE_BRIEF_V1.md`
- `docs/product/CREATIVE_WORKFLOW.md`
- accepted Project/Creative Brief backend conventions

## Contract
`docs/contracts/AI_PROPOSAL_V1.md` is frozen. Do not change it from this branch.

This task provides persistence/application behavior only. It does not call an AI provider and does not expose a generation endpoint.

## Primary write paths
- `apps/api/internal/creativeproposal/**`
- `apps/api/internal/postgres/creative_proposals*.go`
- `apps/api/internal/migrations/sql/0003_create_creative_proposals.sql`
- Proposal-specific `apps/api/internal/httpserver/creative_proposals*.go`

## Allowed shared hotspots
- minimal Proposal service interface/route registration in `apps/api/internal/httpserver/server.go`;
- minimal service composition in `apps/api/cmd/api/main.go`.

## Reserved / do not touch
- `apps/api/internal/proposalgeneration/**` — TASK-007.
- `apps/web/**` — TASK-008.
- accepted provider package internals.
- Creative Brief contract/schema except reading its revision relationship.

## Scope
- Proposal domain types/validation matching frozen editable content.
- Project-scoped monotonically increasing versions.
- Internal `CreateDraft` application/repository operation for later generation integration.
- Optimistic `revision` on draft edits.
- Status `draft|approved|superseded` and immutable non-draft behavior.
- Atomic approval with expected revision and durable `approved_at`.
- Version listing newest first and full version retrieval.
- Owner isolation through Project/principal boundary; no client-supplied owner id.
- HTTP GET list/version, PUT draft edit, POST approve routes exactly as contract.
- Real PostgreSQL integration/concurrency coverage.

## Important invariants
- approved content cannot be mutated;
- new draft allocation cannot overwrite an approved version;
- two concurrent draft creations cannot allocate the same version;
- stale edit/approval cannot silently win;
- creating a new draft must supersede the previous active unapproved draft atomically, leaving at most one active draft per Project;
- project/cross-owner existence remains non-disclosing.

## TDD plan
Start RED before implementation for at least:
1. create first draft -> version 1/revision 1/status draft;
2. create next draft -> monotonic version and prior unapproved draft superseded;
3. approved version remains immutable and is never superseded;
4. draft update with current revision increments exactly once;
5. stale update returns `STALE_REVISION`;
6. approval with current revision is atomic and durable;
7. stale/concurrent approval/update cannot both silently succeed;
8. owner A cannot list/get/update/approve owner B Proposal;
9. versions list newest first;
10. validation rejects frozen-contract violations.

## Acceptance criteria
- [ ] PostgreSQL migration is transactional/idempotent under existing migration runner.
- [ ] Domain/persistence/API match `AI_PROPOSAL_V1.md` exactly.
- [ ] `CreateDraft` exists for TASK-009 without public manual-create endpoint.
- [ ] Draft concurrency and immutable approval are proven against real PostgreSQL.
- [ ] Owner isolation is tested for list/get/update/approve.
- [ ] Stable HTTP errors include `STALE_REVISION` and `PROPOSAL_IMMUTABLE`.
- [ ] No AI provider call/vendor SDK/generation prompt is introduced.
- [ ] TDD evidence is truthful and CI green.

## Verification
At minimum:
- targeted domain/service tests;
- real PostgreSQL integration/concurrency tests;
- `gofmt`, `go vet ./...`, `go test ./...`, backend build;
- full repository verification and `git diff --check`.

## Merge order
Independent from TASK-007 implementation. TASK-008 may develop against the frozen HTTP contract but must run its final real backend smoke after TASK-006 merges.

Do not self-merge or self-mark DONE.
