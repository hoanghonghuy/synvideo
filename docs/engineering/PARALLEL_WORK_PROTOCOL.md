# Parallel Developer Work Protocol

SynVideo may run multiple AI Developers in parallel, but parallelism is controlled by dependency, write-surface, **remote ownership isolation**, and—when applicable—**local filesystem isolation** rather than by keeping every agent busy.

`docs/engineering/CONTROL_PLANE_PROTOCOL.md` is authoritative for remote freshness, claimability, transition ordering, drift handling and abandoned-claim recovery.

`docs/engineering/EXECUTION_SUBSTRATES.md` defines the execution-substrate distinction used below.

## Isolation model

The universal cross-agent ownership boundary is:

**One implementation task = one canonical remote task branch.**

A dedicated Git worktree is additionally required only when the Developer runs on a shared/persistent local filesystem where multiple agents/tasks could interfere with the same checkout.

### Local/shared-filesystem Developer
- one active implementation task = one dedicated Git worktree + canonical remote branch;
- the primary/control checkout stays on `develop` and is used only for fetch/status/PM-control operations while concurrent agents are active;
- do not run multiple implementation tasks in the same working tree;
- never `git switch`, reset, clean, remove or reuse another task's worktree;
- review fixes reuse the existing task branch/worktree/PR.

### Scheduled/cloud/API-isolated Developer
- do not create or inspect Git worktrees merely to satisfy convention when no shared persistent filesystem exists;
- use the canonical remote task branch as the required ownership/isolation boundary;
- use the platform's isolated workspace, ephemeral checkout, connector/API file operations, or equivalent execution substrate;
- if the platform exposes a shared persistent checkout, apply the local/shared-filesystem rules above.

Remote branch locking and local worktrees solve different problems: the remote branch prevents cross-agent ownership races; a worktree protects machine-local filesystem state.

## Remote-first ownership
Before deciding whether a task is available:
1. inspect the live authoritative GitHub task issue;
2. inspect canonical remote task branch existence;
3. inspect active PRs for that task/branch;
4. resolve current `develop` and relevant remote refs;
5. only for a shared local filesystem, inspect local worktrees/branches for machine-local execution ambiguity.

A stale local ref or stale BOARD/task mirror is not proof that remote work is absent. `READY` authorizes execution but does not mean unclaimed. A canonical remote branch or active PR means claimed/owned.

## Waves
PM normally plans up to 3 independent implementation tasks. A task may share a wave only when prerequisite contracts are accepted/frozen, primary write paths do not materially overlap, shared hotspots are explicit, and merge/integration order is known.

## Path ownership
Every parallel task must declare:
- primary write paths;
- allowed shared integration files;
- reserved/do-not-touch paths.

If implementation unexpectedly needs a material change outside its declared write surface, stop and report the dependency instead of silently expanding scope.

## Contract-first parallelism
When frontend/backend or provider/consumer work in parallel, PM freezes the shared API/domain/event contract on `develop` before authorizing both tasks. A claimed task must not be silently re-scoped; material contract changes follow `CONTROL_PLANE_PROTOCOL.md` and must be surfaced to the active branch/PR owner.

## Atomic branch-as-lock task claiming
The canonical remote task branch is the cross-process/cross-machine claim only when created via atomic create-if-absent semantics.

### New-task claim workflow
1. Complete the remote-first ownership preflight above.
2. Consider only tasks whose authoritative issue currently authorizes execution, whose current `develop` task spec is executable, whose dependencies are satisfied and whose PM ordering permits selection.
3. Confirm the canonical remote task branch does not exist and no active PR represents the task.
4. For a shared local filesystem only, inspect `git worktree list --porcelain`; unresolved local ownership ambiguity blocks claiming on that machine.
5. Record the selected current `develop` SHA as `BASE_SHA` and the canonical branch as `BRANCH`.
6. Atomically create `refs/heads/$BRANCH` at `BASE_SHA` via GitHub create-ref/create-branch fail-if-exists semantics. **Plain same-SHA `git push` is not a concurrency lock.**
7. If create-ref says the branch already exists, this agent lost the race. Do not overwrite/delete/take over the winner. Re-fetch live remote state and choose the next eligible task.
8. After successful remote claim, initialize execution for the current substrate:
   - local/shared filesystem: fetch the branch and create/attach the dedicated task worktree tracking it;
   - scheduled/cloud/API-isolated: attach/use the platform's isolated workspace or remote branch editing mechanism; no synthetic worktree is required.
9. Verify branch ownership and execution isolation, then begin TDD implementation.

Equivalent authenticated CLI create-ref for a local execution environment:

```bash
REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)
BASE_SHA=$(git rev-parse origin/develop)
gh api --method POST "repos/$REPO/git/refs" \
  -f ref="refs/heads/$BRANCH" \
  -f sha="$BASE_SHA"
```

If the remote claim succeeds but local/workspace setup fails, the task remains claimed; fix/report setup rather than deleting the remote branch.

## Existing branch / PR
When a canonical branch or PR already exists, do not claim a replacement. Resolve the exact current remote PR/branch head and live task issue first, then continue using that canonical branch.

- Local/shared filesystem: locate/create the dedicated worktree for that existing branch and inspect uncommitted state before editing.
- Scheduled/cloud/API-isolated: reattach to the platform workspace or operate against the exact current remote branch/PR head; do not fabricate local worktree state.

Keep fixes on the same branch/PR.

## Abandoned claims
A generic coding agent never releases, deletes or takes over an existing canonical remote claim. PM/Team Lead recovery follows `CONTROL_PLANE_PROTOCOL.md`: verify no active PR/agent ownership, understand/preserve unmerged commits, then explicitly reassign the existing branch or release an empty/stale claim.

## Shared branch rules
- Never share one implementation branch between independent tasks.
- Never commit implementation directly to `develop` or `main`.
- Review fixes stay on the original task branch/PR and, when local, the original task worktree.
- Do not merge unrelated task branches into yours.
- Sync with `develop` only when required for integration; after sync, rerun relevant verification.
- Never run destructive cleanup against another active task path/ref/workspace.

## Merge strategy
Team Lead reviews each PR independently against its current task contract and exact current remote head. Independent accepted PRs may merge as soon as gates pass. When one PR provides runtime/contract needed by another, merge provider first, then refresh/sync the dependent branch and rerun integration verification.

After merge, clean local worktrees only when the task is confirmed merged/finished and no active process/uncommitted work depends on them. Ephemeral/cloud workspace cleanup is substrate-specific and must not delete useful unmerged remote work. Completion/control-plane housekeeping follows `CONTROL_PLANE_PROTOCOL.md`.

## PM responsibilities
PM owns dependency graph, wave composition, live authorization (`READY`/blocked/cancelled), frozen contracts, path boundaries, merge order, duplicate-task prevention, claimed-task re-scope decisions and abandoned-claim recovery. AI Developers own only their claimed canonical task branch and execution workspace. Team Lead owns acceptance, not implementation throughput.
