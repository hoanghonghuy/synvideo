# Agent Context / Token Policy

Goal: give coding/review agents enough **fresh authoritative** context to be correct without repeatedly loading the entire project history and documentation.

## Mandatory freshness preflight
Before the normal minimal read sequence, follow `docs/engineering/CONTROL_PLANE_PROTOCOL.md`:
- resolve live issue/PR/canonical remote branch state relevant to the task;
- refresh `origin/develop` and remote refs before reading versioned development contracts;
- never infer remote absence from a stale local checkout/ref;
- do not rely on repository-default-branch code search for current development state because `main` may lag `develop`.

## Default read sequence for new implementation
1. root `AGENTS.md`.
2. `docs/engineering/CONTROL_PLANE_PROTOCOL.md` when making workflow/state decisions.
3. live authoritative GitHub task issue + canonical remote branch/active PR state.
4. refreshed `docs/tasks/BOARD.md` from current `origin/develop` for PM ordering.
5. selected task spec from current `origin/develop`.
6. only docs/sections listed in that task's **Read first** section.
7. affected code discovered from task scope and dependency tracing.

## Existing PR / review-fix sequence
1. exact current remote PR head/base/diff;
2. latest reviews/threads/comments/checks for that head;
3. live task issue;
4. refreshed current task/product/engineering contracts from `origin/develop`;
5. only then local worktree state and affected code.

## Do not do by default
- recursively read all `docs/**`;
- read every ADR for every task;
- scan every source directory before determining the task's affected module;
- load every skill;
- repeatedly re-read unchanged long docs in the same run;
- treat local/default-branch search results as proof that current remote `develop` work is absent.

## Skills
Skills are workflows, not a knowledge dump. Open a skill only when its description matches the current activity. The skill should point to exact supporting docs needed.

## Task authoring rule
PM should make each task self-contained enough that an implementer can begin from a small set of files. Reference headings/paths, not vague instructions such as “read all product docs”. PM must also run duplicate/overlap checks before creating a new task.

## Escalation
Read broader context only when a dependency cannot be understood locally, code conflicts with the task/product contract, an architectural change crosses domains, regression analysis requires tracing an existing flow, or remote sources reveal a material contract conflict.

## Review context
Team Lead begins with exact current PR head → live task issue → current task acceptance criteria → remote diff → affected tests → exact supporting docs. Expand only where diff/risk demands it. Approval and CI from an older head do not count for a newer head.
