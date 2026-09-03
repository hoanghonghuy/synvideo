# TASK-033 — Stock Media Search & Acquisition V1

Status: BACKLOG
Priority: P1
Milestone: F1 Creative Workflow
Issue: #68
Canonical branch when activated: `feature/TASK-033-stock-media`
Depends on: MediaAsset + Scene Media Binding foundations
Contract: `docs/contracts/STOCK_MEDIA_ACQUISITION_V1.md`

## Product outcome
Creator can search a supported stock image/video source for a scene, preview truthful source/license/attribution data, explicitly acquire a selected item into SynVideo-managed storage, and assign/replace it while preserving durable provenance and binding history.

## Scope
- Provider-neutral stock search and acquisition contract.
- Scene-aware editable search query with bounded pagination and explicit result type/orientation capability.
- Truthful source, creator, license/usage, attribution and source-page metadata.
- Explicit acquisition step that copies the chosen remote item into SynVideo-managed MediaAsset storage before assignment can succeed.
- Idempotent/recoverable acquisition and duplicate-selection semantics.
- Scene assignment/replacement through existing MediaAsset binding/history behavior.
- Creator search/preview/acquisition UI with loading/empty/error/unsupported states, i18n and accessibility.
- First adapter must be revalidated against current API/license/attribution/redistribution terms immediately before READY.

## Required behavior
1. Provider-specific result IDs, URLs, license strings and API pagination details remain behind the stock-provider adapter boundary.
2. Search results expose enough normalized metadata to make source/creator/license/attribution truth visible before acquisition.
3. Unsupported provider capabilities are not fabricated; result type, orientation, pagination and availability reflect provider truth.
4. Remote media is not considered assigned until acquisition has produced a durable same-project MediaAsset.
5. Successful acquisition must not depend on remote URL lifetime afterwards.
6. Attribution/source metadata required or recommended by the selected provider is preserved durably with provenance and remains available to downstream UI/export/publishing flows.
7. Acquisition retry for the same logical selection is idempotent or deterministically deduplicated; it must not create uncontrolled duplicate MediaAssets.
8. Cross-project/principal result acquisition and MediaAsset assignment are rejected.
9. Provider errors, rate limits, removed items and expired download URLs surface deterministic recoverable/non-recoverable states; the system must not silently substitute another stock item.
10. Binding history remains intact when stock media is assigned/replaced.
11. Provider terms that prohibit bulk mirroring, dataset creation, competing-stock-service behavior or other restricted reuse must be reflected in adapter/product behavior rather than ignored.

## Acceptance criteria
- `STOCK_MEDIA_ACQUISITION_V1` is implemented behind a provider-neutral adapter boundary.
- A first live stock provider is revalidated immediately before READY and its API/license/attribution/redistribution constraints are documented with the activation evidence.
- Search supports bounded pagination and truthful capability/result metadata.
- Creator can preview source/creator/license/attribution information before acquisition.
- Acquisition creates/reuses a durable stock-origin MediaAsset before scene assignment succeeds.
- Remote URL expiry after successful acquisition does not break the durable asset.
- Duplicate logical acquisition is idempotent/deduplicated and regression-tested.
- Removed/unavailable source items and provider rate/error conditions surface truthfully without silent substitution.
- Project isolation, assignment/history, refresh recovery, i18n and accessibility are covered.
- No test performs live provider calls; adapter tests use deterministic fixtures/fakes.
- RED → GREEN → REFACTOR evidence follows `docs/engineering/TDD_PROTOCOL.md`.
- Required `Frontend`, `Backend`, `Local Infrastructure` CI remains green.

## Non-scope
- Building a general-purpose stock marketplace or mirroring a provider catalog.
- Bulk corpus ingestion, dataset creation, model training/evaluation or scraping outside approved provider API behavior.
- Final publishing attribution placement rules; downstream export/publishing must consume the preserved provenance metadata.
- Multiple providers in V1 unless a second adapter is separately justified.

## Provider revalidation note
As of 2026-09-04, Pexels publicly exposes image/video search APIs and states its media is free for personal/commercial use under the Pexels license, while its API guidance asks applications to credit photographers when possible and higher API limits may depend on acceptable attribution. Current terms also restrict using/compiling content to replicate a competing service and prohibit unauthorized large-scale extraction/ML dataset use. Treat Pexels as a candidate, not a frozen implementation dependency, until READY-time API approval/terms review confirms SynVideo's explicit per-user search/acquire workflow is permitted.

## Dependencies / relations
- MediaAsset and Scene Media Binding foundations own durable asset identity, project isolation and assignment history.
- TASK-030/TASK-032 own AI-generated media workflows; TASK-033 owns third-party stock discovery/acquisition and must not conflate provenance/licensing semantics with generated media.
- TASK-036 consumes selected scene visual MediaAssets, regardless of stock/generated origin.

## Activation gate
Remain BACKLOG after planning freeze. Immediately before READY, PM/TL must re-check exact `develop`, duplicate branches/PRs/issues, revalidate the chosen first provider's current API approval/limits/license/attribution/redistribution rules, confirm MediaAsset/binding semantics have not drifted, and reconcile bounded implementation WIP. Move issue #68 to READY only after those checks.

## TDD focus
Result normalization, bounded pagination, attribution/provenance mapping, project isolation, acquisition recovery, remote-expiry independence, MediaAsset creation/deduplication, removed-source behavior and assignment/history.
