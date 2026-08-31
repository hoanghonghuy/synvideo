---
name: synvideo-code-review
description: Review a SynVideo PR or diff as Team Lead for requirement compliance, bugs, architecture, regressions, tests, security, and acceptance readiness.
---

# SynVideo Code Review

## Use when
Reviewing an implementation PR/diff before merge to `develop`.

## Required inputs
- PR/diff and changed files.
- Corresponding task spec and acceptance criteria.
- Only product/engineering docs referenced by that task.
- `docs/engineering/REVIEW_CHECKLIST.md`.

## Review order
1. Verify scope and acceptance criteria.
2. Trace affected user flows and state transitions.
3. Look for correctness bugs, race/idempotency issues, data-loss paths and regression risk.
4. Check provider boundaries, secret handling, authorization and media/job lifecycle where relevant.
5. Check tests verify behavior rather than only implementation details.
6. Check the PR does not introduce unrelated changes or product behavior not approved by PM.

## Output
Classify findings by severity:
- BLOCKER: unsafe to merge.
- MAJOR: behavior/architecture materially wrong.
- MINOR: should fix but does not invalidate the main flow.
- NOTE: non-blocking observation.

End with one verdict: `APPROVE`, `REQUEST_CHANGES`, or `NEEDS_PRODUCT_DECISION`.
Do not mark the task DONE until required findings are resolved and acceptance criteria are satisfied.
