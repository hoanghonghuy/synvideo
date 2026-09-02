# Remote Control-Plane Authority Protocol

This protocol defines how PM, AI Developers and Team Lead resolve repository state when GitHub, `develop`, task documents and local workspaces can change independently.

## Core invariant

**Shared workflow decisions are remote-first. Local Git state is an execution cache/workspace, never proof of current shared state.**

No agent may conclude that a task, branch, PR, review, check, dependency or product/control-plane change does not exist merely because it is absent from a local checkout or local remote-tracking refs.

Before making a workflow decision, refresh or directly inspect the relevant GitHub remote state. If current remote state cannot be established, do not infer absence from stale local data.

`main` is the stable/release branch and may intentionally lag development. Current development decisions must explicitly target `develop`, the canonical task branch, or the exact PR head. Generic GitHub code search that implicitly reads the repository default branch is not sufficient evidence for current `develop` state.

## Authority by concern

Different concerns have different authorities; there is no single file that wins every conflict.

| Concern | Authority |
|---|---|
| Approved product behavior | accepted product specs/ADRs at current `origin/develop` |
| Engineering/process constraints | `AGENTS.md` and `docs/engineering/**` at current `origin/develop` |
| Task scope, acceptance criteria, canonical branch, frozen contracts | `docs/tasks/TASK-XXX.md` at current `origin/develop` |
| PM execution authorization/lifecycle (`READY`, `BLOCKED`, `CANCELLED`, etc.) | authoritative GitHub task issue |
| Task claim / cross-agent ownership | canonical remote task branch, created atomically only-if-absent |
| Implementation under review | exact remote task branch / PR head SHA |
| Review state | latest PR head + latest review submissions/threads/comments + checks for that exact head |
| Merge/completion fact | merged PR plus PM/Team Lead issue closure/acceptance record |
| Queue overview and relative PM ordering | `docs/tasks/BOARD.md` at current `origin/develop` |
| Local branch/worktree/uncommitted files | local execution state only; never shared workflow authority |

`BOARD.md` and `Status:` lines inside task files are useful mirrors/indexes, but a stale mirror must not make an agent pretend that fresher live GitHub state does not exist.

## Freshness preflight

### PM
Before planning, activating, blocking, cancelling or completing work:
1. inspect the current remote `develop` head;
2. inspect relevant open and recently completed GitHub task issues;
3. inspect active/recent PRs and canonical task branches;
4. inspect the current task/contract files on `develop`;
5. resolve duplicate/ownership/dependency state before mutating the queue.

### AI Developer
Before selecting or continuing implementation:
1. inspect the live GitHub task issue, canonical remote branch and active PR state;
2. for an active PR, inspect its exact current head, latest reviews/comments and current checks;
3. fetch/refresh `origin/develop` and remote branch refs before reading versioned control-plane files;
4. read the task spec and referenced contracts from the refreshed `origin/develop` baseline;
5. only then inspect/use local branches and worktrees for execution.

Never reset, clean or overwrite legitimate local uncommitted work merely because remote state changed.

### Team Lead
Before reviewing or merging:
1. resolve the exact current PR head SHA and base branch;
2. read the latest PR diff/changed files from remote;
3. read current reviews, review threads/comments and checks for that exact head;
4. read the live task issue and the task/product/engineering contracts from current `origin/develop`;
5. anchor the verdict to the exact reviewed head SHA.

A review or green check from an older head is not reusable evidence after the PR head changes.

## Claimability

A new task is claimable only when all of the following are true:
- the authoritative GitHub issue currently authorizes execution (normally `READY`);
- the task spec exists on current `origin/develop` and provides an executable scope/branch/acceptance contract;
- live dependency facts satisfy the task gate;
- the canonical remote branch does not already exist;
- no active PR already represents the task;
- no known local task branch/worktree creates unresolved ownership ambiguity on the current machine.

Relative priority comes from the PM-controlled board/issue metadata. If multiple live `READY` tasks exist but the available remote control-plane data gives no safe ordering, report the ambiguity rather than inventing priority.

A `READY` issue does **not** mean unclaimed. Remote branch/PR state determines ownership.

## Drift classification

### Metadata drift — repair, do not hallucinate absence
Examples: stale BOARD status, stale task `Status:` line, old PR/head/CI note, issue missing from the board.

For the affected concern, the authority table above wins. PM/Team Lead should repair the mirror during housekeeping. Metadata drift alone does not erase a valid live task or PR.

### Contract drift — stop and resolve
Examples: conflicting task scope, different canonical branch names, incompatible acceptance criteria, missing required frozen contract, unresolved dependency semantics, product/ADR conflict.

Do not guess which behavior to build. PM/Team Lead must reconcile the contract before implementation proceeds.

### Local drift — refresh before deciding
A stale local `develop`, stale remote-tracking ref or missing local branch is not evidence that the corresponding remote object is absent.

## PM transition ordering

GitHub issues and versioned docs cannot be updated atomically, so use ordering that minimizes unsafe windows.

### Activate `READY`
1. finalize/freeze task spec, required contracts and relevant board ordering on `develop`;
2. verify the resulting remote `develop` SHA and dependencies;
3. update the authoritative GitHub issue to `READY` **last**. The issue transition is the live activation signal.

### Block or cancel unclaimed work
1. update the authoritative GitHub issue first so new claims stop;
2. then reconcile task/board mirrors on `develop`.

If a canonical branch was already claimed, do not silently delete/take it over; make an explicit PM/Team Lead stop/re-scope decision.

### Complete work
1. Team Lead accepts the exact PR head and required exact-head/current-base verification;
2. merge the PR;
3. close/update the authoritative issue with acceptance/merge evidence;
4. reconcile task and board mirrors and remove stale actionable instructions.

### Requested changes
The exact-head PR review is the live review authority. Issue/task/board status may mirror `CHANGES_REQUESTED`, but stale mirror text never supersedes a newer PR head/review.

## No silent mutation of claimed work

Once a task has a canonical remote claim branch, PM must not silently change its material scope, acceptance criteria, branch contract or frozen integration contract.

A material change must be recorded in the authoritative issue/task contract and surfaced to the active branch/PR owner. The developer must acknowledge/re-read the updated remote contract before continuing. If the change invalidates work already in progress, PM/Team Lead decides whether to re-scope, block, cancel or preserve/reassign the existing branch.

## Duplicate-task prevention

Before creating a new implementation task/issue, PM must search remote state for overlapping work across:
- open and recently completed task issues;
- existing `TASK-XXX.md` specs on `develop`;
- active and recently merged PRs;
- canonical task branches/claims;
- the same product outcome, domain/contract and material write surface.

If the same outcome already exists, update/reopen/extend/link the existing task when appropriate instead of creating a duplicate. If work only partially overlaps, record the dependency/boundary explicitly so two developers are not assigned the same outcome under different task IDs.

## Abandoned-claim recovery

A generic coding agent never deletes or takes over an existing canonical remote branch.

PM/Team Lead may recover an apparently abandoned claim only after checking:
- no active PR represents the branch;
- the branch head and unmerged commits are understood;
- no known active agent/worktree still owns it;
- any useful unmerged work is preserved.

If the branch contains useful work, prefer explicit reassignment of that existing branch. Delete/release an empty/stale claim only by an explicit PM/Team Lead decision, then re-run the normal atomic claim flow.

## Housekeeping invariant

After a merge, cancellation, re-scope or activation pass, PM/Team Lead should leave live issue state and versioned mirrors coherent. However, consumers must still follow the authority/freshness rules above rather than assuming mirrors can never drift.
