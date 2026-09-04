#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

RUN_ID="${SYNVIDEO_E2E_RUN_ID:-$(date +%s)-$$}"
SAFE_RUN_ID="$(printf '%s' "$RUN_ID" | tr -cd '[:alnum:]_' | cut -c1-32)"
COMPOSE_PROJECT_NAME="synvideo-e2e-${SAFE_RUN_ID:-run}"
POSTGRES_PORT="${SYNVIDEO_E2E_POSTGRES_PORT:-55432}"
S3_PORT="${SYNVIDEO_E2E_S3_PORT:-58333}"
DB_NAME="synvideo_e2e_${SAFE_RUN_ID:-run}"
ARTIFACT_DIR="$ROOT_DIR/e2e/artifacts"
mkdir -p "$ARTIFACT_DIR"

export SYNVIDEO_E2E_RUN_ID="$RUN_ID"
export POSTGRES_DB="$DB_NAME"
export POSTGRES_USER=synvideo
export POSTGRES_PASSWORD=synvideo_dev_password
export POSTGRES_PORT
export SEAWEEDFS_S3_PORT="$S3_PORT"
export SYNVIDEO_ENV=test
export SYNVIDEO_API_ADDR=127.0.0.1:8080
export SYNVIDEO_DATABASE_URL="postgres://synvideo:synvideo_dev_password@127.0.0.1:${POSTGRES_PORT}/${DB_NAME}?sslmode=disable"
export SYNVIDEO_TEST_DATABASE_URL="$SYNVIDEO_DATABASE_URL"
export SYNVIDEO_LOCAL_ACTOR_ID=11111111-1111-4111-8111-111111111111
export SYNVIDEO_MEDIA_STORAGE_ENDPOINT="http://127.0.0.1:${S3_PORT}"
export SYNVIDEO_MEDIA_STORAGE_REGION=local
export SYNVIDEO_MEDIA_STORAGE_BUCKET="synvideo-e2e-${SAFE_RUN_ID:-run}"
export SYNVIDEO_MEDIA_STORAGE_ACCESS_KEY_ID=synvideo
export SYNVIDEO_MEDIA_STORAGE_SECRET_ACCESS_KEY=synvideo_dev_password
export SYNVIDEO_MEDIA_STORAGE_PATH_STYLE=true
export SYNVIDEO_MEDIA_STORAGE_TIMEOUT=5s

API_PID=''
WEB_PID=''

redact() {
  sed -E \
    -e 's/(Authorization:)[^ ]+/\1 [REDACTED]/Ig' \
    -e 's/(Cookie:)[^ ]+/\1 [REDACTED]/Ig' \
    -e 's/(SYNVIDEO_MEDIA_STORAGE_SECRET_ACCESS_KEY=)[^ ]+/\1[REDACTED]/g'
}

cleanup() {
  status=$?
  set +e
  if [ -n "$API_PID" ]; then kill "$API_PID" 2>/dev/null || true; fi
  if [ -n "$WEB_PID" ]; then kill "$WEB_PID" 2>/dev/null || true; fi
  docker compose -p "$COMPOSE_PROJECT_NAME" -f infra/docker-compose.yml logs --no-color 2>&1 | redact > "$ARTIFACT_DIR/infra.log" || true
  docker compose -p "$COMPOSE_PROJECT_NAME" -f infra/docker-compose.yml down -v --remove-orphans >/dev/null 2>&1 || true
  exit "$status"
}
trap cleanup EXIT INT TERM

wait_http() {
  url="$1"
  name="$2"
  for _ in $(seq 1 60); do
    if curl -fsS "$url" >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  echo "$name did not become ready at $url" >&2
  return 1
}

docker compose -p "$COMPOSE_PROJECT_NAME" -f infra/docker-compose.yml up -d --wait

(
  cd apps/api
  go run ./cmd/migrate up
  exec go run ./cmd/api
) > >(redact > "$ARTIFACT_DIR/api.log") 2>&1 &
API_PID=$!
wait_http http://127.0.0.1:8080/readyz API

npm run dev:web -- --host 127.0.0.1 --port 4173 > >(redact > "$ARTIFACT_DIR/web.log") 2>&1 &
WEB_PID=$!
wait_http http://127.0.0.1:4173/projects Web

# Keep the existing repository lockfile untouched while pinning the acceptance runner.
npm install --no-save --package-lock=false @playwright/test@1.55.0
npx playwright install chromium
npx playwright test --config e2e/playwright.config.ts
