---
name: synvideo-task-worker
description: Implement exactly one PM-authorized SynVideo task using remote-first freshness, atomic task claiming, substrate-appropriate execution isolation, minimal context, TDD, required verification, and a PR to develop.
---

# SynVideo Task Worker

## Use when
Implementing a PM-approved SynVideo task.

## Remote-first preflight
Before deciding whether work exists or is claimable, follow `docs/engineering/CONTROL_PLANE_PROTOCOL.md`.

- Inspect the live GitHub task issue, canonical remote branch and active PR state first.
- Resolve current `develop` and remote task refs before reading versioned control-plane files.
- Read `BOARD.md`, task spec and referenced contracts from current `develop`, not from an unrefreshed local checkout.
- Local absence is never proof of remote absence.
- `main` may lag development; do not use default-branch code search as evidence of current `develop` task state.

Read `docs/engineering/EXECUTION_SUBSTRATES.md` before assuming local Git/worktree mechanics. Shared local filesystems require a dedicated worktree; scheduled/cloud/API-isolated Developers do not create worktrees solely for convention.

## Workflow
1. Read root `AGENTS.md`, `docs/engineering/CONTROL_PLANE_PROTOCOL.md`, and `docs/engineering/EXECUTION_SUBSTRATES.md`.
2. Inspect live GitHub task issues, active PRs and canonical remote task branches relevant to the candidate task.
3. Resolve current `develop`/remote refs, then read `docs/tasks/BOARD.md` and the selected task spec from that refreshed remote baseline.
4. Confirm the authoritative issue currently authorizes execution (normally `READY`), the task spec is executable, dependencies are satisfied, and PM ordering permits taking it.
5. Read `docs/engineering/PARALLEL_WORK_PROTOCOL.md`. Confirm the canonical remote task branch does not already exist and there is no active PR. Only on a shared local filesystem, also inspect `git worktree list --porcelain` and local branches for ownership ambiguity.
6. Record the selected current `develop` SHA and atomically create the canonical **remote** task branch at that SHA using GitHub create-ref/create-branch fail-if-exists semantics. A plain same-base `git push` is not a sufficient concurrency lock. If the remote ref already exists, this agent lost the race: do not alter it; re-fetch live remote state and choose another eligible task.
7. After the remote claim succeeds, initialize substrate-appropriate execution isolation:
   - shared local filesystem: fetch that canonical branch and create/attach a dedicated task worktree tracking it; the shared/control checkout remains on `develop`;
   - scheduled/cloud/API-isolated: use the platform's isolated/ephemeral workspace or remote branch editing mechanism; do not manufacture a worktree requirement when no shared persistent filesystem exists.
8. Verify the task branch/workspace belongs to the claimed canonical branch. On local worktrees, verify cleanliness/upstream before editing. From this point, all edits/tests/commits/review fixes stay within the task's canonical branch and execution workspace.
9. Re-read the current task contract and only the exact docs/contracts it references. If PM materially changes the claimed task contract, acknowledge/re-read the remote change before continuing; do not silently implement against an old snapshot.
10. If the task builds a substantial subsystem, invoke the open-source research workflow first.
11. Follow `docs/engineering/TDD_PROTOCOL.md` for behavior changes: write the failing test first, make it pass minimally, then refactor with tests green.
12. Respect declared primary write paths, shared integration files and reserved paths. Never modify/reset/clean/reuse another task's local worktree or cloud workspace.
13. Implement only the stated scope and acceptance criteria.
14. Run required targeted checks plus the task's repository verification using the executable environment actually available. Never claim local checks ran if the cloud/API substrate cannot execute them; rely on exact-head CI for checks performed only there.
15. Before opening/updating the PR, re-inspect the live task issue for cancellation/block/re-scope signals and the current remote branch state.
16. Open/update a PR to `develop` with: scope, implementation summary, TDD evidence, tests, risks, assumptions and reuse/license notes if any.

## Existing task / review fixes
If the task already has a canonical remote branch or PR, do **not** claim a new branch. Resolve the latest remote PR head/reviews/comments/checks and live issue state first. Continue on the same canonical branch/PR.

- Shared local filesystem: locate the branch's existing worktree with `git worktree list --porcelain`; if none exists, fetch and attach/create a dedicated worktree. Inspect uncommitted state before editing.
- Scheduled/cloud/API-isolated: reattach/use the platform workspace or operate against the exact current remote branch/PR head. No synthetic local worktree is required.

A stale BOARD/task status does not erase a valid live PR/branch. A material scope/acceptance/contract conflict is different: stop and have PM/Team Lead reconcile it before implementing further.

Never switch a shared control checkout to the PR branch just to perform review fixes.

## Stop conditions
Do not silently broaden scope. If a requirement materially conflicts with an accepted contract/ADR/spec, or a parallel task requires a write outside this task's declared surface, record the conflict for PM/Team Lead rather than inventing behavior.

On shared local filesystems, stop rather than touching another agent's filesystem state if worktree/branch ownership is ambiguous. On cloud/API-isolated execution, remote branch/PR ownership is the relevant isolation signal.

If the remote claim succeeded but execution workspace setup fails, the task remains claimed; fix/report setup rather than deleting the remote branch without PM/Team Lead direction.

If an existing claim appears abandoned, do not delete/take it over. PM/Team Lead owns recovery under `CONTROL_PLANE_PROTOCOL.md`.

## Context discipline
Do not recursively read `docs/**`. Follow links from the current task only after the remote freshness preflight.
