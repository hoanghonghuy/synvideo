---
name: synvideo-continue-workflow
description: Resume the next valid SynVideo engineering workflow step when the user says "continue", "tiếp tục", or otherwise asks to proceed without restating the task.
---

# SynVideo Continue Workflow

Use this skill when the user says `continue`, `tiếp tục`, `làm tiếp`, or an equivalent short request to resume work.

The command means: **inspect the current repository/workflow state and perform the next valid action without asking the user to repeat context.**

## State resolution order

Resolve state in this order. Earlier states have higher priority.

1. **Unfinished local work on the current task branch**
   - Inspect git status/current branch and the task contract.
   - Continue the current task; do not abandon it to take a new task.

2. **Open PR for the current task branch**
   - Read the latest PR review submissions, inline/top-level comments, and CI status.
   - If there are actionable review findings, fix them on the same branch, run verification, push, and update/reply to the PR as appropriate.
   - If CI failed, diagnose and fix the failure on the same branch, then rerun/push as needed.
   - If code changed after the latest review, treat the latest head as requiring review again.
   - If PR is green and has no actionable feedback but is still waiting for Team Lead acceptance, stop after reporting that it is awaiting review. Do not start another task on the same branch.

3. **Current task implementation is complete but has no PR**
   - Run the task's required verification.
   - Push the task branch if needed.
   - Open a PR to `develop` using the task/issue contract.
   - Never self-merge.

4. **Previous task PR has been merged**
   - Switch to `develop`.
   - Fetch/pull latest `origin/develop` with fast-forward only.
   - Re-read `docs/tasks/BOARD.md` and relevant open GitHub issues.
   - Select the highest-priority unblocked `READY` task.
   - Read its task spec, create the required dedicated branch, and invoke `synvideo-task-worker`.

5. **Idle on `develop` with no active task**
   - Sync latest `origin/develop`.
   - Read `docs/tasks/BOARD.md` and open implementation issues.
   - Take only a `READY` task whose dependencies are satisfied.
   - If no task is `READY`, report that there is currently no executable coding task. Do not invent work.

## Source priority

When deciding what to do next, use this precedence:

1. Current branch/task state.
2. Latest actionable Team Lead PR review/comment.
3. Failed CI on the current PR.
4. Current task spec / GitHub issue acceptance criteria.
5. `docs/tasks/BOARD.md` for the next task.
6. Product/engineering docs referenced by that task.

A newer Team Lead review may supersede an older review finding. Verify against the latest PR head instead of blindly replaying old comments.

## Git safety

- Never implement directly on `main` or `develop`.
- Never discard or overwrite uncommitted work merely to move to another task.
- Never use force-push unless the task/review explicitly requires history rewriting or it is necessary after an intentional rebase; use `--force-with-lease`, never blind `--force`.
- Never merge your own implementation PR.
- Never mark a task `DONE`; Team Lead/PM owns acceptance state.
- Do not continuously merge/rebase `develop` while an unrelated task is in progress.

## Expected behavior examples

### Example A: review feedback exists
User: `tiếp tục`

Current state: feature branch + open PR + Team Lead comments.

Action: read latest review and CI -> fix comments -> verify -> push same branch -> update PR.

### Example B: PR was merged
User: `tiếp tục`

Action: sync `develop` -> read board/issues -> pick next `READY` task -> create task branch -> implement it.

### Example C: PR is green and waiting for review
User: `tiếp tục`

Action: confirm there is no newer actionable review/CI failure -> report `waiting for Team Lead review`; do not create unrelated work on the active branch.

### Example D: idle repository
User: `tiếp tục`

Action: sync `develop` -> check board/issues -> start the next `READY` task, or report no READY work.

## Context discipline

Do not load all repository docs. Resolve workflow state first, then read only:
- root `AGENTS.md`;
- current task/PR/issue;
- `docs/tasks/BOARD.md` when selecting the next task;
- exact references required by that task.
