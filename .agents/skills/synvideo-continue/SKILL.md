---
name: synvideo-continue
description: Resume SynVideo work from repository state when the user says "tiếp tục", "continue", "go on", or otherwise asks to proceed without naming a specific task.
---

# SynVideo Continue

Use this skill when the user asks to continue/proceed and does not give a more specific instruction.

The command means **inspect current repository state and perform the next valid workflow action**. Do not ask the user to restate context when the active branch, PR, CI, task issue/spec, task board or Git worktree state can resolve it.

It does not authorize scope expansion, direct commits to protected integration branches, self-merge, or invention of new product work.

## Worktree safety first
Before modifying code, determine the current Git worktree and branch.

- One implementation task must run in one dedicated worktree.
- The shared/control checkout remains on `develop` while concurrent agents are active.
- Never `git switch` the shared control checkout to jump between concurrent task branches.
- Inspect `git worktree list --porcelain` before selecting/continuing work.
- If an existing task/PR branch already has a worktree, continue in that path.
- If an existing task/PR branch has no attached worktree, fetch it and create/attach a dedicated worktree before editing.
- Never reset, clean, remove or reuse a worktree that may belong to another agent.

`docs/engineering/PARALLEL_WORK_PROTOCOL.md` is authoritative for worktree and atomic-claim behavior.

## Priority order

Always resolve the highest applicable state below. Do not skip an earlier state to start new work.

1. **Current PR has requested changes, actionable review comments, or failing checks**
   - Identify/enter the dedicated worktree for the PR's canonical branch before editing.
   - Inspect current branch/working tree first so existing uncommitted work is never discarded.
   - Read the latest PR head, latest Team Lead review/comments, and current CI status/logs.
   - A newer review can supersede an older finding; evaluate comments against the latest PR head rather than blindly replaying stale feedback.
   - Fix only actionable findings that belong to the current task.
   - Follow the task's TDD/regression requirements for the fix.
   - Run the affected verification plus the task-required checks.
   - Push to the same task branch and update/reply to the same PR.
   - Do not create a replacement PR unless the existing PR is unusable.

2. **Current task/branch has unfinished implementation**
   - Continue only inside the task's dedicated worktree.
   - Re-read the task contract and current diff/state.
   - Continue only the remaining acceptance criteria using `synvideo-task-worker` discipline.
   - Verify, push, and open/update the PR to `develop` when complete.

3. **Current task is implemented and verified but has no PR**
   - From the task worktree, push the task branch if needed.
   - Open a PR to `develop` using the task/issue contract.
   - Never self-merge.

4. **Current PR is green and awaiting Team Lead acceptance**
   - Confirm there is no newer actionable review and no failing check.
   - Do not merge your own PR.
   - Do not mark the task `DONE`.
   - Do not start another task that depends on this PR.
   - Report that the PR is ready for Team Lead review and stop unless the PM-controlled board explicitly permits independent parallel work.

5. **Previous task PR has been merged / no active task or PR remains**
   - Return to/use the control checkout on `develop`; do not repurpose another task worktree.
   - Fetch `origin`, update local `develop` with fast-forward only, fetch remote task branches, and inspect `git worktree list --porcelain`.
   - Read `docs/tasks/BOARD.md` and relevant open GitHub task issues.
   - Consider READY tasks in PM priority order whose dependencies are satisfied.
   - Skip any task whose canonical remote branch already exists, already has an active PR, or is already represented by another local task worktree/branch: that task is claimed.
   - For the first unclaimed eligible task, read its spec and only its referenced docs.
   - Execute the claim protocol in `PARALLEL_WORK_PROTOCOL.md`: atomically create the canonical **remote** task branch at the selected latest `origin/develop` SHA using GitHub create-ref/create-branch fail-if-exists semantics.
   - A plain same-base `git push` is not a sufficient lock.
   - If remote create-ref loses a race, do not alter/delete the winning branch; re-fetch and try the next eligible READY task.
   - After a successful remote claim, create/attach the dedicated local worktree for that branch and execute the task using `synvideo-task-worker` there.

6. **No executable unclaimed READY task exists**
   - Stop.
   - Report that there is no executable unclaimed task and identify the blocker/claimed status if known.
   - Never invent a feature or silently promote a BACKLOG/BLOCKED task to READY.

## Repository status checks

Inspect only enough state to make the decision safely:
- current worktree path, branch and working tree;
- `git worktree list --porcelain`;
- active/open PR for the task branch;
- latest PR head, reviews and comments;
- CI/check status and failing logs;
- current task issue/spec;
- `docs/tasks/BOARD.md`;
- remote task branches/active PRs when selecting parallel READY work;
- latest `origin/develop` before taking a new task.

Do not recursively load all docs or source code. Do not repeatedly merge/rebase `develop` into an active unrelated task merely because `develop` changed.

## Git rules

- Implementation never goes directly to `main` or `develop`.
- New implementation starts from latest `origin/develop` on a dedicated task branch **and dedicated worktree**.
- The canonical remote task branch is the parallel-work claim only when this agent successfully creates the previously absent remote ref atomically.
- Review fixes stay on the existing task branch, worktree and PR.
- Never discard/overwrite uncommitted work merely to move to another task.
- Never `git switch` a shared control checkout between concurrent task branches.
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
