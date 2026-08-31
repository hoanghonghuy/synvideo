# Team Lead Review Checklist

Apply relevant sections, not every item mechanically.

## Product correctness
- Does behavior match the task and approved product flow?
- Are all acceptance criteria demonstrated?
- Did implementation invent behavior outside PM scope?
- Are approval/revision checkpoints preserved where required?
- Does it work for intended short/long form rather than only demo inputs?

## TDD / regression evidence
- Does the PR include credible RED -> GREEN -> REFACTOR evidence for changed behavior?
- Were meaningful tests defined before implementation rather than added only after the fact to satisfy CI?
- Do tests assert observable behavior/risk instead of relying on snapshots or trivial coverage?
- Are persistence/concurrency/security boundaries covered at the appropriate integration level (real PostgreSQL where required)?
- If test-first was skipped, is the exception genuinely justified and is alternative automated verification sufficient?

## Parallel-work integrity
- Did the task stay inside its declared primary write paths and allowed shared integration files?
- Did it avoid reserved paths owned by another task in the wave?
- Does it still implement the frozen shared contract rather than silently redefining it?
- If another wave PR is a runtime dependency, was merge/rebase/integration verification done in the documented order?
- Were conflicts resolved by preserving both accepted behaviors rather than dropping another task's work?

## State/data integrity
- Are transitions explicit and valid?
- Can refresh/retry/restart lose accepted/generated work?
- Are writes atomic enough for the operation?
- Are duplicate callbacks/retries/idempotency considered?
- Are stale versions/races handled where concurrent jobs/users can update state?

## AI/provider integration
- Is domain code provider-neutral?
- Are provider failures/timeouts/rate limits surfaced safely?
- Are model capability differences represented explicitly?
- Is user/provider cost exposure considered for expensive actions?
- Are retries bounded and safe?

## Media / rendering
- Are asset IDs/provenance stable?
- Are temporary provider URLs handled appropriately?
- Are duration/aspect ratio/codec/size assumptions validated?
- Can partial generation be retried without restarting unrelated work?
- Does render/export failure preserve editable project state?

## Channel publishing
- Does implementation respect platform-specific capabilities/scopes?
- Is publish idempotent enough to avoid duplicate posts?
- Is scheduling/timezone behavior clear?
- Are disconnected/revoked accounts handled?
- Are external platform IDs/status persisted?

## Security/privacy
- Authentication and ownership checks exist on server-side resources.
- BYOK/API tokens are encrypted/secret and not logged/leaked to clients unnecessarily.
- URLs/uploads are validated against SSRF/content/storage risks as relevant.
- OAuth state/PKCE/token refresh/revocation are handled where relevant.

## UX / i18n
- No significant hard-coded user-facing copy outside locale resources.
- Loading, empty, error, disabled and retry states exist.
- Destructive/expensive actions communicate consequences.
- Basic accessibility and keyboard/focus behavior are not regressed.

## Quality
- Code fits repository architecture rather than creating a parallel pattern.
- No unrelated refactor is hidden in the task.
- Tests cover observable risky behavior and important failure paths.
- CI/typecheck/lint/build/task-specific checks pass.
- Migrations/config changes have rollback/deployment considerations.

## Verdict rule
`APPROVE` only when acceptance criteria are satisfied, required TDD/regression evidence is credible, parallel-wave boundaries/contracts are respected, and no BLOCKER/MAJOR finding remains.
