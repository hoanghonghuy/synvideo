#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FIXTURE_API_BASE_URL="https://api.qa-fixture.synvideo.example"

export ROOT
export FIXTURE_API_BASE_URL
python3 <<'PY'
import json
import os
import pathlib
import re
import sys

root = pathlib.Path(os.environ["ROOT"])
fixture_api_base_url = os.environ["FIXTURE_API_BASE_URL"]

vercel = root / "apps/web/vercel.json"
render = root / "render.yaml"
dockerfile = root / "apps/api/Dockerfile"

for path in (vercel, render, dockerfile):
    if not path.is_file():
        sys.exit(f"missing required deployment file: {path}")

vercel_data = json.loads(vercel.read_text())
headers = vercel_data.get("headers", [])
required_header_keys = {
    "X-Content-Type-Options",
    "X-Frame-Options",
    "Referrer-Policy",
    "Content-Security-Policy",
}
found = set()
csp_value = ""
vercel_text = vercel.read_text()
if "onrender.com" in vercel_text.lower():
    sys.exit("apps/web/vercel.json must not hard-code provider-specific API hosts")
for group in headers:
    for header in group.get("headers", []):
        key = header.get("key")
        found.add(key)
        if key == "Content-Security-Policy":
            csp_value = header.get("value", "")
missing = required_header_keys - found
if missing:
    sys.exit(f"apps/web/vercel.json missing security headers: {sorted(missing)}")
if not csp_value:
    sys.exit("apps/web/vercel.json must define a Content-Security-Policy response header")
if "*" in csp_value.replace("'self'", ""):
    sys.exit("apps/web/vercel.json CSP must not use wildcard connect hosts")

render_text = render.read_text()
if "preDeployCommand: /usr/local/bin/synvideo-migrate up" not in render_text:
    sys.exit("render.yaml must run explicit migrations before app deploy")
if "healthCheckPath: /api/v1/readyz" not in render_text:
    sys.exit("render.yaml must gate traffic on /api/v1/readyz")

docker_text = dockerfile.read_text()
if "ffmpeg" not in docker_text:
    sys.exit("apps/api/Dockerfile must install ffmpeg for production runtime")
if "synvideo-migrate" not in docker_text:
    sys.exit("apps/api/Dockerfile must ship migration binary")

print("deployment config validation passed")
PY

echo "render.yaml syntax: OK"
test -f "${ROOT}/.env.production.example"
grep -q 'SYNVIDEO_CORS_ALLOWED_ORIGINS' "${ROOT}/.env.production.example"
grep -q 'VITE_API_BASE_URL' "${ROOT}/.env.production.example"
echo ".env.production.example: OK"

node --test "${ROOT}/apps/web/scripts/production-csp.test.mjs"
echo "production-csp unit tests: OK"

FIXTURE_VERCEL_JSON="$(mktemp)"
cp "${ROOT}/apps/web/vercel.json" "${FIXTURE_VERCEL_JSON}"

echo "validating Vercel edge CSP alignment for ${FIXTURE_API_BASE_URL}"
export FIXTURE_VERCEL_JSON
(
  cd "${ROOT}"
  node --input-type=module <<'NODE'
import { readFileSync } from 'node:fs'

import { syncVercelContentSecurityPolicy } from './apps/web/scripts/sync-vercel-csp.mjs'

const vercelPath = process.env.FIXTURE_VERCEL_JSON
const apiBaseUrl = process.env.FIXTURE_API_BASE_URL

syncVercelContentSecurityPolicy({
  apiBaseUrl,
  vercelPath,
  requireApiBaseUrl: true,
})

const synced = readFileSync(vercelPath, 'utf8')
if (!synced.includes(`connect-src 'self' ${apiBaseUrl}`)) {
  console.error('synced vercel.json CSP is missing configured API origin in connect-src')
  process.exit(1)
}
if (/onrender\.com/i.test(synced)) {
  console.error('synced vercel.json CSP contains provider-specific host hard-coding')
  process.exit(1)
}
NODE
)

echo "vercel edge CSP validation: OK"

echo "validating build-time CSP alignment for ${FIXTURE_API_BASE_URL}"
(
  cd "${ROOT}/apps/web"
  VITE_API_BASE_URL="${FIXTURE_API_BASE_URL}" npm run build >/tmp/synvideo-csp-build.log 2>&1
  grep -q "connect-src 'self' ${FIXTURE_API_BASE_URL}" dist/index.html
  if grep -qi 'onrender.com' dist/index.html; then
    echo "built index.html contains hard-coded onrender.com CSP host" >&2
    exit 1
  fi
)
echo "build-time CSP validation: OK"
