# Implementation Planning Pointer

`IMPLEMENTATION_PLAN.md` is no longer an authoritative task plan.

The previous root-level contents were a historical TASK-028 implementation plan and had drifted from the current repository state. Current planning and execution state is maintained in the canonical task/governance sources below:

- `docs/tasks/BOARD.md` — current task-state mirror / execution queue;
- `docs/tasks/TASK-*.md` — authoritative per-task scope, status, acceptance criteria, dependencies, and activation gates;
- `docs/contracts/*.md` — frozen cross-task/product/technical contracts where applicable;
- GitHub issues — canonical live PM/TL status, blocker, claim, and completion evidence;
- protected `develop` — integration source of truth for accepted code/docs.

Developer implementation must start only from a PM-authorized READY/claimed task and its canonical task branch. A historical implementation plan must not override the current task file, contract, issue state, branch state, review state, or required CI.

For historical TASK-028 context, use repository/Git history rather than treating this root file as an active implementation specification.
