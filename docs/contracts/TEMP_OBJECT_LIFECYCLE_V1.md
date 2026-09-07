# TEMP_OBJECT_LIFECYCLE_V1

## Purpose
Define the durable lifecycle for non-MediaAsset object-storage intermediates used to recover asynchronous SynVideo jobs.

Temporary does not mean process-local: some intermediates intentionally survive worker/process failure. Because they may contain creator content and consume storage, their lifecycle must be durable, bounded and observable.

## Object classes
V1 begins with TASK-031 narration checkpoint chunks stored under a server-controlled project/job prefix. Future render/generation intermediates may join this contract only when their ownership and recovery requirements fit the same rules.

User-visible durable MediaAssets are excluded; they retain their own accepted deletion contract.

## Ownership identity
Every temporary object is attributable to a durable logical job and project using server-controlled identity. Object keys are implementation details and must never be accepted from an untrusted client as cleanup selectors.

Cleanup eligibility is derived from durable job/object metadata and policy, not directory-name guessing alone.

## Lifecycle states
Conceptually a temporary object belongs to one of these lifecycle conditions:
- `RECOVERABLE`: required by a job that may legitimately resume/retry;
- `CLEANUP_ELIGIBLE`: no longer required because the job completed, reached a terminal state, was superseded/cancelled, or exceeded the approved recovery-retention window;
- `CLEANUP_PENDING`: selected/claimed for cleanup, including prior failed cleanup;
- `REMOVED`: object absent and cleanup complete.

Exact persistence fields may differ, but behavior must preserve these distinctions durably enough for multi-instance correctness.

## Recovery invariant
Cleanup must fail safe toward preserving data whenever durable job state is ambiguous. A checkpoint used by an active/retryable job must not be removed before its approved recovery window closes.

Retry/resume of the same logical job may reuse retained checkpoints without recreating paid work unnecessarily.

## Cleanup convergence
Inline best-effort cleanup after successful work is allowed as an optimization, but it is not the sole correctness mechanism.

If inline deletion fails, the failure must remain observable/retryable and eligible objects must eventually be reconsidered by a durable reconciliation mechanism or equivalent provider-neutral expiry process.

Terminal failure after partial checkpointing must also enter the same bounded cleanup lifecycle; it cannot rely on a later successful handler path that will never execute.

Already-missing objects count as idempotently removed.

## READY policy freeze
Initial V1 policy values are frozen for implementation and remain environment-configurable:
- recoverable checkpoints for retryable/active narration jobs are preserved for at least **24 hours** from the last durable job/checkpoint activity unless the durable job remains legitimately active/retryable longer;
- completed, cancelled, superseded, terminal-failed or abandoned narration checkpoints become cleanup-eligible once no recovery invariant requires them and must not remain indefinitely; the initial maximum cleanup-eligible retention target is **72 hours**;
- one reconciliation pass claims at most **100** eligible records;
- one pass has a **30 second** wall-time budget;
- one object delete attempt has a **5 second** operation timeout;
- repeated cleanup failure remains durable/retryable and observable; it does not extend retention indefinitely as a correctness policy.

These are initial production-safe defaults, not API guarantees. Future topology evidence may tune configuration without weakening the lifecycle invariants.

## Operational execution model
V1 reuses the existing durable jobs infrastructure pattern rather than bucket scanning:
- PostgreSQL-backed application state is the source of cleanup eligibility and durable claiming;
- the application already has a lease-based, multi-instance-safe polling executor; cleanup may run as a dedicated bounded reconciliation worker/tick using the same durable claim/lease principles rather than being coupled to a single request path;
- S3-compatible object storage remains the object deletion boundary; provider bucket lifecycle rules are defense-in-depth only;
- no unbounded bucket namespace scan is permitted.

## Reconciliation / reaper
A cleanup execution must be bounded by batch size and/or runtime. It must be safe to run repeatedly and concurrently across instances using a durable claim/selection strategy where duplicate work could matter.

For each eligible record/object it should:
1. establish durable ownership and current cleanup eligibility;
2. attempt delete under a bounded operation context;
3. treat not-found as success;
4. record/retain retryable cleanup state when storage fails;
5. avoid deleting anything whose ownership/eligibility changed or cannot be established safely.

## Isolation and security
Cleanup is scoped by server-owned owner/project/job identity. A malformed/stale record must not permit prefix traversal or deletion across projects.

No user API exposes internal checkpoint bytes, raw storage credentials or arbitrary object-delete primitives.

## Privacy-safe observability
Operational telemetry may expose safe fields such as temporary-object class, job kind/state, age bucket, attempt/result category, counts and duration.

Do not log creator narration bytes, provider secrets, authorization material or unnecessary raw object keys.

## Failure semantics
Storage/transient cleanup failure is not the same as job-generation failure once user-visible output is safely committed. Cleanup can converge asynchronously, but the failure must remain visible to the cleanup subsystem rather than being silently discarded.

Corrupt or contradictory ownership/state must fail closed and surface an operational signal; destructive auto-repair is not the default.

## Required regression coverage
- partial narration checkpoints followed by terminal job failure become cleanup-eligible;
- successful final asset commit followed by storage-delete failure is retried/eventually converges;
- active/retryable job checkpoints survive cleanup passes;
- already-missing objects are idempotently accepted;
- expired/abandoned objects become eligible under bounded policy;
- cross-project/job cleanup is impossible;
- bounded batch/runtime behavior;
- concurrent cleanup workers do not violate eligibility or ownership;
- diagnostics avoid private payloads.

## READY-time validation result
Revalidated against protected `develop` `35ac2a5af8b19e47347c13fb4e91738023f0bbdf`: the concrete V1 durable temporary-object class remains narration `internal_chunks`; TASK-042 cancellation/lease-loss safety is accepted; current jobs infrastructure provides durable claim/lease/poll execution; current implementation WIP is below the normal maximum of three. With the policy and execution model above frozen, TASK-047 is eligible for READY activation after this governance change is accepted on protected `develop`.