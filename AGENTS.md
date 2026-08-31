# SynVideo Agent Router

Keep this file short. Do not load the whole repository or `docs/` tree unless the current task requires it.

## Source of truth
- Product intent: `docs/product/`
- Task queue: `docs/tasks/BOARD.md`
- Current task contract: the selected `docs/tasks/TASK-xxx.md` or GitHub issue/PR
- Engineering constraints: `docs/engineering/`
- Open-source research: `docs/research/OPEN_SOURCE_REFERENCES.md`
- Decisions that must not silently drift: `docs/decisions/`

## Coding workflow
1. Start from the latest `origin/develop` when taking a new task.
2. Read `docs/tasks/BOARD.md` and select only a `READY` task.
3. Create a dedicated branch such as `feature/TASK-012-scene-editor`.
4. Read only the files explicitly referenced by that task.
5. Implement only the task scope. Record unrelated findings instead of expanding scope.
6. Run the task's required verification.
7. Open a PR to `develop`. Never implement directly on `main` or `develop`.
8. Do not mark a task `DONE`; Team Lead does that after acceptance review.

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
- `synvideo-task-worker`: implement one READY task end-to-end.
- `synvideo-code-review`: Team Lead review of a PR/diff against requirements and quality gates.
- `synvideo-open-source-research`: research reuse candidates before building a subsystem.
- `synvideo-product-audit`: audit the existing codebase against the product baseline.
