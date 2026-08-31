# Continue Protocol

This protocol defines what an AI coding agent must do when the user says only `tiếp tục`, `continue`, `go on`, `làm tiếp`, or equivalent wording without naming a task.

## Intent
Treat continuation as a repository-state command, not as permission to invent work.

The agent must discover the next valid action from GitHub/repository state and follow the PM -> AI Developer -> PR -> Team Lead workflow.

## Filesystem rule
When implementation agents may run concurrently, **one task = one dedicated Git worktree**.

The shared/control checkout stays on `develop`. Do not `git switch` that checkout between task branches. Branch-as-lock prevents duplicate task ownership; the dedicated worktree prevents branch/index/uncommitted-file collisions. Both are required.

Before modifying code inspect:
- current worktree root/branch/status;
- `git worktree list --porcelain`;
- the canonical task branch/PR.

For an existing PR, continue in the branch's existing dedicated worktree, or attach/create one if none exists. Never reset/clean/remove another task's worktree.

See `docs/engineering/PARALLEL_WORK_PROTOCOL.md` for the authoritative worktree and atomic-claim protocol.

## State machine

```text
CONTINUE
   |
   v
Active PR exists?
   | yes
   v
Locate/enter that PR branch's dedicated worktree
   |
   v
Requested changes / comments / failing CI?
   | yes ----------------------------> TDD/regression fix -> verify -> push -> same PR
   | no
   v
PR green and implementation complete?
   | yes ----------------------------> Await Team Lead; do not self-merge
   | no
   v
Finish current task acceptance criteria -> verify -> push/PR

No active PR/task
   |
   v
Use control checkout on develop; sync/fetch origin/develop + remote branches + worktree state
   |
   v
Read BOARD + open task issues
   |
   v
READY tasks with satisfied dependencies?
   |
   v
Skip tasks whose canonical remote branch/PR/local task worktree already exists
   |
   v
Highest-priority unclaimed READY task?
   | yes
   v
Atomically create canonical remote branch at selected origin/develop SHA only-if-absent
   | lost race ----------------------> Re-fetch -> next READY task; never alter winner
   | success
   v
Create/attach dedicated local task worktree tracking claimed branch
   |
   v
TDD task-worker inside task worktree
   |
   no task --------------------------> Stop and report blocker/claimed/no executable task
```

## Priority
1. Review feedback on the active PR.
2. CI failures on the active PR.
3. Unfinished acceptance criteria for the active task.
4. Team Lead handoff for a complete green PR.
5. A new unclaimed READY task from the PM-controlled queue.

Never abandon an active review to start unrelated work unless the PM/task board explicitly allows parallel independent work.

## Parallel claim rule
When multiple developers receive `tiếp tục` concurrently:

1. Use the control checkout on `develop`; fetch remote branches and inspect worktrees before selecting work.
2. A canonical remote task branch, active PR, existing local task branch or task worktree means that task is already claimed/ambiguous and must not be silently taken over.
3. Select only an unclaimed READY task whose dependencies are satisfied.
4. Record the latest selected `origin/develop` SHA.
5. Atomically create the canonical **remote** task branch at that SHA using GitHub create-ref/create-branch fail-if-exists semantics.
6. **Do not rely on a plain same-SHA `git push` as the lock**: two agents starting from the same base can otherwise both observe apparent success/up-to-date.
7. If remote create-ref loses a race, do not overwrite/take over/delete the winner's branch. Re-fetch and select the next eligible task.
8. Only after successful remote claim, create/attach the dedicated local worktree for the canonical branch.
9. Perform all implementation and Git history changes inside that dedicated task worktree.

See `docs/engineering/PARALLEL_WORK_PROTOCOL.md` for exact GitHub create-ref, wave, path and worktree rules.

## TDD
New behavior and review fixes follow `docs/engineering/TDD_PROTOCOL.md` unless a task records a justified exception.

A continuation command does not authorize test-after implementation or fake historical RED evidence.

## Sources to inspect
Only inspect what is needed to resolve state:
- current Git worktree/branch/working tree;
- `git worktree list --porcelain`;
- current branch PR;
- PR reviews/comments and checks;
- current task issue/spec;
- `docs/tasks/BOARD.md`;
- remote task branches/active PRs when selecting parallel work;
- `origin/develop` when taking a new task.

Do not recursively load all docs or source code.

## Safety rails
Continuation does **not** authorize:
- direct implementation commits to `develop` or `main`;
- using one working tree for multiple concurrent implementation tasks;
- switching a shared control checkout among concurrent task branches;
- self-merging a PR;
- marking a task `DONE`;
- turning BACKLOG/BLOCKED tasks into READY;
- taking over another developer's claimed branch/worktree;
- broadening scope because the agent notices a nearby opportunity;
- destructive reset/clean/worktree removal against another task;
- blind force-pushing; use `--force-with-lease` when an intentional rebase requires it.

If there is no valid next action, stopping with a concise status is the correct behavior.
