---
name: synvideo-continue
description: Resume SynVideo work from repository state when the user says "tiếp tục", "continue", "go on", or otherwise asks to proceed without naming a specific task.
---

# SynVideo Continue

Use this skill when the user asks to continue/proceed and does not give a more specific instruction.

The command means **inspect current repository state and perform the next valid workflow action**. Do not ask the user to restate context when the active branch, PR, CI, task issue/spec, or task board can resolve it.

It does not authorize scope expansion, direct commits to protected integration branches, self-merge, or invention of new product work.

## Priority order

Always resolve the highest applicable state below. Do not skip an earlier state to start new work.

1. **Current PR has requested changes, actionable review comments, or failing checks**
   - Inspect current branch/working tree first so existing uncommitted work is never discarded.
   - Read the latest PR head, latest Team Lead review/comments, and current CI status/logs.
   - A newer review can supersede an older finding; evaluate comments against the latest PR head rather than blindly replaying stale feedback.
   - Fix only actionable findings that belong to the current task.
   - Run the affected verification plus the task-required checks.
   - Push to the same task branch and update/reply to the same PR.
   - Do not create a replacement PR unless the existing PR is unusable.

2. **Current task/branch has unfinished implementation**
   - Re-read the task contract and current diff/state.
   - Continue only the remaining acceptance criteria.
   - Verify, push, and open/update the PR to `develop` when complete.

3. **Current task is implemented and verified but has no PR**
   - Push the task branch if needed.
   - Open a PR to `develop` using the task/issue contract.
   - Never self-merge.

4. **Current PR is green and awaiting Team Lead acceptance**
   - Confirm there is no newer actionable review and no failing check.
   - Do not merge your own PR.
   - Do not mark the task `DONE`.
   - Do not start another task that depends on this PR.
   - Report that the PR is ready for Team Lead review and stop unless the PM-controlled board explicitly permits independent parallel work.

5. **Previous task PR has been merged / no active task or PR remains**
   - Switch to `develop`.
   - Fetch `origin` and update local `develop` with fast-forward only.
   - Read `docs/tasks/BOARD.md` and relevant open GitHub task issues.
   - Select the highest-priority `READY` task whose dependencies are satisfied.
   - Read that task's spec and only its referenced docs.
   - Create the required dedicated task branch from latest `develop`.
   - Execute it using `synvideo-task-worker`.

6. **No executable READY task exists**
   - Stop.
   - Report that there is no executable task and identify the blocker/status if known.
   - Never invent a feature or silently promote a BACKLOG/BLOCKED task to READY.

## Repository status checks

Inspect only enough state to make the decision safely:
- current branch and working tree;
- active/open PR for the branch;
- latest PR head, reviews and comments;
- CI/check status and failing logs;
- current task issue/spec;
- `docs/tasks/BOARD.md`;
- latest `origin/develop` before taking a new task.

Do not recursively load all docs or source code. Do not repeatedly merge/rebase `develop` into an active unrelated task merely because `develop` changed.

## Git rules

- Implementation never goes directly to `main` or `develop`.
- New implementation starts from latest `origin/develop` on a dedicated task branch.
- Review fixes stay on the existing task branch and PR.
- Never discard/overwrite uncommitted work merely to move to another task.
- Force-push only when branch-history rewriting is genuinely necessary; prefer `git push --force-with-lease`, never blind `git push --force`.
- Never self-merge a PR.
- Never mark a task `DONE`; PM/Team Lead owns acceptance state.

## Interaction contract

Bare continuation phrases should trigger this workflow, including:
- `tiếp tục`
- `continue`
- `go on`
- `làm tiếp`
- `tiếp đi`

A more specific user instruction always overrides this generic continuation workflow.
