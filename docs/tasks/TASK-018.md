# TASK-018 — Script durable generation integration

Status: DONE
Milestone: F1 Creative Workflow
Wave: WAVE-F1-H
Branch: `feature/TASK-018-script-generation-integration`
Base: `develop`
PR: #48
Accepted head: `c5e682d83f939e983b7b2bf275a578b5487bf752`
Logical TL review: `5080177556`
CI: #215 green on accepted head
Squash merge: `6bc3c86b4cf8926dea8b62ecae5a71280de68730`
Issue: #36 completed

## Goal
Make Script generation a real asynchronous backend capability by integrating the accepted Script generation engine with durable jobs, owner-scoped BYOK runtime, crash-safe idempotent Script persistence, feature-specific HTTP endpoints, and runtime composition.

## Authoritative contract
`docs/contracts/SCRIPT_JOB_V1.md`.

## Accepted capability
- `POST /api/v1/projects/{id}/script-generations` and feature-specific generation status GET;
- durable job kind `script_generation_v1`;
- request-time Project + highest approved Proposal snapshot;
- owner-scoped TASK-017 credential/runtime resolution at worker execution;
- strict durable payload decoding and structural validation before provider resolution;
- request-id replay/conflict semantics and duplicate-enqueue race handling;
- database-idempotent generated Script persistence keyed by `source_generation_job_id`;
- request-time Project locale preserved in generated Script persistence even if Project locale changes after enqueue;
- crash-window retry returns the already-created Script instead of creating another version;
- real PostgreSQL same-generation-job concurrency proof;
- safe feature-specific status/result shape with no raw job/provider/credential data;
- Script generation handler registered into the accepted generic jobs executor alongside Proposal generation.

## Accepted invariants
1. HTTP POST never calls a provider.
2. Replay lookup precedes current Proposal/provider/credential checks.
3. Durable payload/result are credential-free.
4. Worker uses the snapshotted Project and approved Proposal intent.
5. `source_generation_job_id` is internal, DB-unique and absent from public Script JSON.
6. Approved Script history remains immutable; only the active unapproved draft follows accepted supersede semantics.
7. Context/lease lifecycle remains owned by `JOB_EXECUTION_V1`.
8. No second credential/provider runtime path was introduced.

## Verification
- exact-head CI #215 green;
- backend verification and race suite green;
- PostgreSQL locale-preservation and concurrent same-job integration tests accepted;
- review blockers from head `95e958f7...` resolved by `c5e682d8...`.

## Completion
Accepted and squash-merged. No further implementation work belongs on the TASK-018 branch. Clean its dedicated worktree/branch according to `PARALLEL_WORK_PROTOCOL.md`.