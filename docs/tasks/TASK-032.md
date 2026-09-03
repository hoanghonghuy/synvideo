# TASK-032 — Per-scene AI Video Generation V1

Status: READY
Milestone: F1 Creative Workflow
Canonical branch: `feature/TASK-032-scene-video-generation`
Issue: #67
Depends on: TASK-025, TASK-028, MediaAsset + Scene Media Binding foundations
Frozen contract: `docs/contracts/SCENE_VIDEO_GENERATION_V1.md`

## Product outcome
Creator can generate video for one approved scene, survive long-running provider execution/restarts without duplicate paid submissions, obtain a durable generated-video MediaAsset, preview alternatives and explicitly assign/replace the scene visual.

## Scope
- Use the frozen provider-neutral scene-video contract.
- Implement the first live video adapter behind existing provider/runtime boundaries; Runway is the current V1 candidate from the 2026-09-03 official-doc revalidation, but provider/model identifiers remain configuration rather than domain enums.
- Snapshot approved scene intent before paid submission.
- Enforce exactly-once logical upstream submit: after successful submit, durably persist the opaque external operation identity and resume/poll that same operation across worker/browser restarts.
- Ambiguous submit outcomes must fail safely; never blindly issue another paid generation when success cannot be ruled out.
- Persist truthful provider/job status and recover through bounded polling/backoff/timeout semantics.
- Acquire the successful provider result into SynVideo-managed storage as a generated-video MediaAsset with provenance.
- Expose creator status/recovery/preview alternatives and explicit scene assign/replace flow.
- Preserve project/owner isolation and current Scene Media Binding history/idempotency semantics.

## Critical invariant
Retry/reclaim after successful upstream submission must not create a second provider video generation merely because in-memory state was lost. External operation identity is durable recovery state, not transient worker state.

## Non-scope
- Full editor or rendering pipeline.
- Batch/multi-scene orchestration.
- Arbitrary vendor passthrough fields in domain/API contracts.
- Channel publishing.
- Cost ledger/billing implementation.

## Acceptance criteria
1. An approved scene can submit one provider-neutral generation request using an enabled/configured supported video model.
2. Scene/source/provider/model/request parameters required for recovery are snapshotted durably before or atomically with the logical job transition.
3. A successful upstream submission persists its external operation ID before the workflow can safely proceed; retry/restart resumes that same operation rather than submitting again.
4. Ambiguous upstream-submit failure does not automatically create a second paid operation; the state/error is explicit and recoverable according to the frozen contract.
5. Polling/retrieve is bounded and maps provider terminal/retryable/timeout/rate-limit states into stable SynVideo job semantics without leaking credentials or raw sensitive payloads.
6. Successful output is downloaded/acquired into SynVideo-managed object storage and persisted as a generated-video MediaAsset with project/owner/provenance metadata.
7. Creator can recover status after refresh/restart, preview generated alternatives, and explicitly assign/replace the scene visual while preserving binding history.
8. Cross-owner/project access is rejected across submit/status/result/assignment paths.
9. Regression tests cover exactly-once logical submit, durable external-operation recovery, ambiguous submit safety, polling resume, MediaAsset ingestion, assignment idempotency and refresh recovery.
10. Required `Frontend`, `Backend`, and `Local Infrastructure` checks pass on the exact reviewed head.

## TDD / quality gate
Follow `docs/engineering/TDD_PROTOCOL.md`. Required evidence must demonstrate that the recovery tests fail when external-operation persistence/resume or ambiguous-submit safety is removed, not merely that tests exist after implementation.

## References / implementation constraints
- Internal contract wins: `docs/contracts/SCENE_VIDEO_GENERATION_V1.md`.
- Provider research was refreshed on 2026-09-03 before contract freeze. Runway asynchronous task ID + retrieve lifecycle currently maps cleanly to the contract; Google Veo long-running operations remain a follow-on/reference adapter.
- Revalidate the selected provider's official API availability, auth, model/status vocabulary, result expiry and rate-limit guidance during implementation if upstream docs changed since activation; do not redesign the provider-neutral contract casually for provider-specific convenience.

## Activation evidence
- TASK-031 is DONE via PR #77 / `develop` `6753f8878da989c09b9015dea2be089cd874defc`, releasing one implementation WIP slot.
- Frozen provider-neutral contract is already on protected `develop` via PR #78.
- Fresh duplicate checks found no canonical `feature/TASK-032-scene-video-generation` branch and no implementation PR for this outcome; only the historical PM contract branch exists.
- TASK-030 remains the other active implementation workstream; activating TASK-032 keeps the intended bounded parallelism rather than creating a third implementation task.

## Delivery
Developer claims by creating the canonical branch from the then-current `origin/develop` if it is still absent, implements there, and opens a PR into `develop`. Existing canonical branch/PR means already claimed. Developer must not self-merge or self-mark DONE.
