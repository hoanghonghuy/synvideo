# Agent Context / Token Policy

Goal: give coding/review agents enough context to be correct without repeatedly loading the entire project history and documentation.

## Default read sequence for implementation
1. root `AGENTS.md`.
2. `docs/tasks/BOARD.md`.
3. selected task spec.
4. only docs/sections listed in that task's **Read first** section.
5. affected code discovered from task scope and dependency tracing.

## Do not do by default
- recursively read all `docs/**`;
- read every ADR for every task;
- scan every source directory before determining the task's affected module;
- load every skill;
- repeatedly re-read unchanged long docs in the same run.

## Skills
Skills are workflows, not a knowledge dump. Open a skill only when its description matches the current activity. The skill should point to the exact supporting docs needed.

## Task authoring rule
PM should make each task self-contained enough that an implementer can begin from a small set of files. Reference headings/paths, not vague instructions such as “read all product docs”.

## Escalation
Read broader context only when:
- a dependency cannot be understood locally;
- code conflicts with the task/product contract;
- an architectural change crosses multiple domains;
- regression analysis requires tracing an existing flow.

## Review context
Team Lead begins with: task → acceptance criteria → PR diff → affected tests → exact supporting docs. Expand to surrounding implementation only where the diff or risk demands it.
