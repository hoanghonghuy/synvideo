# TASK-019 — Script creator workspace

Status: DONE
Milestone: F1 Creative Workflow
Wave: WAVE-F1-H
Branch: `feature/TASK-019-script-workspace`
Base: `develop`
PR: #47
Accepted head: `a9f0c5adf4e43d27dc6811515ce15b12917f41f9`
Logical TL review: `5080349001`
CI: #223 green on accepted head
Merge: `da01e58c7e9a9e6afd9fc117bf464374600d5ed7`
Issue: #37 completed
Depends on: TASK-011 and TASK-018 accepted; frozen `SCRIPT_WORKSPACE_V1` / `SCRIPT_JOB_V1`.

## Goal delivered
The creator-facing Stage 5 Script workspace is now accepted end to end: history, editing, optimistic save, approval, source-staleness awareness, owner-scoped Generate/Regenerate, durable job recovery and safe failure behavior.

## Accepted behavior
- valid fresh UUIDv4 generation/retry request IDs;
- explicit confirmed stale-revision reconcile/reload while preserving local edits until creator choice;
- provider-empty/settings guidance;
- approved/superseded read-only history;
- optimistic save and approval success/conflict preservation;
- terminal failure preserves the mounted Script;
- transient polling retries the same durable job without replacement POST;
- succeeded jobs load exactly the returned Script version and retry that read without regeneration;
- session recovery stores only non-secret project/job identity;
- lint, typecheck, 56 frontend tests, build and exact-head CI green.

## Ownership result
Frontend-only ownership was preserved: Script feature surface plus minimal route/navigation/locale/style integration, with no backend/provider/media/Scene Plan behavior leakage.

This task is complete. Any follow-up behavior belongs in a new explicitly scoped task.