---
name: synvideo-code-review
description: Review a SynVideo PR as Team Lead against the exact current remote head, current task/product contracts, tests, checks and acceptance gates.
---

# SynVideo Code Review

## Use when
Reviewing an implementation PR/diff before merge to `develop`.

Follow `docs/engineering/CONTROL_PLANE_PROTOCOL.md` before issuing a verdict.

## Remote-first required inputs
1. Resolve the exact current PR head SHA and verify the base is `develop`.
2. Fetch the latest remote diff/changed files for that exact head.
3. Fetch latest review submissions, unresolved/resolved threads, comments and checks for that exact head.
4. Read the live authoritative task issue.
5. Refresh current `origin/develop`, then read the corresponding task spec/acceptance criteria and only referenced product/engineering contracts from that baseline.
6. Read `docs/engineering/REVIEW_CHECKLIST.md`.

Never review from an old local branch/diff when a newer remote PR head exists. A review or green CI result from an older head is not evidence for the current head.

A stale BOARD/task status is metadata drift and does not supersede a fresher live PR/issue fact. A material task/product/acceptance contract conflict requires `NEEDS_PRODUCT_DECISION` rather than guessing.

## Review order
1. Confirm exact head/base/freshness and whether previous findings still apply to this head.
2. Verify scope and acceptance criteria.
3. Trace affected user flows and state transitions.
4. Look for correctness bugs, race/idempotency issues, data-loss paths and regression risk.
5. Check provider boundaries, secret handling, authorization and media/job lifecycle where relevant.
6. Check tests verify behavior rather than only implementation details and TDD/regression evidence is credible.
7. Check the PR does not introduce unrelated changes or product behavior not approved by PM.
8. Check parallel write-surface/frozen-contract integrity and current-base integration requirements.
9. Re-check required current-head checks before merge/acceptance.

## Review continuity
When re-reviewing after a push:
- resolve the new exact head SHA first;
- re-evaluate unresolved findings against that head instead of blindly replaying stale comments;
- confirm fixes did not create regressions or scope drift;
- anchor the new verdict to the exact head SHA.

If PM materially changed a claimed task, verify that the PR is being judged against the acknowledged current contract rather than an obsolete copy.

## Output
Classify findings by severity:
- BLOCKER: unsafe to merge.
- MAJOR: behavior/architecture materially wrong.
- MINOR: should fix but does not invalidate the main flow.
- NOTE: non-blocking observation.

End with exactly one verdict: `APPROVE`, `REQUEST_CHANGES`, or `NEEDS_PRODUCT_DECISION`, and identify the reviewed head SHA.

Do not mark the task `DONE` until required findings are resolved, acceptance criteria are satisfied, required exact-head/current-base checks are green, and the PR is actually merged. Completion housekeeping follows `CONTROL_PLANE_PROTOCOL.md`.
