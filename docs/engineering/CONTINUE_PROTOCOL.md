# Continue Protocol

This protocol defines what an AI coding agent must do when the user says only `tiếp tục`, `continue`, `go on`, `làm tiếp`, or equivalent wording without naming a task.

## Intent

Treat continuation as a repository-state command, not as permission to invent work.

The agent must discover the next valid action from GitHub/repository state and follow the existing PM -> AI Developer -> PR -> Team Lead workflow.

## State machine

```text
CONTINUE
   |
   v
Active PR exists?
   | yes
   v
Requested changes / comments / failing CI?
   | yes ----------------------------> Fix current PR -> verify -> push -> same PR
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
Sync latest origin/develop
   |
   v
Read BOARD + open task issues
   |
   v
Highest-priority READY task with satisfied dependencies?
   | yes ----------------------------> Create task branch -> run task-worker workflow
   | no -----------------------------> Stop and report blocker/no executable task
```

## Priority

1. Review feedback on the active PR.
2. CI failures on the active PR.
3. Unfinished acceptance criteria for the active task.
4. Team Lead handoff for a complete green PR.
5. A new READY task from the PM-controlled task queue.

Never abandon an active review to start unrelated work unless the PM/task board explicitly allows parallel independent work.

## Sources to inspect

Only inspect what is needed to resolve state:
- current Git branch/working tree;
- current branch PR;
- PR reviews/comments and checks;
- current task issue/spec;
- `docs/tasks/BOARD.md`;
- `origin/develop` when taking a new task.

Do not recursively load all docs or source code.

## Safety rails

Continuation does **not** authorize:
- direct implementation commits to `develop` or `main`;
- self-merging a PR;
- marking a task `DONE`;
- turning BACKLOG/BLOCKED tasks into READY;
- broadening scope because the agent notices a nearby opportunity;
- force-pushing without a concrete branch-history reason.

If there is no valid next action, stopping with a concise status is the correct behavior.
