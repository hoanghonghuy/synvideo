---
name: synvideo-continue
description: Resume SynVideo work from repository state when the user says "tiếp tục", "continue", "go on", or otherwise asks to proceed without naming a specific task.
---

# SynVideo Continue

Use this skill when the user asks to continue/proceed and does not give a more specific instruction.

The command means **inspect current repository state and perform the next valid workflow action**. It does not authorize scope expansion, direct commits to protected integration branches, self-merge, or invention of new product work.

## Priority order

Always resolve the highest applicable state below. Do not skip an earlier state to start new work.

1. **Current PR has requested changes, review comments, or failing checks**
   - Read the latest PR review/comments and current CI status/logs.
   - Fix only actionable findings that belong to the current task.
   - Run the affected verification plus the task-required checks.
   - Push to the same task branch and update the same PR.
   - Do not create a replacement PR unless the existing PR is unusable.

2. **Current task/branch has unfinished implementation**
   - Re-read the task contract and current diff/state.
   - Continue only the remaining acceptance criteria.
   - Verify, push, and open/update the PR to `develop` when complete.

3. **Current PR is green and awaiting Team Lead acceptance**
   - Do not merge your own PR.
   - Do not start another task that depends on this PR.
   - Report that the PR is ready for Team Lead review and stop unless another explicitly independent READY task is permitted by the board.

4. **No active task/PR**
   - Fetch `origin` and update local `develop` with fast-forward only.
   - Read `docs/tasks/BOARD.md` and relevant open GitHub task issues.
   - Select the highest-priority `READY` task whose dependencies are satisfied.
   - Read that task's spec and only its referenced docs.
   - Create the required dedicated task branch from latest `develop`.
   - Execute it using `synvideo-task-worker`.

5. **No executable READY task exists**
   - Stop.
   - Report that there is no executable task and identify the blocker/status if known.
   - Never invent a feature or silently promote a BACKLOG/BLOCKED task to READY.

## Repository status checks

When available, inspect enough state to make the decision safely:
- current branch and working tree;
- active/open PR for the branch;
- latest PR reviews and comments;
- CI/check status and failing logs;
- current task issue/spec;
- `docs/tasks/BOARD.md`;
- latest `origin/develop` before taking a new task.

Do not repeatedly merge/rebase `develop` into an active unrelated task merely because `develop` changed.

## Git rules

- Implementation never goes directly to `main` or `develop`.
- New implementation starts from latest `origin/develop` on a dedicated task branch.
- Review fixes stay on the existing task branch and PR.
- Force-push only when necessary to repair/rebase the task branch and only after understanding that it rewrites branch history.
- Never self-merge a PR.

## Interaction contract

A bare continuation phrase such as these should trigger this workflow:
- `tiếp tục`
- `continue`
- `go on`
- `làm tiếp`
- `tiếp đi`

A more specific user instruction always overrides this generic continuation workflow.
