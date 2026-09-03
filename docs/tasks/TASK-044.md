# TASK-044 — Production backup, restore & data recovery baseline

Status: BACKLOG
Priority: P1
Base branch: `develop`
Canonical branch: `feature/TASK-044-production-data-recovery`
Issue: #93

## Goal
Provide a production data-protection baseline for PostgreSQL business/domain state and managed object-storage media, with explicit recovery objectives, safe restore procedures and non-destructive restore verification.

## Problem / evidence
Protected `develop` has deployment/release planning through TASK-041, including application rollback and migration recovery constraints, but no independent backup/restore/PITR/RPO/RTO contract for production data. Repository and issue dedupe found no task that owns this outcome.

Application rollback is not a substitute for recovery from operator error, corruption, accidental deletion or storage loss. Production readiness therefore requires explicit data-loss protection and a verified recovery path.

## Scope
- Inventory production-critical data classes and ownership boundaries across PostgreSQL and managed object storage.
- Define initial-production RPO and RTO targets, including assumptions and trade-offs.
- Define provider backup/PITR/snapshot expectations for the selected production database and object-storage topology while keeping application/domain code provider-neutral.
- Define backup cadence, retention, encryption, access-control and expiry/deletion ownership.
- Define database-only, object-only and coordinated restore procedures.
- Define post-restore reconciliation for metadata/object consistency, including missing/orphaned media and invalid durable-job/media references.
- Define schema/application compatibility checks during recovery so restored data is not silently served by an incompatible runtime.
- Provide a non-destructive restore verification/drill path for staging or another isolated environment.
- Document evidence required to consider a restore drill successful without exposing secrets or private media.

## Non-scope
- Multi-region active-active architecture.
- Enterprise DR orchestration without a verified need.
- Reimplementing deployment/release mechanics owned by TASK-041.
- Provider-specific business/domain behavior.
- Automatic destructive reconciliation of missing/orphaned data.

## Acceptance criteria
1. Production-critical PostgreSQL and object-storage data classes, ownership and recovery responsibilities are documented.
2. Initial-production RPO/RTO targets are explicit, justified and usable as release-review gates.
3. Backup/PITR/snapshot cadence and retention are defined for every production-critical store with encryption and least-privilege access expectations.
4. Restore runbooks cover database-only, object-only and coordinated recovery, including application/schema compatibility checks.
5. Post-restore reconciliation can detect missing/orphaned managed objects and invalid durable references without destructive auto-fix by default.
6. A restore verification/drill can be executed in an isolated environment without destructive production actions.
7. Restore evidence records enough information to prove success while excluding credentials, signed URLs, private payloads and media contents.
8. TASK-041 release guidance treats successful backup/restore readiness as a distinct production gate rather than equating rollback with recovery.
9. Existing required `Frontend`, `Backend` and `Local Infrastructure` CI remains green; recovery checks added later are additive and safe.

## Quality / implementation notes
- Prefer managed provider backup/PITR primitives for the selected production topology where they satisfy the frozen RPO/RTO; do not rebuild database backup infrastructure inside application code without evidence.
- Keep provider-specific operational configuration outside domain/business packages.
- Restore validation must include both database state and object-storage referential integrity because MediaAsset metadata can outlive or diverge from stored objects.
- Recovery procedures should fail closed on incompatible schema/application versions.
- Never place production credentials, decrypted secrets, signed object URLs or private media in committed docs, CI logs or drill evidence.

## Dependencies / relations
- TASK-041 owns deployment/release mechanics, migration rollout and application rollback; TASK-044 owns independent data backup/restore/recovery.
- TASK-039 observability can improve recovery diagnosis but does not block planning this task.
- Existing MediaAsset and durable-job contracts define important post-restore integrity relationships.

## Activation gate
Remain BACKLOG until PM/TL confirms the first production database/object-storage topology, freezes RPO/RTO and provider recovery primitives, reruns duplicate/branch checks and reconciles implementation WIP capacity.

## Delivery
Developer implements operational/runtime changes on `feature/TASK-044-production-data-recovery` and opens a PR into `develop`. Developer must not self-merge or self-mark DONE.
