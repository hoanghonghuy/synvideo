# TASK-019 — Script creator workspace

Status: REVIEW
Milestone: F1 Creative Workflow
Wave: WAVE-F1-H
Branch: `feature/TASK-019-script-workspace`
Base: `develop`
PR: #47
Logically approved head: `5270f245e2fb5856fdcef53313bc87cee6e8541a`
Logical TL review: `5080179535`
CI: #216 green on logically approved head
Issue: #37
Depends on: TASK-011 and TASK-018 accepted; frozen `SCRIPT_WORKSPACE_V1` / `SCRIPT_JOB_V1`.

## Goal
Deliver the creator-facing Stage 5 Script workspace: history, editing, optimistic save, approval, source-staleness awareness, owner-scoped Generate/Regenerate, durable job recovery, and safe failure behavior.

## Authoritative contract
`docs/contracts/SCRIPT_WORKSPACE_V1.md`.

## Review result
All prior functional blockers are resolved on `5270f245...`:
- UUID generation now always produces a valid fresh UUIDv4 through `crypto.randomUUID()` or `crypto.getRandomValues`; unsupported secure-random environments fail safely without POSTing an invalid request ID;
- stale-revision saves preserve local edits and expose an explicit confirmation before destructive server reload;
- provider-empty/settings guidance is covered;
- approved/superseded history is read-only;
- optimistic save and approval success/conflict preservation are covered;
- terminal Retry creates a fresh valid UUID;
- terminal generation failure preserves the mounted Script;
- transient polling failure retries the same durable job without POSTing a replacement;
- succeeded jobs load exactly the returned Script version and retain that result when the read must be retried;
- session recovery stores only non-secret project/job identity.

## Final merge gate
TASK-018 merged after CI #216, so PR #47 must now sync/rebase onto latest `develop` and obtain a fresh exact-head green CI. GitHub currently reports the pre-sync PR as non-mergeable after the backend merge.

If the sync introduces no functional delta beyond conflict/rebase resolution, Team Lead only needs delta verification + exact-head CI before squash merge. Do not redesign or reopen already-accepted Script workspace behavior.

## Ownership boundary
Frontend-only:
- `apps/web/src/features/script/**`;
- minimal Script route / Project navigation;
- Script locale keys and unavoidable shared styles.

Do not modify backend/provider/media/Scene Plan behavior while performing the final sync.

## Worktree protocol
Continue only in the existing TASK-019 worktree/PR #47. Rebase/sync latest `origin/develop`, resolve only genuine conflicts, run frontend/full CI, and push. Do not self-merge or self-mark DONE.