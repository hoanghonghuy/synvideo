# WAVE-F1-C — Proposal UI + Durable Jobs + Script Foundation

Status: ACTIVE
Milestone: F1 Creative Workflow

## Objective
Use three genuinely isolated developer slots while preserving the critical path:
1. finish the creator-facing Proposal review/approval workspace;
2. establish restart-safe durable async execution required by Proposal generation and later media/render work;
3. begin Script Stage 5–6 persistence from the already accepted Proposal domain.

## Active tasks
- Dev A — `TASK-008` AI Proposal frontend workspace — `IN_PROGRESS`, branch lock already exists.
- Dev B — `TASK-010` durable job execution foundation — `READY`.
- Dev C — `TASK-011` Script domain, persistence and approval API — `READY`.

## Frozen shared contracts
- `docs/contracts/AI_PROPOSAL_V1.md`
- `docs/contracts/JOB_EXECUTION_V1.md`
- `docs/contracts/SCRIPT_V1.md`

## Write-surface isolation
### TASK-008
Owns `apps/web/src/features/creative-proposal/**` plus minimal route/navigation/i18n additions. No backend edits.

### TASK-010
Owns `apps/api/internal/jobs/**`, job PostgreSQL repository and migration `0004_create_jobs.sql`. No generic public HTTP route and no Proposal/Script semantics.

### TASK-011
Owns `apps/api/internal/script/**`, Script repository/API and migration `0005_create_scripts.sql`, plus minimal backend route/composition hotspots. No jobs/frontend/generation implementation.

## Merge/dependency rules
- TASK-008, TASK-010 and TASK-011 are merge-order independent by primary ownership.
- TASK-009 remains BLOCKED until TASK-008 and TASK-010 are accepted. TASK-006/007 are already accepted.
- TASK-011 does not depend on live Proposal generation; it consumes the already accepted approved Proposal resource contract.
- Do not start Scene Plan implementation yet. Script generation/frontend can be planned as the next wave after a slot opens and SCRIPT_V1 implementation is validated.

## Migration reservation
- `0004_create_jobs.sql` — TASK-010 only.
- `0005_create_scripts.sql` — TASK-011 only.

Agents must not renumber or reuse these migrations from parallel branches.

## TDD / review
All implementation tasks follow `docs/engineering/TDD_PROTOCOL.md`. Team Lead reviews actual PR diff, CI and task-specific integration/concurrency evidence before merge.

## Next gate
When TASK-008 and TASK-010 are accepted, PM freezes Proposal-generation job request/payload/result semantics and may move TASK-009 READY. When a separate slot is available, PM may open the provider/BYOK and Script generation/frontend follow-on tasks without exceeding three active implementation slots.
