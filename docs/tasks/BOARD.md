# SynVideo Task Board

Current milestone: `BOOTSTRAP / CODEBASE AUDIT`

The implementation repository has not yet been audited, so feature tasks must not be invented from this board before the real code/history is available.

| ID | Task | Status | Spec / Notes |
|---|---|---|---|
| BOOT-001 | Push existing SynVideo code/history and establish `develop` | BLOCKED_EXTERNAL | Must preserve the real local/EC2 Git history; do not create a separate docs-only root history. |
| AUDIT-001 | Audit current codebase against approved product baseline | BLOCKED | Run after BOOT-001 using `synvideo-product-audit`. |
| PLAN-001 | Build milestone/task backlog from audit gaps | BLOCKED | PM action after AUDIT-001. |

## Allowed statuses
`BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.

PM owns status changes and task priority.
