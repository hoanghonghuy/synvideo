#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

export SYNVIDEO_ENV=test
export SYNVIDEO_DATABASE_URL="${SYNVIDEO_DATABASE_URL:-postgres://synvideo:synvideo_dev_password@localhost:5432/synvideo?sslmode=disable}"
export SYNVIDEO_TEST_DATABASE_URL="${SYNVIDEO_TEST_DATABASE_URL:-$SYNVIDEO_DATABASE_URL}"
export SYNVIDEO_LOCAL_ACTOR_ID="${SYNVIDEO_LOCAL_ACTOR_ID:-11111111-1111-4111-8111-111111111111}"
export SYNVIDEO_MEDIA_STORAGE_ENDPOINT="${SYNVIDEO_MEDIA_STORAGE_ENDPOINT:-http://localhost:8333}"
export SYNVIDEO_MEDIA_STORAGE_REGION="${SYNVIDEO_MEDIA_STORAGE_REGION:-local}"
export SYNVIDEO_MEDIA_STORAGE_BUCKET="${SYNVIDEO_MEDIA_STORAGE_BUCKET:-synvideo-local}"
export SYNVIDEO_MEDIA_STORAGE_ACCESS_KEY_ID="${SYNVIDEO_MEDIA_STORAGE_ACCESS_KEY_ID:-synvideo}"
export SYNVIDEO_MEDIA_STORAGE_SECRET_ACCESS_KEY="${SYNVIDEO_MEDIA_STORAGE_SECRET_ACCESS_KEY:-synvideo_dev_password}"
export SYNVIDEO_MEDIA_STORAGE_PATH_STYLE="${SYNVIDEO_MEDIA_STORAGE_PATH_STYLE:-true}"

echo "==> Validate local infrastructure"
docker compose -f infra/docker-compose.yml config >/dev/null
docker compose -f infra/docker-compose.yml up -d --wait postgres seaweed-s3

echo "==> Frontend checks"
npm ci --prefer-offline --no-audit --no-fund
npm run lint:web
npm run typecheck:web
npm run test:web
npm run build:web

echo "==> Backend checks"
cd apps/api
unformatted="$(gofmt -l .)"
if [ -n "$unformatted" ]; then
  echo "Unformatted Go files:" >&2
  echo "$unformatted" >&2
  exit 1
fi
go run ./cmd/migrate up
go vet ./...
go test ./...
go build ./cmd/api

echo "Pre-push verification passed."
