# SynVideo

AI-assisted video production workspace for short-form and long-form content.

Active development happens on the `develop` branch. Stable/release-ready work is promoted to `main`.

## Repository Structure

```text
apps/
  web/       Vue 3 + TypeScript frontend
  api/       Go backend API
infra/       Local development infrastructure
docs/        Product, engineering, decision and task records
```

## Prerequisites

- Node.js 24 LTS
- npm 11 or 12
- Go matching `apps/api/go.mod`
- Docker with Docker Compose v2

## Local Bootstrap

Install frontend dependencies:

```sh
npm install
```

Start local infrastructure:

```sh
docker compose -f infra/docker-compose.yml up -d
```

Apply database migrations:

```sh
make migrate
```

Start the frontend:

```sh
npm run dev:web
```

Start the backend API:

```sh
cd apps/api
set -a; . ../../.env.example; set +a
go run ./cmd/api
```

The frontend runs on Vite's default port unless overridden by Vite CLI options. The API listens on `:8080` by default and exposes:

- `GET /api/v1/healthz`
- `GET /api/v1/readyz`

Development-only infrastructure defaults are documented in `.env.example`.

Local object storage uses SeaweedFS `4.44` as a development-only S3-compatible endpoint on `http://localhost:8333`. Configure the media API with the `SYNVIDEO_MEDIA_STORAGE_*` variables in `.env.example`; legacy `SYNVIDEO_S3_*` aliases remain accepted for local compatibility. MinIO Community Edition is intentionally not used for this foundation because its public repository is no longer maintained; production storage remains provider-neutral and should be selected by a later task.

Project routes use `SYNVIDEO_LOCAL_ACTOR_ID` only in `development` and `test`. Production rejects that local actor fallback so project data is not accidentally exposed before a real authentication task exists.

When media storage is configured, the API exposes bounded upload/list/metadata/content/delete routes under `/api/v1/projects/{project_id}/media-assets` and approved scene primary-visual binding routes under `/api/v1/projects/{project_id}/scene-plans/{version}`. Content serving supports full downloads and one standard byte range for preview playback; storage credentials and object keys are never returned by the API.

## Verification

Frontend:

```sh
npm run verify:web
```

Backend:

```sh
cd apps/api
SYNVIDEO_DATABASE_URL=postgres://synvideo:synvideo_dev_password@localhost:5432/synvideo?sslmode=disable go run ./cmd/migrate up
test -z "$(gofmt -l .)"
go vet ./...
SYNVIDEO_DATABASE_URL=postgres://synvideo:synvideo_dev_password@localhost:5432/synvideo?sslmode=disable SYNVIDEO_TEST_DATABASE_URL=postgres://synvideo:synvideo_dev_password@localhost:5432/synvideo?sslmode=disable go test ./...
go build ./cmd/api
```

Infrastructure configuration:

```sh
docker compose -f infra/docker-compose.yml config
```

Equivalent shortcuts are available through `make help`.
