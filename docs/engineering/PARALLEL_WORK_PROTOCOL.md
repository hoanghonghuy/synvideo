# Parallel Developer Work Protocol

SynVideo may run multiple AI Developers in parallel, but parallelism is controlled by dependency, write-surface, **filesystem isolation** and **fresh remote ownership state** rather than by keeping every agent busy.

`docs/engineering/CONTROL_PLANE_PROTOCOL.md` is authoritative for remote freshness, claimability, transition ordering, drift handling and abandoned-claim recovery.

## Non-negotiable filesystem isolation
**One implementation task = one dedicated Git worktree.**

The primary/control checkout stays on `develop` and is used only for fetch/status/PM-control operations while concurrent agents are active. Do not run multiple implementation tasks in the same working tree. Never `git switch`, reset, clean, remove or reuse another task's worktree.

A branch already checked out in another worktree belongs to that worktree. Review fixes reuse the existing task branch/worktree/PR.

## Remote-first ownership
Before deciding whether a task is available:
1. inspect the live authoritative GitHub task issue;
2. inspect canonical remote task branch existence;
3. inspect active PRs for that task/branch;
4. refresh `origin/develop` and remote refs;
5. inspect local worktrees/branches only for machine-local execution ambiguity.

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
2. Consider only tasks whose authoritative issue currently authorizes execution, whose current `origin/develop` task spec is executable, whose dependencies are satisfied and whose PM ordering permits selection.
3. Confirm the canonical remote task branch does not exist and no active PR represents the task.
4. Inspect `git worktree list --porcelain`; unresolved local ownership ambiguity blocks claiming on this machine.
5. Record the selected current `origin/develop` SHA as `BASE_SHA` and the canonical branch as `BRANCH`.
6. Atomically create `refs/heads/$BRANCH` at `BASE_SHA` via GitHub create-ref/create-branch fail-if-exists semantics. **Plain same-SHA `git push` is not a concurrency lock.**
7. If create-ref says the branch already exists, this agent lost the race. Do not overwrite/delete/take over the winner. Re-fetch live remote state and choose the next eligible task.
8. After successful remote claim, fetch the branch and create/attach the dedicated worktree tracking it.
9. Verify branch/upstream/worktree cleanliness and begin TDD implementation only there.

Equivalent authenticated CLI create-ref:

```bash
REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)
BASE_SHA=$(git rev-parse origin/develop)
gh api --method POST "repos/$REPO/git/refs" \
  -f ref="refs/heads/$BRANCH" \
  -f sha="$BASE_SHA"
```

If the remote claim succeeds but local worktree setup fails, the task remains claimed; fix/report setup rather than deleting the remote branch.

## Existing branch / PR
When a canonical branch or PR already exists, do not claim a replacement. Resolve the exact current remote PR/branch head and live task issue first, then locate/create the dedicated worktree for that existing branch. Inspect uncommitted state before editing. Keep fixes on the same branch/PR.

## Abandoned claims
A generic coding agent never releases, deletes or takes over an existing canonical remote claim. PM/Team Lead recovery follows `CONTROL_PLANE_PROTOCOL.md`: verify no active PR/agent ownership, understand/preserve unmerged commits, then explicitly reassign the existing branch or release an empty/stale claim.

## Shared branch rules
- Never share one implementation branch between independent tasks.
- Never share one implementation working tree between independent tasks.
- Never commit implementation directly to `develop` or `main`.
- Review fixes stay on the original task branch/worktree/PR.
- Do not merge unrelated task branches into yours.
- Rebase/sync with `develop` only when required for integration; do it inside the task worktree and use `--force-with-lease` when intentional history rewrite is necessary.
- Never run destructive cleanup against another active task path/ref.

## Merge strategy
Team Lead reviews each PR independently against its current task contract and exact current remote head. Independent accepted PRs may merge as soon as gates pass. When one PR provides runtime/contract needed by another, merge provider first, then rebase and rerun dependent integration verification.

After merge, clean local worktrees only when the task is confirmed merged/finished and no active process/uncommitted work depends on them. Completion/control-plane housekeeping follows `CONTROL_PLANE_PROTOCOL.md`.

## PM responsibilities
PM owns dependency graph, wave composition, live authorization (`READY`/blocked/cancelled), frozen contracts, path boundaries, merge order, duplicate-task prevention, claimed-task re-scope decisions and abandoned-claim recovery. AI Developers own only their claimed task/worktree. Team Lead owns acceptance, not implementation throughput.
