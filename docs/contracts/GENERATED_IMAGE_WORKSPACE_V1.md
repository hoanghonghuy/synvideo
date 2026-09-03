# Generated Image Workspace V1 Contract

Status: FROZEN for planned `TASK-030` after TASK-029 acceptance.

## Product outcome
A creator working on an approved Scene Plan can generate an image for one scene, observe the durable generation lifecycle, recover after refresh/transient failures, inspect successful alternatives, and explicitly assign/replace the scene primary visual without losing prior selection history.

TASK-030 turns the backend acquisition capability from TASK-029 into a complete creator-facing feature. It does not create a second generation pipeline.

## Preconditions
- owner-authenticated project;
- approved current Scene Plan version with the target scene key;
- TASK-029 durable generated-image API accepted on `develop`;
- owner image provider/model options resolved through the accepted provider settings/runtime.

## Creator flow
1. Open the Media/Scene workspace for an approved Scene Plan.
2. Select a scene.
3. Review/edit the generation prompt derived from the scene visual instruction.
4. Select an enabled internal image provider/model option.
5. Choose whether the successful result should be assigned immediately or only saved as an alternative.
6. Submit with a stable client request ID.
7. UI enters queued/running state and polls the exact returned job.
8. Refresh/re-entry resumes that same known active job where safe; it must not silently submit a second paid generation.
9. On success, load the exact returned MediaAsset and show preview/provenance-safe metadata.
10. Creator may keep it as an alternative or explicitly assign/replace the current primary visual through existing scene-binding behavior.
11. Prior binding history remains available.

## State truthfulness
The UI distinguishes provider/model options unavailable; invalid/stale Scene Plan context; queued/running generation; transient polling/load failure with same-job retry; terminal provider/generation failure; successful asset acquisition but assignment failure; successful unassigned alternative; and successful assigned result.

A polling/network failure is never displayed as a generation failure unless the backend job says so.

## Prompt semantics
- Default prompt may be seeded from approved scene visual instruction.
- Creator may edit the request prompt before submit.
- Submitted prompt becomes immutable request intent for that job.
- Switching scenes must not leak a dirty prompt into another scene.
- No raw prompt is stored in browser persistence beyond what is necessary for the current editing session unless the backend contract explicitly exposes it safely.

## Idempotency/recovery
- one user submit creates one request ID;
- retry after ambiguous POST result reuses that request ID;
- refresh/resume uses returned job identity, never a new request ID;
- duplicate successful load must not duplicate assignment;
- exact succeeded result is loaded even if later generations for the same scene exist;
- starting a deliberate regeneration uses a new request ID and preserves older generated assets/history.

## Assignment semantics
Generation success and scene assignment are separate visible outcomes. `assign_on_success=false` changes no binding. `assign_on_success=true` uses TASK-029 idempotent backend assignment semantics. Manual assignment/replacement reuses accepted Scene Media Binding API/history.

## Safe metadata
Never expose provider credential, base URL, ciphertext, external model identifier, signed output URL or raw upstream body.

## Non-scope
Image editing/inpainting, generated video, stock search, TTS/audio/captions/music, backend redesign, batch/autopilot generation and cost accounting beyond already-safe metadata.

## Required TDD gates
1. default prompt + scene edit isolation;
2. provider/model unavailable states;
3. request-ID reuse after ambiguous submit failure;
4. same-job polling/refresh recovery;
5. transient polling retry;
6. truthful terminal failure;
7. exact succeeded asset preview;
8. unassigned vs assigned success;
9. asset success + assignment failure recovery;
10. deliberate regeneration preserves alternatives/history;
11. stale response protection;
12. i18n/accessibility/full CI.
