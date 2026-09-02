# Developer Execution Substrates

SynVideo supports more than one execution substrate for AI Developers. The shared safety invariant is **remote ownership isolation**, not a specific local Git layout.

## Universal requirements

Every implementation task, regardless of where the agent runs, must:
- resolve fresh GitHub issue/branch/PR state before acting;
- use the task's canonical remote branch as the cross-agent ownership claim;
- create that branch atomically only-if-absent when claiming new work;
- keep one implementation task per canonical branch;
- implement only PM-authorized scope;
- deliver through a PR to protected `develop`;
- never direct-push `develop`/`main`, bypass protection, self-merge or self-mark the task `DONE`.

## Local/shared-filesystem Developer

A Developer running on a machine where multiple agents or tasks can share the same repository filesystem must use a dedicated Git worktree for each active implementation task.

The worktree is **machine-local filesystem isolation**. It prevents one agent from switching/resetting/cleaning another task's checkout and protects uncommitted work. The shared control checkout stays on `develop`.

For this substrate:
- inspect `git worktree list --porcelain` during claim/continue preflight;
- one active task = one dedicated task worktree + canonical remote branch;
- review fixes reuse the same task worktree/branch/PR;
- never modify another task's worktree.

## Scheduled/cloud/API-isolated Developer

A scheduled or cloud Developer that does not share a persistent local repository filesystem with other Developers does **not** need to create or inspect Git worktrees merely to satisfy workflow convention.

For this substrate:
- the canonical remote task branch is the required isolation/ownership boundary;
- atomic remote branch creation remains mandatory for new claims;
- active PR/branch state remains authoritative for continuation;
- use the platform's isolated workspace, ephemeral checkout, connector/API file operations, or equivalent execution environment;
- if the platform does expose a shared persistent checkout, treat it as the local/shared-filesystem substrate and use worktree isolation.

Do not manufacture a fake worktree requirement in an environment that has no shared local filesystem. Conversely, do not drop worktree isolation on a shared machine just because remote branch locking exists: remote locking protects cross-agent ownership, while worktrees protect local filesystem state.

## Tests and verification

Execution substrate does not weaken the quality gate. Behavior changes and bug fixes still follow the repository TDD protocol where applicable.

A cloud/API agent that cannot execute repository tests locally must not claim local test success. It should use whatever executable environment is actually available and rely on exact-head CI for checks that only CI can run. A lack of local shell access is not permission to weaken tests or required checks.

## Ownership summary

| Concern | Local/shared filesystem | Scheduled/cloud/API-isolated |
|---|---|---|
| Cross-agent claim | canonical remote branch | canonical remote branch |
| Atomic claim | required | required |
| Dedicated Git worktree | required | not required unless filesystem is shared/persistent |
| PR to `develop` | required | required |
| TDD/verification | required | required |
| Direct push / self-merge | forbidden | forbidden |
