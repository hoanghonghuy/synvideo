# Health and request observability

## Liveness

`GET /api/v1/healthz` is a cheap process-liveness check. It does not probe external dependencies and returns `200` while the HTTP process can serve the handler.

## Readiness

`GET /api/v1/readyz` is the traffic-readiness gate.

- When `SYNVIDEO_DATABASE_URL` is configured, PostgreSQL must accept a bounded ping.
- When media storage is configured, the configured object-storage bucket must be reachable through a bounded, non-mutating bucket-existence check.
- Unconfigured optional capabilities do not make the whole API unready.
- Each dependency probe is bounded by the application readiness timeout; a timeout or dependency failure returns `503` with a generic `{"status":"unready"}` response and does not expose connection strings, credentials, provider errors, bucket names, or internal failure details.
- Recovery is automatic: a later readiness request can return `200` once required dependencies recover.

Deployments should use `healthz` for process restart decisions and `readyz` for admission/load-balancer traffic decisions.

## Request correlation

Every API response includes `X-Request-ID`.

- A valid incoming UUID in `X-Request-ID` is propagated in canonical UUID form.
- Missing or invalid incoming values are replaced with a generated UUID.
- Request completion logs contain the request ID, method, matched Go route pattern, response status, and duration in milliseconds.
- Dynamic raw URL paths are not logged when a route template is available. Unmatched requests use the fixed route label `unmatched`.
- Panic logging uses the same request ID and intentionally omits panic payload/details so secrets or sensitive values are not emitted through this boundary.

This is the baseline correlation contract for future metrics/tracing work; callers may send their own valid UUID request ID when end-to-end correlation is required.
