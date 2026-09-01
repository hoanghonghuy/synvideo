# Scene Plan Creator Workspace V1 Contract

Status: FROZEN for planned `TASK-022`.

This contract defines the creator-facing Stage 7 Scene Plan workspace over accepted `SCENE_PLAN_V1` and planned `SCENE_PLAN_JOB_V1` backend APIs.

## Product boundary
The workspace lets a creator:
- browse Scene Plan version history;
- inspect source Script/Proposal/version/staleness;
- generate/regenerate a Scene Plan through durable owner-scoped AI jobs;
- edit visual planning metadata on the active draft;
- split/merge approved narration into production scenes without rewriting approved Script text;
- save with optimistic revision;
- approve the current draft;
- recover generation state after refresh/navigation.

It does not upload/generate media, bind assets, synthesize voice, render or publish.

## Route
Canonical route:
`/projects/:id/scene-plan`

Project detail exposes one clear Stage 7 navigation action when the Project is visible.

## Initial load
Load in parallel where safe:
- Project;
- Scene Plan version list;
- highest approved Script needed for stale/source context;
- owner-scoped text generation options.

Opening behavior:
- if an active draft exists, open it;
- otherwise open the newest Scene Plan version;
- if no Scene Plan exists, show a true empty state rather than a load-error disguise;
- partial fetch failure is a retryable error, not false empty state.

## History/version switching
Show newest-first version history with status `draft|approved|superseded`, source Script version and useful timestamps.

Switching away from a dirty editable draft requires Save / Discard / Cancel. Approved/superseded versions are read-only.

The workspace must not lose unsaved edits because a background generation poll or history refresh completes.

## Source/stale awareness
A Scene Plan stores immutable `source_script_version` and `source_proposal_version`.

Compare the displayed Scene Plan source Script version with the Project's highest approved Script version:
- equal => current source;
- lower/different => stale warning;
- missing approved Script => source warning / generation disabled.

Staleness never mutates the Scene Plan automatically.

## Narration-preserving editing
Approved Script narration is not a freeform text-edit field in this workspace.

Creator may edit per-scene planning fields:
- `visual_instruction`;
- `planned_source_type`;
- `expected_duration_seconds`;
- `caption_intent`;
- `transition_notes`.

Creator may split one scene into multiple scenes only by selecting a boundary in its existing narration and preserving the original ordered text. New scene keys are locally generated in the accepted slug/key shape and remain editable only within validation rules.

Creator may merge adjacent scenes only when they belong to the same `script_section_key`; merged narration is the ordered concatenation of the source scene narration.

The UI must not expose an operation that can silently add, omit, paraphrase or reorder approved narration. Server validation remains authoritative.

Arbitrary cross-section reorder/delete is not part of V1 because it conflicts with the frozen Script-preservation invariant. To rewrite narration, creator must return to Script, create/edit/approve a new Script version, then generate a new Scene Plan.

## Save / optimistic revision
Only an active draft is editable.

PUT the complete editable Scene Plan content with the current positive `revision`.

States:
- clean;
- dirty;
- saving;
- saved;
- stale revision;
- validation error;
- network error.

On stale revision, preserve local edits and offer reload/reconcile; never silently overwrite the newer server draft.

## Approval
Approve only a clean current draft and send the exact current revision.

Dirty draft must be saved or discarded first.

After success:
- displayed version becomes immutable approved;
- history/status refresh;
- Generate becomes Regenerate and any later generation creates a new draft version rather than mutating approval history.

## Durable generation UX
Use owner-scoped `GET /api/v1/ai/text-generation-options`.

If no configured/enabled text model exists:
- disable Generate/Regenerate;
- show localized guidance linking to AI Provider settings;
- do not register/use a deterministic fake in production UI.

Each explicit Generate/Regenerate gets a fresh UUID `request_id`.

Dirty draft blocks generation until Save/Discard.

Show queued/running/succeeded/failed state without replacing or hiding the currently displayed Scene Plan.

Persist only active generation identity needed for recovery (project/job/request identity) in a safe route/session-scoped mechanism; never persist credentials/provider secrets.

On refresh/navigation return:
- resume GET polling of the same job ID;
- transient status fetch failures retry the same job, not POST another request;
- succeeded status opens exact `scene_plan_version` from result and refreshes history;
- if succeeded result version fetch fails transiently, retain succeeded identity and retry loading that exact version rather than offering a misleading Regenerate action;
- terminal Retry creates a fresh request ID.

## Generation/source guards
Generate is enabled only when:
- owner has at least one generation option;
- an approved Script exists;
- no dirty draft blocks replacement;
- no non-terminal Scene Plan generation job is already active for this Project/session.

Backend is authoritative for source relationship errors; frontend maps `SCRIPT_APPROVAL_REQUIRED` / `SCENE_PLAN_SOURCE_INVALID` to localized actionable guidance.

## Accessibility/i18n/responsiveness
- Vietnamese strings via existing locale resources; no hard-coded creator-facing English architecture debt;
- field labels/errors associated accessibly;
- keyboard operable history, split/merge, provider/model selection and approval;
- status is not color-only;
- scene cards remain usable on narrow screens and long-form projects with many scenes.

## Regression/TDD gates
Frontend tests must cover at least:
1. true empty vs load error;
2. open active draft/newest fallback;
3. dirty version-switch guard;
4. approved/superseded read-only behavior;
5. stale source warning;
6. planning-field edits + save revision;
7. stale save preserves local content;
8. split preserves narration exactly and section key;
9. merge only adjacent same-section scenes and preserves ordered narration;
10. no freeform approved-narration rewrite control;
11. dirty blocks Generate/Approve;
12. no-provider disabled guidance;
13. fresh request ID for each explicit generation;
14. pending/failure preserves currently displayed plan;
15. refresh resumes same durable job;
16. transient poll failure retries same job;
17. success opens exact returned Scene Plan version;
18. success-version fetch retry does not Regenerate;
19. terminal Retry uses a fresh request ID;
20. locale/build/typecheck/full frontend verification green.

## Scheduling gate
Contract is frozen now. TASK-022 remains BACKLOG/BLOCKED until TASK-019 releases the shared frontend router/locale/navigation hotspot and TASK-021 backend API is accepted or stable enough for exact contract verification.