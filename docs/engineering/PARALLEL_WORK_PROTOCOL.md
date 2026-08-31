# Parallel Developer Work Protocol

SynVideo may run multiple AI Developers in parallel, but parallelism is controlled by dependency, write-surface **and filesystem isolation** rather than by keeping every agent busy.

## Non-negotiable filesystem isolation

**One implementation task = one dedicated Git worktree.**

A task branch is a Git ref; it does not isolate files. Multiple agents coding inside the same working tree share the same checked-out branch, index and uncommitted files. One agent running `git switch`, `reset`, `rebase`, checkout/restore operations or editing the same path can therefore corrupt or overwrite another agent's work even when their intended task branches are different.

Rules:
- The primary/control checkout should remain on `develop` and is used only for fetch/status/PM-control operations when concurrent agents are active.
- Do **not** run implementation work for multiple agents in that shared control checkout.
- Every claimed task must have a dedicated worktree bound to its canonical task branch before implementation starts.
- All edits, tests, commits, rebases and review fixes for that task happen inside that task's worktree.
- Never `git switch` the shared control checkout merely to move between concurrent tasks.
- Before using a path, inspect `git worktree list --porcelain`; never enter, delete, reset or reuse another task's worktree.
- A branch already checked out in another worktree belongs to that worktree. Reuse that existing task worktree for review fixes rather than trying to attach the branch somewhere else.

Recommended sibling layout (the exact parent path may vary):

```text
<workspace>/
├── synvideo/                 # control checkout, stays on develop
└── synvideo-worktrees/
    ├── TASK-008/             # feature/TASK-008-...
    ├── TASK-010/             # feature/TASK-010-...
    └── TASK-011/             # feature/TASK-011-...
```

If a task branch already legitimately exists for the current agent/task (for example an active PR needing fixes), fetch first and create or locate the dedicated worktree for that existing branch; do not create a replacement branch/PR.

## Waves
PM groups independent work into a small execution wave, normally up to 3 concurrent implementation tasks.

A task may enter the same wave only when:
- its prerequisite contracts are already accepted/frozen;
- its primary write paths do not materially overlap another task in the wave;
- shared integration files are explicitly identified;
- merge order and integration gates are known.

If those properties are not true, the tasks are sequential even if developers are available.

## Path ownership
Every parallel task must declare:
- **Primary write paths** — files/directories that task owns.
- **Allowed shared integration files** — small known hotspots it may need to edit.
- **Reserved / do-not-touch paths** — areas owned by another task in the wave.

If implementation unexpectedly requires a material change outside its declared write surface, stop and report the dependency instead of silently expanding the PR.

## Contract-first parallelism
When frontend and backend implement the same feature concurrently, PM freezes the API/domain contract on `develop` before both tasks start.

Both developers implement against that contract independently. The consumer task may use deterministic mocks in tests, but final acceptance requires an integration/smoke check against the real merged provider implementation when specified.

Contract changes during the wave require PM/Team Lead coordination; one developer must not silently redefine the shared contract in its branch.

## Atomic branch-as-lock task claiming

Multiple agents may receive the generic `tiếp tục` command at the same time. The canonical remote task branch is the cross-process/cross-machine task claim, but the claim operation must be an **atomic create-if-absent remote ref creation**.

Why: if two agents start from the same `origin/develop` SHA, two ordinary pushes of the same branch name can both appear successful/up-to-date. Even an update guard may be bypassed by a no-op same-SHA push. That is not an exclusive lock.

### New-task claim workflow

1. From the shared/control checkout on `develop`, fetch latest `origin/develop`, remote branches and worktree state.
2. Consider only `READY` tasks whose canonical remote branch does not already exist and has no active PR.
3. Confirm no local branch/worktree already represents that task. If local ownership is ambiguous, stop/report rather than deleting it.
4. Record `BASE_SHA=$(git rev-parse origin/develop)` and the canonical `BRANCH`.
5. **Atomically create the remote branch at `BASE_SHA` using GitHub's create-ref operation, which fails if that ref already exists.** Do this before creating the implementation worktree.

   With authenticated GitHub CLI, the equivalent is:

   ```bash
   REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)
   BASE_SHA=$(git rev-parse origin/develop)

   gh api --method POST "repos/$REPO/git/refs" \
     -f ref="refs/heads/$BRANCH" \
     -f sha="$BASE_SHA"
   ```

   A GitHub connector/API `create ref/branch` operation with the same fail-if-exists semantics is also valid.

   **Do not use plain `git push` as the concurrency claim.** The claim is successful only when this agent actually created the previously absent remote ref.
6. If create-ref reports that the ref already exists, this agent lost the race. Do not work on, overwrite or delete that branch. Re-fetch state and select another eligible READY task.
7. After a successful remote claim, fetch that branch and create the dedicated task worktree/local branch tracking it. For example:

   ```bash
   WORKTREE_ROOT="$(dirname "$(git rev-parse --show-toplevel)")/synvideo-worktrees"
   WORKTREE="$WORKTREE_ROOT/TASK-xxx"

   git fetch origin "$BRANCH"
   git worktree add -b "$BRANCH" "$WORKTREE" "origin/$BRANCH"
   ```

8. Verify from the task worktree that:
   - current branch is exactly the canonical task branch;
   - `HEAD` starts from the claimed `BASE_SHA` unless PM-approved upstream synchronization happened afterward;
   - worktree is clean;
   - upstream is `origin/$BRANCH`.
9. Only then begin TDD/implementation inside that dedicated worktree.

If remote claim succeeds but local worktree setup fails, the task remains claimed by this agent. Fix/report the local setup; do not silently release/delete the remote claim unless PM/Team Lead explicitly decides to release it.

An abandoned remote claim is released only by PM/Team Lead decision.

## Shared branch rules
- Never share one implementation branch between independent tasks.
- Never share one implementation working tree between independent tasks.
- Never commit implementation directly to `develop` or `main`.
- Review fixes remain on the original task branch **and its dedicated worktree**.
- Do not merge another task branch into yours to obtain unrelated work.
- Rebase/sync with `develop` when necessary for final integration after upstream dependencies merge; run that operation inside the task worktree and use `--force-with-lease` if an intentional rebase requires rewriting the remote task branch.
- Never run destructive cleanup (`reset --hard`, `clean`, worktree removal) against a path/ref that may belong to another active agent.

## Existing-branch / review-fix workflow

When an active PR already exists:
- identify the canonical branch from the PR/task;
- inspect `git worktree list --porcelain`;
- if that branch is already attached to a worktree, continue there;
- if it is not attached, fetch the branch and create a dedicated worktree for it;
- inspect the worktree for uncommitted changes before editing;
- keep all review fixes on the same branch/PR.

Do not switch a shared control checkout to the PR branch just because review work is required.

## Merge strategy
Team Lead reviews each PR independently against its task contract and TDD evidence.

For a wave:
- merge PRs with no dependency on other wave PRs as soon as accepted;
- when one PR provides a contract/runtime needed by another, merge the provider first, then rebase and run the consumer's integration verification;
- do not resolve conflicts by dropping another task's behavior merely to get a green merge.

After merge, local worktree cleanup may happen only when the task is confirmed merged/finished and no agent process or uncommitted work still depends on that worktree. Remote branch deletion remains a repository-policy/PM decision.

## PM responsibilities
PM owns:
- dependency graph and wave composition;
- READY/BLOCKED status;
- frozen shared contracts;
- path ownership boundaries;
- integration/merge order.

AI Developers own only the task they claimed and its dedicated worktree. Team Lead owns acceptance, not implementation throughput.
