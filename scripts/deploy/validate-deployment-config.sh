#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

export ROOT
python3 <<'PY'
import json
import os
import pathlib
import sys

root = pathlib.Path(os.environ["ROOT"])

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
}
forbidden_csp_header = "Content-Security-Policy"
found = set()
vercel_text = vercel.read_text()
if "onrender.com" in vercel_text.lower():
    sys.exit("apps/web/vercel.json must not hard-code provider-specific API hosts")
for group in headers:
    for header in group.get("headers", []):
        key = header.get("key")
        found.add(key)
        if key == forbidden_csp_header:
            sys.exit(
                "apps/web/vercel.json must not define static Content-Security-Policy; "
                "CSP connect-src is injected at build time from VITE_API_BASE_URL"
            )
missing = required_header_keys - found
if missing:
    sys.exit(f"apps/web/vercel.json missing security headers: {sorted(missing)}")

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

FIXTURE_API_BASE_URL="https://api.qa-fixture.synvideo.example"
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

node --test "${ROOT}/apps/web/scripts/production-csp.test.mjs"
echo "production-csp unit tests: OK"
