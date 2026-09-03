# TASK-033 — Stock Media Search & Acquisition V1

Status: BACKLOG
Milestone: F1 Creative Workflow
Canonical branch when activated: `feature/TASK-033-stock-media`
Depends on: MediaAsset + Scene Media Binding foundations

## Product outcome
Creator can search supported stock image/video providers for a scene, preview truthful source/license data, acquire selected media into SynVideo storage and assign/replace it while preserving provenance/history.

## Scope
Provider-neutral stock search/acquisition contract; first live adapter revalidated before READY; scene-aware editable query; bounded pagination; license/attribution metadata; durable acquisition into stock-origin MediaAsset; assignment/history; creator search/preview/acquisition UI; isolation/idempotency/integration/i18n/accessibility.

## Required behavior
Provider-specific result IDs/URLs/licenses stay out of generic scene semantics; unsupported capabilities are not faked; remote item becomes durable MediaAsset before assignment success; required attribution is preserved; remote URL expiry after acquisition does not break asset; binding history remains intact.

## Activation gate
BACKLOG. Revalidate current stock provider API/license/attribution, freeze contract on `develop`, then move issue #68 to READY last.

## TDD focus
Result mapping, attribution, bounded search, isolation, acquisition recovery, MediaAsset creation, duplicate acquisition semantics and assignment/history.
