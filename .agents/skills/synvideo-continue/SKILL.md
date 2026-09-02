---
name: synvideo-continue
description: Resume SynVideo work from fresh remote GitHub/repository state when the user says "tiếp tục", "continue", "go on", or otherwise asks to proceed without naming a specific task.
---

# SynVideo Continue

Use this skill when the user asks to continue/proceed and does not give a more specific instruction.

The command means **inspect current remote repository/GitHub state and perform the next valid workflow action**. Do not ask the user to restate context when live issue/PR/branch/check state plus refreshed `origin/develop` can resolve it.

Follow `docs/engineering/CONTROL_PLANE_PROTOCOL.md` before deciding what work exists. Local Git state is execution state only; never infer remote absence from a stale local checkout/ref. `main` may lag development, so current work must be resolved against `develop`, the canonical task branch or exact PR head.

It does not authorize scope expansion, direct implementation commits to integration branches, self-merge, or invention of new product work.

## Remote freshness first
Before worktree/code decisions:
- inspect live GitHub task issue, canonical remote branch and active PR state;
- for an active PR, resolve its exact current head, latest reviews/threads/comments and checks;
- fetch/refresh `origin/develop` and remote task refs before reading BOARD/task/contracts;
- classify discrepancies using `CONTROL_PLANE_PROTOCOL.md`: metadata drift can be reconciled; material contract drift requires PM/Team Lead resolution.

## Worktree safety
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
Always resolve the highest applicable live state below. Do not skip an earlier state to start new work.

1. **Current PR has requested changes, actionable review comments, or failing checks**
   - Re-resolve the exact current remote PR head before editing; an older review/check is not proof about a newer head.
   - Identify/enter the dedicated worktree for the PR's canonical branch.
   - Inspect uncommitted work first so it is never discarded.
   - Evaluate the latest review/comments against the latest PR head and current task contract.
   - Fix only actionable findings that belong to the task; use TDD/regression discipline.
   - Run affected verification plus task-required checks, push to the same branch and update/reply on the same PR.

2. **Current claimed task/branch has unfinished implementation**
   - Inspect the live task issue first for block/cancel/re-scope signals.
   - Continue only inside the task's dedicated worktree.
   - Re-read the task contract from current `origin/develop`; if PM materially changed claimed scope, acknowledge/reconcile it before continuing.
   - Finish remaining acceptance criteria using `synvideo-task-worker` discipline.

3. **Current task is implemented and verified but has no PR**
   - Re-check live issue/remote branch state.
   - Push the same task branch if needed and open a PR to `develop` using the current task contract.
   - Never self-merge.

4. **Current PR is green and awaiting Team Lead acceptance**
   - Confirm the exact current head has no newer actionable review and the required checks are green.
   - Do not merge your own PR or mark the task `DONE`.
   - Do not start a dependent task; independent parallel work is allowed only when PM state permits it.

5. **Previous task PR has been merged / no active task or PR remains**
   - Use the control checkout on `develop`; do not repurpose another task worktree.
   - Inspect live open task issues, active PRs and canonical remote task branches first.
   - Fetch `origin`, refresh latest `origin/develop`, remote task refs and worktree state.
   - Read current `BOARD.md` and candidate task specs from refreshed `origin/develop`.
   - Consider tasks whose authoritative issue currently authorizes execution (normally `READY`), whose specs/dependencies are executable, and whose PM ordering permits selection.
   - Skip any task whose canonical remote branch already exists, has an active PR, or has unresolved local ownership ambiguity: it is claimed/ambiguous even if its issue still says `READY`.
   - Atomically create the absent canonical remote branch at the selected latest `origin/develop` SHA using create-ref/create-branch fail-if-exists semantics.
   - If the claim loses a race, never alter/delete the winner; re-fetch live state and try the next eligible task.
   - After successful claim, create/attach the dedicated worktree and execute with `synvideo-task-worker`.

6. **No executable unclaimed authorized task exists**
   - Stop and report the real reason: no authorized task, all candidates claimed, dependency/contract blocker, or unresolved priority ambiguity.
   - Never invent a feature, silently promote a blocked/backlog task, or treat stale local state as proof that there is no work.

## Git rules
- Implementation never goes directly to `main` or `develop`.
- New implementation starts from current `origin/develop` on a dedicated task branch and dedicated worktree.
- The canonical remote task branch is the cross-agent claim only when this agent successfully creates the previously absent ref atomically.
- Review fixes stay on the existing task branch/worktree/PR.
- Never discard/overwrite uncommitted work merely to move to another task.
- Force-push only when history rewriting is genuinely necessary; use `--force-with-lease`, never blind force.
- Never self-merge or self-mark `DONE`.
- Never delete/take over an apparently abandoned claim; PM/Team Lead owns recovery under `CONTROL_PLANE_PROTOCOL.md`.

## Interaction contract
Bare continuation phrases should trigger this workflow, including `tiếp tục`, `continue`, `go on`, `làm tiếp`, and `tiếp đi`. A more specific user instruction overrides this generic continuation workflow.
