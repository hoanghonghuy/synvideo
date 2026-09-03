# TASK-038 — Channel Hub & Publishing V1

Status: BACKLOG
Milestone: F1 Creative Workflow
Canonical branch when activated: `feature/TASK-038-channel-hub-publishing`
Depends on: TASK-037 accepted rendered-artifact contract

## Product outcome
Creator can connect at least one supported publishing channel, see truthful capabilities, select completed render, configure supported metadata, publish/schedule where permitted and recover/inspect upload-processing failures/history.

## Scope
Provider-neutral connected-channel/capability model; secure OAuth/token lifecycle; first live adapter revalidated before READY (YouTube current candidate); capability discovery; rendered artifact selection; adapter-bounded metadata mapping; durable publish lifecycle; scheduling where supported; Channel Hub connections/publish/history UI; secret-safe publication provenance; isolation/idempotency/provider-local tests.

## Required behavior
Tokens never enter public resources/generic jobs/frontend storage; publishing references immutable render; ambiguous retry avoids duplicate public posts where platform supports recovery; upload success vs platform processing success are distinct where applicable; revocation yields reconnect state; unsupported controls absent/disabled; history links safe platform/content identity to exact render.

## Activation gate
BACKLOG. Revalidate chosen platform API/OAuth scopes/quotas/verification/audit/upload/scheduling immediately before READY, freeze Channel Hub + first-adapter contracts, then move issue #73 READY last.

## TDD focus
OAuth isolation, capability discovery, artifact ownership, metadata mapping, ambiguous retry, upload-vs-processing state, reconnect, scheduling truthfulness and history provenance.
