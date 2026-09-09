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
import sys

root = pathlib.Path(os.environ["ROOT"])

vercel_config = root / "vercel.mjs"
apps_web_vercel_config = root / "apps/web/vercel.mjs"
vercel_json = root / "apps/web/vercel.json"
root_lockfile = root / "package-lock.json"
apps_web_lockfile = root / "apps/web/package-lock.json"
root_package = root / "package.json"
render = root / "render.yaml"
dockerfile = root / "apps/api/Dockerfile"

for path in (vercel_config, root_lockfile, root_package, render, dockerfile):
    if not path.is_file():
        sys.exit(f"missing required deployment file: {path}")

if apps_web_vercel_config.is_file():
    sys.exit(
        "apps/web/vercel.mjs must not be used when Vercel Root Directory is the repository root; use vercel.mjs at repository root"
    )

if vercel_json.is_file():
    sys.exit("apps/web/vercel.json must not coexist with vercel.mjs; use dynamic Vercel deployment config only")

if apps_web_lockfile.is_file():
    sys.exit("apps/web/package-lock.json must not exist; the npm workspace lockfile lives at repository root")

vercel_text = vercel_config.read_text()
if "onrender.com" in vercel_text.lower():
    sys.exit("vercel.mjs must not hard-code provider-specific API hosts")
if "vercel-config.mjs" not in vercel_text:
    sys.exit("vercel.mjs must import the shared Vercel deployment config builder")
if "sync-vercel-csp" in vercel_text:
    sys.exit("vercel.mjs must not mutate vercel.json via sync-vercel-csp")

vercel_config_builder = root / "apps/web/scripts/vercel-config.mjs"
if "production-csp.mjs" not in vercel_config_builder.read_text():
    sys.exit("apps/web/scripts/vercel-config.mjs must import the shared production CSP builder")

root_package_data = json.loads(root_package.read_text())
workspaces = root_package_data.get("workspaces", [])
if "apps/web" not in workspaces:
    sys.exit("root package.json must declare apps/web in npm workspaces")
scripts = root_package_data.get("scripts", {})
if scripts.get("build:web") != "npm --workspace apps/web run build":
    sys.exit("root package.json must expose build:web for the Vercel monorepo build command")

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

node --test "${ROOT}/apps/web/scripts/vercel-config.test.mjs"
echo "vercel-config unit tests: OK"

echo "validating dynamic Vercel deployment config for ${FIXTURE_API_BASE_URL}"
export FIXTURE_API_BASE_URL
(
  cd "${ROOT}"
  node --input-type=module <<'NODE'
import assert from 'node:assert/strict'

import {
  REQUIRED_SECURITY_HEADER_KEYS,
  VERCEL_MONOREPO_CONTRACT,
  buildVercelDeploymentConfig,
  getVercelContentSecurityPolicy,
  requireConfiguredApiBaseUrl,
} from './apps/web/scripts/vercel-config.mjs'

const apiBaseUrl = process.env.FIXTURE_API_BASE_URL

assert.throws(() => requireConfiguredApiBaseUrl(''), /VITE_API_BASE_URL is required/)

const config = buildVercelDeploymentConfig(apiBaseUrl)
assert.equal(config.installCommand, VERCEL_MONOREPO_CONTRACT.installCommand)
assert.equal(config.buildCommand, VERCEL_MONOREPO_CONTRACT.buildCommand)
assert.equal(config.outputDirectory, VERCEL_MONOREPO_CONTRACT.outputDirectory)

const headerKeys = new Set(
  config.headers.flatMap((group) => group.headers.map((header) => header.key)),
)
for (const key of REQUIRED_SECURITY_HEADER_KEYS) {
  if (!headerKeys.has(key)) {
    console.error(`vercel deployment config is missing security header: ${key}`)
    process.exit(1)
  }
}

const csp = getVercelContentSecurityPolicy(config)
if (!csp.includes(`connect-src 'self' ${apiBaseUrl}`)) {
  console.error('vercel deployment config CSP is missing configured API origin in connect-src')
  process.exit(1)
}
if (csp.includes('*')) {
  console.error('vercel deployment config CSP must not use wildcard connect hosts')
  process.exit(1)
}
if (/onrender\.com/i.test(csp)) {
  console.error('vercel deployment config CSP contains provider-specific host hard-coding')
  process.exit(1)
}

process.env.VITE_API_BASE_URL = apiBaseUrl
const { config: exportedConfig } = await import('./vercel.mjs')
const exportedCsp = getVercelContentSecurityPolicy(exportedConfig)
if (!exportedCsp.includes(`connect-src 'self' ${apiBaseUrl}`)) {
  console.error('vercel.mjs export is missing configured API origin in connect-src')
  process.exit(1)
}
NODE
)

echo "vercel deployment config validation: OK"

echo "validating Vercel monorepo install/build path from repository root"
(
  cd "${ROOT}"
  npm ci >/tmp/synvideo-vercel-install.log 2>&1
  VITE_API_BASE_URL="${FIXTURE_API_BASE_URL}" npm run build:web >/tmp/synvideo-vercel-build.log 2>&1
  test -f "${ROOT}/apps/web/dist/index.html"
  grep -q "connect-src 'self' ${FIXTURE_API_BASE_URL}" "${ROOT}/apps/web/dist/index.html"
  if grep -qi 'onrender.com' "${ROOT}/apps/web/dist/index.html"; then
    echo "built index.html contains hard-coded onrender.com CSP host" >&2
    exit 1
  fi
)
echo "vercel monorepo install/build validation: OK"
