# SynVideo Agent Router

Keep this file short. Do not load the whole repository or `docs/` tree unless the current task requires it.

## Source of truth
- Product intent: `docs/product/`
- Task queue: `docs/tasks/BOARD.md`
- Current task contract: the selected `docs/tasks/TASK-xxx.md` or GitHub issue/PR
- Engineering constraints: `docs/engineering/`
- Open-source research: `docs/research/OPEN_SOURCE_REFERENCES.md`
- Decisions that must not silently drift: `docs/decisions/`

## Continuation command
When the user says only `tiếp tục`, `continue`, `go on`, `làm tiếp`, `tiếp đi`, or equivalent wording without a more specific instruction, use the `synvideo-continue` skill.

Continuation means: inspect repository/GitHub state and perform the next valid workflow action. Prioritize active PR review comments and failing CI before unfinished task work, and unfinished task work before taking a new `READY` task. It never authorizes inventing work, self-merging, marking a task `DONE`, or implementing directly on `main`/`develop`.

See `docs/engineering/CONTINUE_PROTOCOL.md` only when the continuation state needs clarification.

## Coding workflow
1. Start from the latest `origin/develop` when taking a new task.
2. Read `docs/tasks/BOARD.md` and select only a `READY` task whose dependencies are satisfied.
3. Before claiming it, fetch remote branches and `git worktree` state; confirm the task's canonical remote branch does not already exist, has no active PR, and is not already represented by another local task worktree/branch.
4. Atomically create the canonical **remote** task branch at the selected `origin/develop` SHA using GitHub create-ref/create-branch fail-if-exists semantics. A plain same-SHA `git push` is not a sufficient concurrency lock. If create-ref loses a race, do not work on or alter that branch; select another eligible task.
5. After the remote claim succeeds, create/attach a **dedicated Git worktree** for that canonical branch. The shared/control checkout stays on `develop` while concurrent agents are active. Never use `git switch` in that shared checkout to move between concurrent tasks.
6. After a successful claim/worktree setup, do all edits, tests, commits, rebases and review fixes inside that task worktree. Read only the files explicitly referenced by that task.
7. Follow `docs/engineering/TDD_PROTOCOL.md`: RED -> GREEN -> REFACTOR for behavior changes, with meaningful regression coverage.
8. Respect the task's declared write paths/integration contract. For parallel work follow `docs/engineering/PARALLEL_WORK_PROTOCOL.md`.
9. Implement only the task scope. Record unrelated findings instead of expanding scope.
10. Run the task's required verification.
11. Open a PR to `develop`. Never implement directly on `main` or `develop`.
12. Do not mark a task `DONE`; Team Lead does that after acceptance review.

For an existing task/PR, locate or create the dedicated worktree for its existing canonical branch and continue there. Never switch a shared control checkout to another active task branch, and never reset/clean/remove a worktree that may belong to another agent.

Do not continuously merge `develop` while implementing an unrelated task. Sync when starting work, when the task requires new upstream changes, and before final PR integration when necessary.

## Product rules
- Product specs and accepted ADRs win when existing code conflicts with them.
- Do not invent missing product behavior. Surface the gap in the PR/task.
- SynVideo is human-in-the-loop: expensive/irreversible generation should have review checkpoints where specified.
- Keep product/domain semantics provider-neutral. Provider names belong in adapters/configuration, not core concepts.
- i18n architecture is required from the beginning; do not scatter hard-coded user-facing strings.

## Research-before-build
Before implementing a substantial AI, media, editor, rendering, TTS, stock-media or publishing subsystem, consult `docs/research/OPEN_SOURCE_REFERENCES.md`.

Never copy/adapt source unless its current license has been verified and the task records the reuse decision and obligations.

## Skills
Use a skill only when its description matches the work; open the `SKILL.md` then follow only the references it names.
- `synvideo-continue`: determine and execute the next valid workflow action from repo/PR/issue state after a generic continuation command.
- `synvideo-task-worker`: implement one READY task end-to-end using TDD, atomic remote claiming and a dedicated worktree.
- `synvideo-wave-planner`: PM planning for a small batch of independent parallel tasks with frozen contracts, path/worktree ownership and merge order.
- `synvideo-code-review`: Team Lead review of a PR/diff against requirements and quality gates.
- `synvideo-open-source-research`: research reuse candidates before building a subsystem.
- `synvideo-product-audit`: audit the existing codebase against the product baseline.
