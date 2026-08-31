# SynVideo Task Board

Current milestone: `FOUNDATION PLANNING`

SynVideo starts from a clean repository. Product documentation, agent workflow, rules and skills are now established on `develop`. No application implementation exists yet, so the next PM action is to define the first implementation milestone and task contracts from the approved product baseline.

| ID | Task | Status | Spec / Notes |
|---|---|---|---|
| BOOT-001 | Initialize repository and establish `develop` | DONE | `main` contains the initial repository commit; PM/product scaffold lives on `develop`. |
| AUDIT-001 | Audit pre-existing codebase | CANCELLED | Not applicable: user confirmed there is no pre-existing application code. |
| PLAN-001 | Define Foundation milestone and first implementation task set | IN_PROGRESS | PM action based on `docs/product/**`, ADRs and engineering constraints. |

## Allowed statuses
`BACKLOG`, `READY`, `IN_PROGRESS`, `REVIEW`, `CHANGES_REQUESTED`, `BLOCKED`, `BLOCKED_EXTERNAL`, `DONE`, `CANCELLED`.

PM owns status changes and task priority. AI Developers may only start implementation tasks explicitly marked `READY`.
