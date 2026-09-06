# TASK-047 — Durable Temporary-Object Lifecycle & Orphan Cleanup

Status: ACTIVATION_PENDING
Priority: P1
Milestone: Production Readiness
Issue: #102
Canonical branch when activated: `feature/TASK-047-temp-object-lifecycle`
Contract: `docs/contracts/TEMP_OBJECT_LIFECYCLE_V1.md`

## Product outcome
Durable job intermediates remain available long enough for safe retry/resume, but completed, terminal or abandoned work cannot retain private creator content or consume object-storage capacity indefinitely.

## Evidence
TASK-031 narration recovery stores durable chunks under a server-owned `internal_chunks` prefix. Current cleanup suppresses object-delete failures and cleanup is only attempted after final asset persistence, so terminal failures after partial checkpointing and failed cleanup attempts can leave durable chunks without a bounded lifecycle.

Normal MediaAsset deletion is already tombstone/retry protected and is not the scope of this task.

## Scope
- Inventory non-MediaAsset durable temporary object classes, starting with narration checkpoints.
- Define lifecycle states/ownership and a bounded retention policy tied to durable job state.
- Preserve checkpoints required by active/retryable work.
- Make completion/terminal cleanup observable and retryable.
- Add an idempotent bounded reconciliation/reaper mechanism or equivalent durable expiry implementation.
- Keep cleanup project/job scoped using server-controlled prefixes and stable ownership metadata.
- Define privacy-safe cleanup diagnostics/metrics.
- Provide a reusable contract for future render/generation intermediates rather than feature-specific garbage-collection rules.

## Frozen activation policy
Initial V1 environment-configurable defaults:
- minimum recovery window: 24h for active/retryable narration checkpoints unless durable job state legitimately requires longer;
- maximum cleanup-eligible retention target: 72h;
- reconciliation batch: max 100 records per pass;
- reconciliation wall time: max 30s per pass;
- object delete timeout: 5s per attempt;
- PostgreSQL-backed durable state/claiming is authoritative; S3-compatible bucket lifecycle is defense-in-depth only;
- execution reuses the existing durable jobs lease/poll pattern as a bounded reconciliation worker/tick; no unbounded bucket scan.

## Required behavior
1. Temporary objects required by a currently active/retryable logical job are never removed before the defined recovery window closes.
2. Successful and terminal jobs eventually converge to their permitted retained-object state even when an inline storage-delete operation fails.
3. Abandoned temporary objects have a bounded retention/eligibility rule derived from durable state/time, not process memory.
4. Cleanup can be retried safely and does not fail if an eligible object is already absent.
5. Cleanup selection and deletion cannot cross owner/project/job boundaries.
6. A cleanup failure is observable; code must not report successful cleanup merely because an underlying delete error was ignored.
7. Cleanup processing is bounded per execution and safe across multiple instances/workers.
8. Private intermediate content and object keys are not unnecessarily logged or exposed through user APIs.

## Acceptance criteria
- `TEMP_OBJECT_LIFECYCLE_V1` is implemented and documented.
- Existing narration chunk recovery uses the lifecycle rather than best-effort-only silent cleanup.
- Terminal failure after partial checkpointing has deterministic eventual cleanup semantics.
- Storage-delete failure after successful finalization is retryable/observable and eventually converges.
- Retry/resume tests prove still-needed checkpoints remain available.
- Cross-project isolation and already-missing-object idempotency are covered.
- Cleanup/reconciliation has explicit bounded batch/runtime behavior and multi-instance-safe claiming/selection where necessary.
- Operational metrics/logs expose safe counts/statuses without generated audio/private payloads.
- Existing required CI remains green.

## Non-scope
- General MediaAsset delete semantics already owned by accepted media contracts.
- Production backup/restore, owned by TASK-044.
- User-facing trash/recycle-bin product behavior.
- Cloud-provider lifecycle rules as the sole correctness mechanism.

## Dependencies / relations
- TASK-031 is DONE and supplies the first durable temporary-object use case.
- TASK-042 is DONE and supplies bounded cancellation/lease-loss behavior.
- Durable job state from existing jobs infrastructure is the source of truth for recovery eligibility.
- TASK-039 may surface cleanup diagnostics.
- TASK-044 remains responsible for backup/restore and post-restore consistency.
- TASK-037 and future generation features should reuse this lifecycle for durable intermediates.

## Activation evidence — 2026-09-07
- exact protected `develop`: `35ac2a5af8b19e47347c13fb4e91738023f0bbdf`;
- concrete V1 temp-object inventory remains narration `internal_chunks`;
- current jobs executor provides PostgreSQL-backed claim/lease/poll semantics with bounded cancellation;
- no implementation PR for TASK-047 exists; prior PR #103 is planning-only;
- current board has one active implementation task (TASK-036) against normal max WIP 3, so an independent P1 slot is available;
- activation policy values and execution model are frozen in `TEMP_OBJECT_LIFECYCLE_V1`.

After this governance change is accepted on protected `develop`, issue #102 may move to `READY / CLAIMABLE`; Developer then owns `feature/TASK-047-temp-object-lifecycle`.

## TDD focus
Partial checkpoint + terminal failure, successful finalization + delete failure, retryable job preservation, expired/abandoned cleanup, idempotent missing objects, bounded batches, concurrent cleanup claim safety, and cross-project isolation.