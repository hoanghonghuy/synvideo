# PM / AI Developer / Team Lead Workflow

`docs/engineering/CONTROL_PLANE_PROTOCOL.md` defines remote authority, freshness, transition ordering, duplicate prevention and drift handling for every role.

`docs/engineering/EXECUTION_SUBSTRATES.md` defines when local Git worktree isolation is required versus when a scheduled/cloud/API-isolated Developer can operate using only its canonical remote task branch.

## Roles
### PM
Owns product intent, scope, priorities, user flows, acceptance criteria, roadmap, task readiness and control-plane documentation.

PM changes to `AGENTS.md`, `docs/**`, task metadata and other control-plane files must respect repository branch protection. When protection is enabled, use a short-lived control-plane/docs branch and PR to `develop`; do not bypass protected-branch rules merely because the change is non-implementation. Before planning or changing task state, PM inspects fresh remote issues/PRs/branches and current `develop`; PM must not treat a local checkout as authoritative.

PM owns live task authorization and lifecycle through the authoritative GitHub task issue, with BOARD/task status fields maintained as mirrors/indexes. PM also owns explicit abandoned-claim recovery and material re-scope decisions.

### AI Developer
Owns implementation for one PM-authorized task at a time. Never implements directly on `main` or `develop`.

A developer resolves live issue/PR/remote-branch state before local state. `READY` authorizes execution but does not prove availability: a canonical remote branch or active PR means the task is claimed.

Execution isolation depends on substrate. Shared local filesystems require a dedicated worktree per task; scheduled/cloud/API-isolated Developers do not create worktrees solely for convention and instead use the canonical remote branch plus the platform's isolated execution environment.

### Team Lead
Reviews implementation against the exact current remote PR head, current task/product contract, architecture constraints, regressions, security, tests and real user behavior. Team Lead does not silently rewrite product decisions during review and does not reuse stale-head approval/CI as evidence for a newer head.

## Branch model
- `main`: stable/release-ready and may intentionally lag development.
- `develop`: current integration/control-plane baseline and protected integration branch.
- implementation: `feature/TASK-xxx-*`, `fix/TASK-xxx-*` → PR to `develop`.
- PM/TL control-plane housekeeping when protection is enabled: short-lived `docs/*`, `chore/*` or equivalent → PR to `develop`.

Generic/default-branch code search is not proof of current development state; inspect explicit `develop`, canonical task branch or exact PR head.

## Protected-branch enforcement

`main` and `develop` should reject direct history mutation, force pushes and branch deletion. `develop` requires PR integration and the current required CI checks. Because PM/TL/Developer automation may share one GitHub identity, do not rely on admin bypass to distinguish roles: repository protection should apply to administrators too, and role separation is enforced through branch/PR workflow rather than shared-account identity.

The repository helper `scripts/admin/protect-branches.sh` applies the intended baseline through authenticated GitHub CLI. If protection policy changes, update this workflow and the script together.

## Developer loop
When idle:
1. inspect live GitHub task issues, active PRs and canonical task branches;
2. refresh/inspect current `develop` and relevant remote refs through the execution substrate available to the agent;
3. read current BOARD/task spec from that refreshed remote baseline;
4. select only PM-authorized executable work whose dependencies/order permit it;
5. confirm it has no canonical remote claim branch/active PR/ownership ambiguity;
6. atomically create the absent canonical remote branch at the selected current `develop` SHA;
7. initialize execution isolation appropriate to the substrate: dedicated worktree for a shared local filesystem, or the platform's isolated/ephemeral/API workspace for a scheduled/cloud agent;
8. implement/test using TDD and the current task contract;
9. re-check live task state before delivery, then open/update the PR to `develop`;
10. address Team Lead findings on the same canonical branch/PR and, when local, the same task worktree;
11. after merge, return to refreshed remote state instead of assuming local mirrors are current.

Local/shared-filesystem Developers should not continuously pull/merge `develop` while implementing an unrelated task. Sync only when upstream changes are required or at an appropriate integration point. Scheduled/cloud/API-isolated Developers likewise re-resolve current remote contracts before meaningful continuation rather than relying on a stale execution snapshot.

## Status and ownership
Typical PM lifecycle:
`BACKLOG → READY → IN_PROGRESS → REVIEW → (CHANGES_REQUESTED ↔ REVIEW) → DONE`.
Additional states: `BLOCKED`, `BLOCKED_EXTERNAL`, `CANCELLED`.

The GitHub issue is the live authorization/lifecycle authority. BOARD/task status fields are maintained mirrors. Claim ownership is separate and is determined by the canonical remote task branch/active PR.

AI Developers may report progress but do not self-certify `DONE`, take over another claim, or silently continue through a material PM re-scope.

## Transition discipline
- To activate work, merge/finalize versioned task/contracts/order on protected `develop` first and change the issue to `READY` last.
- To block/cancel unclaimed work, update the issue first, then reconcile mirrors through the protected control-plane PR path.
- To complete work, accept exact head → merge → close/update issue → reconcile BOARD/task mirrors.
- Metadata drift is repaired using the authority rules; material contract drift stops implementation until PM/Team Lead resolves it.
