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

Start the frontend:

```sh
npm run dev:web
```

Start the backend API:

```sh
cd apps/api
go run ./cmd/api
```

The frontend runs on Vite's default port unless overridden by Vite CLI options. The API listens on `:8080` by default and exposes:

- `GET /api/v1/healthz`
- `GET /api/v1/readyz`

Development-only infrastructure defaults are documented in `.env.example`.

Local object storage uses SeaweedFS `4.44` as a development-only S3-compatible endpoint on `http://localhost:8333`. MinIO Community Edition is intentionally not used for this foundation because its public repository is no longer maintained; production storage remains provider-neutral and should be selected by a later task.

## Verification

Frontend:

```sh
npm run verify:web
```

Backend:

```sh
cd apps/api
test -z "$(gofmt -l .)"
go vet ./...
go test ./...
go build ./cmd/api
```

Infrastructure configuration:

```sh
docker compose -f infra/docker-compose.yml config
```

Equivalent shortcuts are available through `make help`.
