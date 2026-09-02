---
name: synvideo-product-audit
description: Audit the current remote SynVideo codebase against approved product workflows, identify usable/conflicting/missing behavior, and produce non-duplicate PM task candidates.
---

# SynVideo Product Audit

## Use when
Auditing the real codebase after a milestone, when the planned queue is exhausted, or when PM wants to discover bugs, gaps, hardening or optimization work.

Follow `docs/engineering/CONTROL_PLANE_PROTOCOL.md` first. Audit current remote `develop`/accepted history, not a stale local checkout or repository-default `main` if it lags development.

## Remote preflight
Before proposing work:
1. inspect current remote `develop` head and recently merged PRs;
2. inspect open and recently completed task issues;
3. inspect existing TASK specs and canonical active branches/PRs;
4. identify already-planned outcomes, known bugs/hardening items and active write surfaces;
5. only then inspect product/code gaps.

## Workflow
1. Read `docs/product/VISION.md`, `FEATURE_MAP.md`, and `CREATIVE_WORKFLOW.md` from current `origin/develop`.
2. Inspect repository structure, runtime entry points, data model, APIs, UI routes and tests without changing implementation.
3. Build an inventory by product capability:
   - `GOOD`: aligned and usable.
   - `PARTIAL`: useful foundation but incomplete.
   - `CONFLICT`: implementation contradicts approved product behavior.
   - `MISSING`: capability absent.
   - `REMOVE/REPLACE`: implementation creates more risk than value.
4. Identify cross-cutting gaps: auth, i18n, jobs, persistence, provider abstraction, media lifecycle, error handling, tests, observability, performance and deployment.
5. Before converting any finding into a task, run duplicate/overlap detection across live/recent issues, TASK specs, active/recent PRs and canonical branches using product outcome + domain/contract + material write surface.
6. If an existing task already owns the outcome, update/link/reopen it when appropriate rather than creating another issue. If partially overlapping, define explicit dependency/boundary first.
7. Convert only genuinely new, actionable findings into milestone/task candidates. Bugs, optimization and hardening changes still go through PM-approved issues/tasks and normal implementation PRs; the audit itself does not write feature/fix code.

## Evidence quality
Do not create speculative "cleanup" tasks merely because code looks imperfect. A candidate must identify concrete user/product risk, correctness/security/performance evidence, missing acceptance behavior or measurable maintainability/deployment impact.

For specialized implementation approaches, reference existing project docs first. If no suitable internal guidance exists and reputable upstream/open-source documentation or reference repositories would materially help the developer, record those references in the task/research map with current license/source verification rather than vague repository suggestions.

## Output
Produce a concise audit report and proposed queue changes. PM decides priority/status and follows safe activation ordering: freeze task/contracts/board on `develop`, then set the authoritative GitHub issue `READY` last.
