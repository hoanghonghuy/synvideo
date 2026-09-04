# TASK-040 — Production authentication & request principal boundary

Status: BACKLOG
Priority: P1
Base branch: `develop`
Canonical branch: `feature/TASK-040-production-auth`
Issue: #82

## Goal
Introduce a production-safe authentication and request-principal boundary so protected SynVideo APIs can establish a trusted caller identity without leaking authentication-provider concerns into project/domain logic.

## Problem / evidence
Current `apps/api/internal/actor/resolver.go` provides only `LocalResolver`. In production, `Resolve` always returns `ErrNoPrincipal`; outside production it ignores the HTTP request and trusts development-only `LocalActorID`.

SynVideo therefore has no production path that can authenticate a real caller and resolve that caller into the existing `project.Principal` ownership boundary. Protected creator/project APIs cannot safely become production-usable until that trust boundary exists.

Dedupe before issue #82 covered open/closed issues plus repository docs/code for authentication, authorization, principal, login and account ownership; no existing task owns this outcome.

## Scope
### Authentication boundary
- Define one supported V1 production authentication model and its trust boundary before implementation activation.
- Validate production request credentials explicitly and fail closed.
- Resolve a successfully authenticated caller to the existing `project.Principal` / owner boundary or an intentionally evolved equivalent.
- Keep authentication infrastructure isolated behind an HTTP/actor boundary rather than coupling domain packages directly to a concrete identity provider.

### Production/local separation
- Production must never derive identity from `LocalActorID`, development defaults, or an implicit anonymous owner.
- Preserve an explicit local-development-only identity path so local workflows remain usable.
- Configuration must make the environment distinction unambiguous and testable.

### HTTP and authorization semantics
- Missing, malformed, invalid or expired credentials return documented unauthenticated semantics.
- Authenticated callers without access to a project/resource continue to be rejected by ownership/authorization checks without resource leakage.
- Introducing authentication must not weaken existing project ownership enforcement.

### Credential safety
- Define validation, expiry and revocation/session assumptions appropriate to the selected V1 mechanism.
- Bound any remote validation/key/session lookup so authentication cannot hang indefinitely.
- Do not log or return raw authorization headers, cookies, bearer tokens, session secrets or sensitive identity claims.

### Verification and operations
- Add focused regression tests for valid authentication, missing/invalid/expired credentials, production no-local-fallback, local-development behavior and cross-owner access rejection.
- Document production auth configuration and credential/session lifecycle assumptions.

## Non-scope
- Full organization/team/RBAC collaboration model.
- Billing/subscription identity.
- Broad project authorization redesign unrelated to establishing a trustworthy caller.
- Social-login UX unless it is required by the selected V1 authentication mechanism.

## Acceptance criteria
1. Production protected APIs cannot obtain a principal from `LocalActorID` or any implicit local-development fallback.
2. A supported, verifiable production credential resolves deterministically to the caller principal used by project/resource authorization.
3. Missing/invalid/expired credentials fail with documented 401 semantics; authenticated callers lacking access fail with documented 403 or existing equivalent behavior without leaking protected resource existence.
4. Cross-owner project/resource access remains rejected after authentication is introduced.
5. Authentication validation is bounded and fails closed; identity-backend/key/session failures never silently authenticate a caller.
6. Raw credentials/secrets are absent from application logs and error responses.
7. Local development remains usable only through an explicitly non-production identity path, with tests proving production cannot enable that shortcut accidentally.
8. Regression tests cover successful auth, missing/invalid/expired auth, production no-local-fallback and authorization ownership behavior.
9. Deployment/configuration docs explain required production auth settings and credential/session lifecycle assumptions.
10. Required `Frontend`, `Backend` and `Local Infrastructure` checks remain green.

## Quality gate
- Follow `docs/engineering/TDD_PROTOCOL.md` for behavior changes.
- Do not weaken branch protection or required checks.
- Preserve the `actor.Resolver`-style abstraction where useful; do not pass vendor SDK/session objects into domain services.
- Prefer standards-based credential verification with explicit issuer/audience/key/session validation rather than custom cryptography.
- Re-run threat/abuse-path review for auth bypass, cross-owner access and credential leakage before merge.

## Dependencies / relations
- Existing project ownership enforcement is prerequisite context and must remain intact.
- Related to TASK-039 production readiness/observability, but the outcomes are independently implementable.
- Active feature implementation does not functionally block specification; bounded WIP still gates implementation activation.

## Activation gate
Remain BACKLOG until PM/TL freezes the concrete V1 authentication contract against the actual production/frontend deployment architecture, reruns duplicate/branch checks and reconciles implementation WIP capacity. If public production deployment becomes imminent, this task may be promoted as a release gate ahead of lower-priority backlog work.

## Delivery
Developer owns implementation on `feature/TASK-040-production-auth` and opens a PR into `develop`. Developer must not self-merge or self-mark DONE.
