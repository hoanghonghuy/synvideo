---
name: synvideo-task-worker
description: Implement exactly one SynVideo READY task in a dedicated Git worktree/branch, using atomic task claiming, minimal context, TDD, required verification, and a PR to develop.
---

# SynVideo Task Worker

## Use when
Implementing a PM-approved SynVideo task.

## Workflow
1. Read root `AGENTS.md`.
2. Read `docs/tasks/BOARD.md`.
3. Confirm the task is `READY`, its dependencies are satisfied, and identify its canonical branch/spec.
4. Read `docs/engineering/PARALLEL_WORK_PROTOCOL.md` before claiming. Fetch latest `origin/develop`, remote branches and `git worktree list --porcelain`. Confirm the canonical remote task branch does not already exist, there is no active PR, and another local worktree/local branch is not already using the task.
5. Record the selected `origin/develop` SHA and atomically create the canonical **remote** task branch at that SHA using GitHub create-ref/create-branch fail-if-exists semantics. A plain same-base `git push` is not a sufficient concurrency lock. If the remote ref already exists, this agent lost the race: do not alter it; re-fetch and choose another eligible READY task.
6. After the remote claim succeeds, fetch that canonical branch and create/attach a **dedicated task worktree** tracking it. The shared/control checkout must remain on `develop` while concurrent agents are active; never `git switch` it to the task branch.
7. Verify the task worktree is clean, on the canonical branch and tracking the claimed remote branch. From this point, **all edits, tests, commits, rebases and review fixes happen inside this worktree**.
8. Read only the task spec and the exact docs/contracts it references.
9. If the task builds a substantial subsystem, invoke the open-source research workflow first.
10. Follow `docs/engineering/TDD_PROTOCOL.md` for behavior changes: write the failing test first, make it pass minimally, then refactor with tests green.
11. Respect declared primary write paths, shared integration files and reserved paths. Never modify, reset, clean, remove or reuse another task's worktree.
12. Implement only the stated scope and acceptance criteria.
13. Run required targeted checks plus the task's repository verification from the task worktree.
14. Open/update a PR to `develop` with: scope, implementation summary, TDD evidence, tests, risks, assumptions and reuse/license notes if any.

## Existing task / review fixes
If the task already has a branch or PR, do **not** claim a new branch. Locate the branch's existing worktree with `git worktree list --porcelain`; if none exists, fetch and attach/create a dedicated worktree for that existing canonical branch. Inspect uncommitted state before editing and keep all fixes on the same branch/PR.

Never switch the shared control checkout to the PR branch just to perform review fixes.

## Stop conditions
Do not silently broaden scope. If a requirement materially conflicts with an accepted contract/ADR/spec, or a parallel task requires a write outside this task's declared surface, record the conflict for PM/Team Lead rather than inventing behavior.

Stop rather than touching another agent's filesystem state if the expected worktree/branch ownership is ambiguous.

If the remote claim succeeded but local worktree setup fails, the task remains claimed; fix/report the setup rather than deleting the remote branch without PM/Team Lead direction.

## Context discipline
Do not recursively read `docs/**`. Follow links from the current task only.
