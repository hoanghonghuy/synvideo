---
name: synvideo-product-audit
description: Audit the current SynVideo codebase against approved product workflows to inventory what exists, what is usable, what conflicts, what is missing, and what should become tasks.
---

# SynVideo Product Audit

## Use when
The real SynVideo codebase/history is available or after a major milestone.

## Workflow
1. Read `docs/product/VISION.md`, `FEATURE_MAP.md`, and `CREATIVE_WORKFLOW.md`.
2. Inspect repository structure, runtime entry points, data model, APIs, UI routes and tests without changing implementation.
3. Build an inventory by product capability:
   - `GOOD`: aligned and usable.
   - `PARTIAL`: useful foundation but incomplete.
   - `CONFLICT`: implementation contradicts approved product behavior.
   - `MISSING`: capability absent.
   - `REMOVE/REPLACE`: implementation creates more risk than value.
4. Identify cross-cutting gaps: auth, i18n, jobs, persistence, provider abstraction, media lifecycle, error handling, tests, observability and deployment.
5. Convert gaps into milestone candidates and task-sized work; do not write feature code during the audit.

## Output
Produce a concise audit report and propose updates to `docs/tasks/BOARD.md`. PM decides priority/status.
