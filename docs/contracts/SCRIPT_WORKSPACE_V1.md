# Script Creator Workspace V1 Contract

Status: FROZEN for `WAVE-F1-H`.

This contract makes the accepted Script domain and the frozen `SCRIPT_JOB_V1` capability usable by creators as Stage 5 of the production workflow.

## Product boundary
V1 provides a project-scoped Script workspace for:
- Script version history;
- draft editing and optimistic save;
- approval;
- Generate / Regenerate using owner-scoped live text providers;
- durable generation status and refresh recovery;
- source-staleness awareness without silently rewriting creator-approved work.

V1 is frontend-only. It consumes the accepted/frozen Script and Script-generation APIs and must not change backend contracts.

## Route and navigation
Canonical route: `/projects/:id/script`.

The Project workspace exposes Script as the next stage after AI Proposal. Navigation must remain localized and must not remove existing Creative Brief / Proposal access.

## Data dependencies
The workspace consumes:
- Project GET;
- Creative Proposal version list/read as needed for source readiness/staleness;
- Script list/get/update/approve from `SCRIPT_V1`;
- owner-scoped `GET /api/v1/ai/text-generation-options`;
- `POST /api/v1/projects/{project_id}/script-generations`;
- `GET /api/v1/projects/{project_id}/script-generations/{job_id}`.

No provider credential is ever handled by this workspace.

## Empty and readiness states
When no Script exists:
- show a clear Script empty state, not a generic error;
- generation requires at least one approved Proposal;
- if no approved Proposal exists, disable Generate and guide the creator back to Proposal approval;
- if no enabled text provider/model exists, disable Generate with localized guidance linking to AI Provider settings;
- do not invent a manual/public Script create flow outside the frozen backend contract.

## Version history and selection
- list Script versions deterministically newest-first;
- display status `draft|approved|superseded`, version and revision;
- default to the active draft when present, otherwise latest approved/latest version;
- approved and superseded versions are read-only;
- switching versions while a draft has unsaved changes requires an explicit discard/cancel decision;
- history refresh after generation or approval must not silently discard local unsaved content.

## Draft editing
Editable Script fields follow `SCRIPT_V1` exactly:
- ordered sections with key, optional heading and body;
- optional estimated duration;
- notes.

Required UX states:
- clean;
- dirty;
- saving;
- saved;
- validation error;
- stale revision / conflict;
- network/retryable load error.

Saving sends the current revision. A stale revision response must preserve the creator's local edits and offer an explicit reload/reconcile path; never silently overwrite newer server state.

## Approval
- approval is available only for a clean current draft;
- dirty drafts must be saved or discarded first;
- approval sends the current revision;
- success refreshes history and opens the exact approved version;
- stale/error responses preserve the current workspace state.

## Source staleness
Each Script preserves `source_proposal_version`.

If a newer approved Proposal exists than the selected Script's source version:
- show a localized stale-source warning;
- do not mutate the existing Script;
- existing approved Script remains valid history;
- Regenerate creates a fresh durable generation request using the backend's current approved Proposal selection rules.

## Generate / Regenerate
- each explicit action creates a fresh UUID `request_id`;
- creator selects from current owner-scoped provider/model options;
- dirty Script blocks Generate/Regenerate until save or discard;
- no provider call occurs in the browser;
- queued/running/succeeded/failed states are visible;
- current Script stays mounted while a generation job is queued/running or fails;
- terminal Retry creates a fresh request ID;
- transient polling/network errors retry polling the same job ID and must never POST a replacement request automatically.

On success:
- read the returned `script_version`;
- refresh Script history;
- open exactly that Script version;
- clear the active generation marker only after the succeeded result has been consumed or explicitly dismissed.

## Durable UI recovery
The active Script generation job identity must survive page refresh/navigation within the same creator session using a non-secret namespaced client marker consistent with the accepted Proposal workspace pattern.

Only non-secret identifiers such as project ID/job ID may be persisted. Do not persist provider credentials, Script content drafts, or raw job payload/result in localStorage/sessionStorage.

If a recovered job is terminal:
- succeeded -> load the exact returned Script version;
- failed -> show safe error state and allow explicit retry with a new request ID.

## Failure preservation
A failure to reload Script data after a succeeded job is not a generation failure. The UI must keep the succeeded job result and retry loading that exact Script version; it must not reinterpret this as Regenerate.

General list/get failures must render retryable load errors rather than false empty states.

## i18n and accessibility
- all creator-facing text uses locale keys; no hard-coded product copy;
- Vietnamese is required now and structure must remain ready for English;
- form controls have labels and keyboard focus behavior;
- status/error/success messages use appropriate accessible semantics;
- destructive discard actions require explicit confirmation.

## Deterministic verification
Required frontend coverage includes:
1. no-Script empty state;
2. no-approved-Proposal generation disabled state;
3. no-provider generation disabled/settings guidance;
4. history selection and read-only approved/superseded versions;
5. dirty guard on version switch, approval and regeneration;
6. optimistic save success and stale-revision preservation;
7. approval success/stale/error behavior;
8. newer-approved-Proposal stale-source warning;
9. fresh request ID per explicit Generate/Regenerate/Retry;
10. queued/running/failure preserves the current Script;
11. refresh resumes the same generation job;
12. transient poll error continues the same job without POSTing again;
13. succeeded job opens exact `script_version`;
14. succeeded-result load failure retries the read, not regeneration;
15. list/load error never renders a false empty state;
16. no secret/client credential persistence;
17. lint, typecheck, unit tests and production build green.

## Ownership boundary
TASK-019 owns primarily:
- `apps/web/src/features/script/**`;
- Script workspace tests/API client;
- minimal `apps/web/src/router/index.ts`;
- minimal Project-detail navigation integration;
- localized Script keys in existing locale files;
- minimal shared styles only when unavoidable.

TASK-019 must not modify backend code, migrations, provider-settings behavior, Scene Plan/media features, render/publish code, or refactor unrelated shared UI.