---
name: synvideo-task-worker
description: Implement exactly one PM-authorized SynVideo task in a dedicated Git worktree/branch using remote-first freshness, atomic task claiming, minimal context, TDD, required verification, and a PR to develop.
---

# SynVideo Task Worker

## Use when
Implementing a PM-approved SynVideo task.

## Remote-first preflight
Before deciding whether work exists or is claimable, follow `docs/engineering/CONTROL_PLANE_PROTOCOL.md`.

- Inspect the live GitHub task issue, canonical remote branch and active PR state first.
- Fetch/refresh latest `origin/develop` and remote task refs before reading versioned control-plane files.
- Read `BOARD.md`, task spec and referenced contracts from current `origin/develop`, not from an unrefreshed local checkout.
- Local absence is never proof of remote absence.
- `main` may lag development; do not use default-branch code search as evidence of current `develop` task state.

## Workflow
1. Read root `AGENTS.md` and `docs/engineering/CONTROL_PLANE_PROTOCOL.md`.
2. Inspect live GitHub task issues, active PRs and canonical remote task branches relevant to the candidate task.
3. Fetch latest `origin/develop`/remote refs, then read `docs/tasks/BOARD.md` and the selected task spec from that refreshed baseline.
4. Confirm the authoritative issue currently authorizes execution (normally `READY`), the task spec is executable, dependencies are satisfied, and PM ordering permits taking it.
5. Read `docs/engineering/PARALLEL_WORK_PROTOCOL.md`. Inspect `git worktree list --porcelain`. Confirm the canonical remote task branch does not already exist, there is no active PR, and another local worktree/local branch is not already using the task.
6. Record the selected `origin/develop` SHA and atomically create the canonical **remote** task branch at that SHA using GitHub create-ref/create-branch fail-if-exists semantics. A plain same-base `git push` is not a sufficient concurrency lock. If the remote ref already exists, this agent lost the race: do not alter it; re-fetch live remote state and choose another eligible task.
7. After the remote claim succeeds, fetch that canonical branch and create/attach a **dedicated task worktree** tracking it. The shared/control checkout must remain on `develop` while concurrent agents are active; never `git switch` it to the task branch.
8. Verify the task worktree is clean, on the canonical branch and tracking the claimed remote branch. From this point, **all edits, tests, commits, rebases and review fixes happen inside this worktree**.
9. Re-read the current task contract and only the exact docs/contracts it references. If PM materially changes the claimed task contract, acknowledge/re-read the remote change before continuing; do not silently implement against the old local copy.
10. If the task builds a substantial subsystem, invoke the open-source research workflow first.
11. Follow `docs/engineering/TDD_PROTOCOL.md` for behavior changes: write the failing test first, make it pass minimally, then refactor with tests green.
12. Respect declared primary write paths, shared integration files and reserved paths. Never modify, reset, clean, remove or reuse another task's worktree.
13. Implement only the stated scope and acceptance criteria.
14. Run required targeted checks plus the task's repository verification from the task worktree.
15. Before opening/updating the PR, re-inspect the live task issue for cancellation/block/re-scope signals and the current remote branch state.
16. Open/update a PR to `develop` with: scope, implementation summary, TDD evidence, tests, risks, assumptions and reuse/license notes if any.

## Existing task / review fixes
If the task already has a canonical remote branch or PR, do **not** claim a new branch. Resolve the latest remote PR head/reviews/comments/checks and live issue state first. Locate the branch's existing worktree with `git worktree list --porcelain`; if none exists, fetch and attach/create a dedicated worktree for that existing canonical branch. Inspect uncommitted state before editing and keep all fixes on the same branch/PR.

A stale BOARD/task status does not erase a valid live PR/branch. A material scope/acceptance/contract conflict is different: stop and have PM/Team Lead reconcile it before implementing further.

Never switch the shared control checkout to the PR branch just to perform review fixes.

## Stop conditions
Do not silently broaden scope. If a requirement materially conflicts with an accepted contract/ADR/spec, or a parallel task requires a write outside this task's declared surface, record the conflict for PM/Team Lead rather than inventing behavior.

Stop rather than touching another agent's filesystem state if the expected worktree/branch ownership is ambiguous.

If the remote claim succeeded but local worktree setup fails, the task remains claimed; fix/report the setup rather than deleting the remote branch without PM/Team Lead direction.

If an existing claim appears abandoned, do not delete/take it over. PM/Team Lead owns recovery under `CONTROL_PLANE_PROTOCOL.md`.

## Context discipline
Do not recursively read `docs/**`. Follow links from the current task only after the remote freshness preflight.
