# Continue Protocol

This protocol defines what an AI coding agent must do when the user says only `tiếp tục`, `continue`, `go on`, `làm tiếp`, or equivalent wording without naming a task.

## Intent
Treat continuation as a **fresh remote repository-state command**, not as permission to invent work.

Before deciding what exists, follow `docs/engineering/CONTROL_PLANE_PROTOCOL.md`. Local checkout/ref absence is not proof of remote absence. Current development state is resolved against live GitHub plus refreshed `origin/develop`, canonical task branches and exact PR heads; `main` may intentionally lag.

## State machine

```text
CONTINUE
   |
   v
Inspect live issue/PR/remote branches + refresh origin/develop
   |
   v
Active/claimed task or PR exists?
   | yes
   v
Resolve exact PR/branch head + current task authorization/contract
   |
   v
Requested changes / comments / failing exact-head checks?
   | yes ----------------------------> dedicated worktree -> TDD/regression fix -> verify -> same PR
   | no
   v
Unfinished accepted task scope?
   | yes ----------------------------> dedicated worktree -> finish -> verify -> push/PR
   | no
   v
PR green/current and complete?
   | yes ----------------------------> await Team Lead; do not self-merge

No active task/PR
   |
   v
Inspect live authorized task issues + remote claim branches/PRs
   |
   v
Refresh origin/develop; read current BOARD/task specs/contracts
   |
   v
Executable PM-authorized tasks in safe order?
   |
   v
Skip any canonical remote branch/PR/local-ownership ambiguity
   |
   v
Atomically create absent canonical remote branch at selected origin/develop SHA
   | lost race ----------------------> re-fetch remote state -> next eligible task
   | success
   v
Create/attach dedicated task worktree -> TDD task-worker
   |
   no task --------------------------> report real blocker/claimed/no authorized task/priority ambiguity
```

## Priority
1. Latest actionable review feedback on the current PR head.
2. Failing required checks on the current PR head.
3. Unfinished current task acceptance criteria, provided the live issue has not blocked/cancelled/materially re-scoped it.
4. Team Lead handoff for a complete green PR.
5. A new unclaimed PM-authorized task in current PM order.

Never abandon an active review to start unrelated work unless PM explicitly permits independent parallel work.

## Freshness and claim rules
- Resolve live GitHub issue/PR/branch state before local state.
- Refresh `origin/develop` before reading versioned BOARD/task/contracts for a new selection.
- `READY` is authorization, not ownership. A canonical remote branch/active PR means claimed.
- A stale BOARD/task status is metadata drift; use `CONTROL_PLANE_PROTOCOL.md` rather than pretending fresher live state is absent.
- A material scope/branch/acceptance/contract conflict is contract drift and requires PM/Team Lead resolution.
- Atomic create-if-absent remote branch creation is the only new-task claim. Plain same-SHA push is not a lock.
- Never delete/take over an apparently abandoned claim; PM/Team Lead owns recovery.

## Filesystem rule
One implementation task = one dedicated Git worktree. The control checkout stays on `develop`. Never `git switch`, reset, clean, remove or reuse another task's worktree to move between concurrent tasks.

## TDD
New behavior and review fixes follow `docs/engineering/TDD_PROTOCOL.md` unless a task records a justified exception. A continuation command never authorizes fake historical RED evidence.

## Safety rails
Continuation does not authorize direct implementation on `develop`/`main`, self-merge, self-DONE, promotion of backlog/blocked work, takeover of another claim, silent material scope drift, destructive cleanup, or blind force-push.
