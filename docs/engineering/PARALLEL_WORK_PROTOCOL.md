# Parallel Developer Work Protocol

SynVideo may run multiple AI Developers in parallel, but parallelism is controlled by dependency and write-surface isolation rather than by keeping every agent busy.

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

## Branch-as-lock task claiming
Multiple agents may receive the generic `tiếp tục` command at the same time. To avoid two agents claiming the same READY task:

1. Sync/fetch `origin/develop` and the remote branch list.
2. Consider only `READY` tasks whose canonical task branch does not already exist remotely and has no active PR.
3. Create the exact task branch from latest `origin/develop`.
4. **Push that new branch to origin immediately, before implementation.** The remote branch is the task claim/lock.
5. Re-fetch/verify that the remote branch points to the claiming agent's expected base before starting work.
6. If the branch already exists or the push loses the race, do not work on that task; select the next eligible READY task.

An abandoned claim is released only by PM/Team Lead decision (for example deleting/resetting the unused remote branch).

## Shared branch rules
- Never share one implementation branch between independent tasks.
- Never commit implementation directly to `develop` or `main`.
- Review fixes remain on the original task branch.
- Do not merge another task branch into yours to obtain unrelated work.
- Rebase/sync with `develop` when necessary for final integration after upstream dependencies merge; use `--force-with-lease` if an intentional rebase requires rewriting the remote task branch.

## Merge strategy
Team Lead reviews each PR independently against its task contract and TDD evidence.

For a wave:
- merge PRs with no dependency on other wave PRs as soon as accepted;
- when one PR provides a contract/runtime needed by another, merge the provider first, then rebase and run the consumer's integration verification;
- do not resolve conflicts by dropping another task's behavior merely to get a green merge.

## PM responsibilities
PM owns:
- dependency graph and wave composition;
- READY/BLOCKED status;
- frozen shared contracts;
- path ownership boundaries;
- integration/merge order.

AI Developers own only the task they claimed. Team Lead owns acceptance, not implementation throughput.
