# TASK-002 — Project domain and persistence foundation

Status: BLOCKED
Milestone: F0 Technical Foundation
Depends on: TASK-001

## Goal
Introduce the first durable product domain for SynVideo projects without yet implementing the full creative pipeline.

## Scope preview
- Project identity/ownership boundary.
- Core project metadata: title, description/brief seed, target format/aspect ratio/duration intent, locale, lifecycle status, timestamps.
- PostgreSQL persistence and migrations.
- Backend repository/service/API boundaries.
- Minimal frontend project list/create/open flow sufficient to prove durable persistence.
- Validation, empty/loading/error states, refresh persistence and appropriate tests.

## Out of scope preview
- AI proposal/script generation.
- Scene editing.
- Asset upload/generation.
- Authentication product unless separately approved.

Full task contract will be finalized after TASK-001 review so it matches the accepted application structure.
