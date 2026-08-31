# Continue Protocol

This protocol defines what an AI coding agent must do when the user says only `tiếp tục`, `continue`, `go on`, `làm tiếp`, or equivalent wording without naming a task.

## Intent
Treat continuation as a repository-state command, not as permission to invent work.

The agent must discover the next valid action from GitHub/repository state and follow the PM -> AI Developer -> PR -> Team Lead workflow.

## State machine

```text
CONTINUE
   |
   v
Active PR exists?
   | yes
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
Sync/fetch latest origin/develop + remote task branches
   |
   v
Read BOARD + open task issues
   |
   v
READY tasks with satisfied dependencies?
   |
   v
Skip tasks whose canonical remote branch/PR already exists (claimed by another dev)
   |
   v
Highest-priority unclaimed READY task?
   | yes ----------------------------> Create canonical branch -> push immediately as claim/lock -> TDD task-worker
   | no -----------------------------> Stop and report blocker/claimed/no executable task
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

1. Fetch remote branches before selecting work.
2. A canonical remote task branch or active PR means that task is already claimed.
3. Select only an unclaimed READY task whose dependencies are satisfied.
4. Create the exact canonical task branch from latest `origin/develop`.
5. Push it to origin immediately before implementation. This remote branch is the claim/lock.
6. If the push loses a race, do not overwrite/take over that branch. Re-fetch and select the next eligible task.

See `docs/engineering/PARALLEL_WORK_PROTOCOL.md` for wave/path-ownership rules.

## TDD
New behavior and review fixes follow `docs/engineering/TDD_PROTOCOL.md` unless a task records a justified exception.

A continuation command does not authorize test-after implementation or fake historical RED evidence.

## Sources to inspect
Only inspect what is needed to resolve state:
- current Git branch/working tree;
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
- self-merging a PR;
- marking a task `DONE`;
- turning BACKLOG/BLOCKED tasks into READY;
- taking over another developer's claimed branch;
- broadening scope because the agent notices a nearby opportunity;
- force-pushing without a concrete branch-history reason; use `--force-with-lease` when an intentional rebase requires it.

If there is no valid next action, stopping with a concise status is the correct behavior.
