# TASK-032 — Per-scene AI Video Generation V1

Status: BACKLOG
Milestone: F1 Creative Workflow
Canonical branch when activated: `feature/TASK-032-scene-video-generation`
Depends on: TASK-025, TASK-028, MediaAsset + Scene Media Binding foundations; fresh live-video provider research gate

## Product outcome
Creator can generate video for one approved scene, survive long-running provider execution/restarts without duplicate paid submissions, obtain durable generated-video MediaAsset, preview alternatives and assign/replace scene visual.

## Scope
Revalidate first live video provider before READY; implement adapter if absent; snapshot approved scene intent; persist opaque external operation ID after first submit; resume/poll same operation after crash; ingest result to MediaAsset; safe provenance/status/errors; alternatives/assignment UI and integration coverage.

## Critical invariant
Retry/reclaim after successful upstream submission must not create a second provider video generation merely because in-memory state was lost.

## Non-scope
Full editor, batch generation, arbitrary vendor passthrough, render/publish, cost ledger.

## Activation gate
BACKLOG. Do not freeze external adapter details yet. Revalidate provider availability/deprecation immediately before READY, freeze versioned contract on `develop`, then move issue #67 to READY last.

## TDD focus
Exactly-once logical submit, external-operation persistence, same-operation poll resume, MediaAsset ingestion, isolation, assignment idempotency and refresh recovery.
