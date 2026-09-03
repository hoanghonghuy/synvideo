# TASK-041 — Production deployment & release baseline

Status: BACKLOG
Priority: P1
Base branch: `develop`
Canonical branch: `feature/TASK-041-production-deployment-release`
Issue: #84

## Goal
Provide a repeatable, documented production deployment and release baseline for the web/API/data runtime, with safe migration, promotion, rollback, smoke verification, environment separation and secret boundaries.

## Problem / evidence
Current `develop` has local bootstrap and CI but no authoritative production deployment/release contract. `infra/` is local-development infrastructure, `.github/workflows` contains CI only, and there is no production build/runtime manifest, migration-release ordering, promotion contract, health-gated traffic switching or rollback/recovery baseline.

Passing CI alone is therefore insufficient evidence that SynVideo can be deployed and recovered safely.

## Scope
- Define deterministic production build/runtime entrypoints for frontend and Go API.
- Define required runtime configuration and validation without committed secrets.
- Define environment separation for local/test/staging/production.
- Define production database/object-storage connectivity assumptions behind existing provider-neutral interfaces.
- Define database migration ordering, compatibility expectations and failure behavior relative to app rollout.
- Define release promotion from protected `develop` toward `main`/production and the evidence required.
- Define liveness/readiness gates for deployment traffic switching in coordination with TASK-039.
- Define application rollback and migration recovery constraints, especially for destructive/incompatible schema changes.
- Add deployment manifests/container/build configuration appropriate to the selected near-term hosting topology during implementation.
- Add smoke verification for frontend serving, API availability and configured critical dependencies.
- Document secret injection, ownership and rotation boundaries.

## Non-scope
- Choosing an enterprise platform without need.
- Full multi-region/HA architecture.
- Kubernetes unless independently justified.
- Implementing TASK-039 or TASK-040 inside this task.
- Unrelated product feature work.

## Acceptance criteria
1. Frontend and API have deterministic production build/runtime entrypoints reproducible from a clean checkout/CI environment.
2. Production runtime configuration is explicit, validated and contains no committed credentials/secrets.
3. Database migration order and deployment failure semantics are documented and testable; incompatible schema/application combinations are not silently served.
4. Release promotion to production is documented with exact evidence/gates and does not bypass branch protection or required checks.
5. Deployment uses truthful liveness/readiness gates once TASK-039 is available; until then production go-live remains explicitly gated.
6. Rollback/recovery behavior covers application failure and migration incompatibility, including constraints on destructive migrations.
7. Production database/object-storage choices remain behind existing provider-neutral application contracts unless a separate ADR intentionally changes them.
8. A deployment smoke path verifies frontend serving, API availability and configured critical dependency connectivity without exposing secrets.
9. Environment separation and secret injection/rotation ownership are documented.
10. Existing required `Frontend`, `Backend` and `Local Infrastructure` checks remain green; new release checks are additive.

## Quality / implementation notes
- Prefer the simplest architecture compatible with the actual near-term hosting target.
- Freeze provider-specific topology at activation time, not prematurely in this BACKLOG spec.
- Keep database migrations as an explicit release step, not an accidental side effect of every API process startup.
- Do not weaken branch protection, required checks, secret controls or deployment protections.
- Add tests/smoke checks appropriate to deployment configuration and migration ordering.

## Dependencies / relations
- TASK-039 supplies truthful readiness/observability gates.
- TASK-040 supplies production authentication before public creator APIs are exposed.
- These relations gate production release, not unrelated feature coding.

## Activation gate
Remain BACKLOG until PM/TL confirms the intended near-term production hosting topology, freezes the environment/deployment contract or ADR, reruns dedupe/branch checks and reconciles implementation WIP capacity.

## Delivery
Developer implements on `feature/TASK-041-production-deployment-release` and opens a PR into `develop`. Developer must not self-merge or self-mark DONE.
