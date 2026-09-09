# Production deployment and release baseline (TASK-041)

This document is the authoritative runbook for the frozen first-production topology:

| Component | Provider | Notes |
| --- | --- | --- |
| Creator web | Vercel | Vite static SPA, web TLS + security headers |
| API + durable-job executors | Render | One Go web service replica, embedded job executor |
| PostgreSQL | Neon | Existing PostgreSQL contract |
| Object storage | AWS S3 | Existing S3-compatible interface, server-side credentials only |

Browser traffic is intentionally **cross-origin**: Vercel web origin → Render API origin.

## Build and runtime entrypoints

### Web (Vercel)

- **Repository path:** `apps/web`
- **Install (monorepo root):** `npm ci`
- **Build:** `npm run build` (runs `vue-tsc`, `vite build`)
- **Output:** `apps/web/dist`
- **Manifest:** `apps/web/vercel.json` (SPA rewrite + security headers)
- **Client API base:** `VITE_API_BASE_URL` (Render API origin, no trailing slash)

Vercel owns web TLS termination and response security headers (`Content-Security-Policy`, `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`). CSP `connect-src` includes `https://*.onrender.com` for the frozen Render API edge; tighten to the exact API origin when known.

### API (Render)

- **Dockerfile:** `apps/api/Dockerfile`
- **Blueprint:** `render.yaml`
- **Process entrypoint:** `/usr/local/bin/synvideo-api`
- **Migration entrypoint:** `/usr/local/bin/synvideo-migrate up` (Render `preDeployCommand`)
- **Listen address:** Render `PORT` (or `SYNVIDEO_API_ADDR` override)
- **Health gates:**
  - Liveness: `GET /api/v1/healthz` (process only)
  - Readiness / traffic admission: `GET /api/v1/readyz` (TASK-039 dependency probes)
- **Toolchain observability:** `GET /api/v1/runtime/toolchain` (FFmpeg/FFprobe version lines)

Migrations are an **explicit release step** via Render `preDeployCommand`. The API process does **not** auto-migrate on startup. If migration fails, Render aborts the deploy and the previous release keeps serving.

FFmpeg and FFprobe are installed in the production image and probed at runtime. Render export handlers remain disabled in production until a separate activation enables them; toolchain presence is still verified for media/render contracts.

## Production environment contract

See `.env.production.example` for the full variable list. Required API variables in `SYNVIDEO_ENV=production`:

| Variable | Owner | Purpose |
| --- | --- | --- |
| `SYNVIDEO_DATABASE_URL` | Neon / ops | PostgreSQL connection string (`sslmode=require`) |
| `SYNVIDEO_CORS_ALLOWED_ORIGINS` | Render / ops | Comma-separated HTTPS web origins (no `*`) |
| `SYNVIDEO_MEDIA_STORAGE_*` | AWS / ops | S3 bucket + credentials (never in Vite) |
| `SYNVIDEO_CREDENTIAL_ENCRYPTION_KEY` | Render secret | BYOK credential encryption |
| `VITE_API_BASE_URL` | Vercel env | Cross-origin API origin for browser fetch |

Forbidden in production:

- `SYNVIDEO_LOCAL_ACTOR_ID` (config validation rejects it)
- Any object-storage secret in Vite/client configuration

Connection pool numeric limits remain TASK-048; this document does not invent Neon pool ceilings.

## CORS boundary

The API applies an explicit origin allowlist from `SYNVIDEO_CORS_ALLOWED_ORIGINS`:

- Allowed methods: `GET, POST, PUT, PATCH, DELETE, OPTIONS`
- Allowed headers: `Accept, Authorization, Content-Type, X-Request-ID`
- Credentials: enabled for allowlisted origins (TASK-040 will own auth semantics)
- Wildcard `*` is rejected in production config validation
- Disallowed origins do not receive `Access-Control-Allow-Origin`; preflight returns `403`

## Release promotion (`develop` → `main` → production)

1. Merge feature work into protected `develop` via PR with required checks green: **Frontend**, **Backend**, **Local Infrastructure**.
2. PM/TL promotes a tested `develop` commit to `main` using the normal protected-branch flow (no bypass, no force-push).
3. Tag or otherwise record the production release input commit on `main`.
4. Vercel production deploy tracks the protected `main` (or release tag) web build input.
5. Render production deploy tracks the same commit via `render.yaml` / connected branch.
6. Before switching production traffic, run `scripts/deploy/smoke-production.sh` with live `WEB_ORIGIN` and `API_ORIGIN`.

Required evidence before go-live:

- Exact-head CI success on the promoted commit
- Migration `preDeployCommand` success in Render deploy logs
- `readyz` returns `200` on the new release
- Smoke script passes (web, liveness, readiness, toolchain, CORS allow/deny)

TASK-040 production authentication remains a separate gate before public creator exposure.

## Rollback and migration compatibility

| Scenario | Action | Constraint |
| --- | --- | --- |
| New app revision unhealthy after deploy | Roll back Render to previous image/release | Safe when schema unchanged |
| Migration `preDeployCommand` fails | Deploy aborts; old release continues | No incompatible schema applied |
| Forward-compatible migration deployed | May roll back app only if old code tolerates new schema | Requires migration review |
| Destructive / incompatible migration | Do **not** roll back app to old code | Forward-fix or restore DB per TASK-044 |

Never silently pair old application code with a schema it cannot read. When in doubt, keep the previous release serving and treat migration failure as a hard stop.

## Smoke verification

Local/CI config validation (no live credentials):

```bash
./scripts/deploy/validate-deployment-config.sh
```

Live post-deploy smoke (requires real Vercel + Render URLs):

```bash
WEB_ORIGIN=https://your-app.vercel.app \
API_ORIGIN=https://your-api.onrender.com \
./scripts/deploy/smoke-production.sh
```

Checks:

1. Web root serves `200`
2. API `healthz` liveness `200`
3. API `readyz` readiness `200` (DB + configured storage probes)
4. `runtime/toolchain` exposes FFmpeg/FFprobe versions
5. CORS preflight succeeds for allowed web origin
6. CORS preflight returns `403` for disallowed origin

## Secret injection and rotation

| Secret | Inject at | Rotate via |
| --- | --- | --- |
| `SYNVIDEO_DATABASE_URL` | Render secret | Neon credential rotation + Render update |
| `SYNVIDEO_MEDIA_STORAGE_*` | Render secret | AWS IAM key rotation |
| `SYNVIDEO_CREDENTIAL_ENCRYPTION_KEY` | Render secret | Planned re-encryption workflow (TASK-040 area) |
| `VITE_API_BASE_URL` | Vercel env | Update on API domain change + redeploy web |
| `SYNVIDEO_CORS_ALLOWED_ORIGINS` | Render env | Update when web origin changes |

## Diagnostics

| Symptom | Check |
| --- | --- |
| `503` on `readyz` | Neon connectivity, S3 credentials/bucket, Render logs |
| Browser CORS failure | `SYNVIDEO_CORS_ALLOWED_ORIGINS` matches exact Vercel origin (scheme + host) |
| Migration deploy failure | Render pre-deploy logs for `synvideo-migrate`; do not restart API hoping it migrates |
| Missing FFmpeg | `GET /api/v1/runtime/toolchain` and Dockerfile build logs |
| Upload failures | `SYNVIDEO_MEDIA_MAX_UPLOAD_BYTES`, reverse-proxy limits per `docs/operations/http-resource-bounds.md` |

## Related tasks

- TASK-039: liveness/readiness semantics (`docs/operations/HEALTH_AND_REQUEST_OBSERVABILITY.md`)
- TASK-040: production authentication (separate release gate)
- TASK-044: backup/restore policy
- TASK-048: Neon connection pool budgeting
