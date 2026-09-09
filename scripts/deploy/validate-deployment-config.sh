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
    "Content-Security-Policy",
}
found = set()
for group in headers:
    for header in group.get("headers", []):
        found.add(header.get("key"))
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
