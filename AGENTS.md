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
3. Before claiming it, fetch remote branches and confirm the task's canonical remote branch does not already exist and has no active PR.
4. Create the dedicated task branch from latest `origin/develop` and push it to origin immediately as the task claim/lock. If that push loses a race, do not work on the task; select another eligible READY task.
5. Read only the files explicitly referenced by that task.
6. Follow `docs/engineering/TDD_PROTOCOL.md`: RED -> GREEN -> REFACTOR for behavior changes, with meaningful regression coverage.
7. Respect the task's declared write paths/integration contract. For parallel work follow `docs/engineering/PARALLEL_WORK_PROTOCOL.md`.
8. Implement only the task scope. Record unrelated findings instead of expanding scope.
9. Run the task's required verification.
10. Open a PR to `develop`. Never implement directly on `main` or `develop`.
11. Do not mark a task `DONE`; Team Lead does that after acceptance review.

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
- `synvideo-task-worker`: implement one READY task end-to-end using TDD and parallel-safe claiming.
- `synvideo-wave-planner`: PM planning for a small batch of independent parallel tasks with frozen contracts, path ownership and merge order.
- `synvideo-code-review`: Team Lead review of a PR/diff against requirements and quality gates.
- `synvideo-open-source-research`: research reuse candidates before building a subsystem.
- `synvideo-product-audit`: audit the existing codebase against the product baseline.
