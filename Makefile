.PHONY: help install dev-web dev-api infra-config infra-up infra-down verify-web verify-api verify-infra verify

help:
	@printf '%s\n' \
		'SynVideo developer commands:' \
		'  make install       Install frontend dependencies' \
		'  make dev-web       Start the Vue frontend' \
		'  make dev-api       Start the Go API' \
		'  make infra-up      Start local PostgreSQL and S3-compatible storage' \
		'  make verify        Run frontend, backend and infra checks'

install:
	npm install

dev-web:
	npm run dev:web

dev-api:
	cd apps/api && go run ./cmd/api

infra-config:
	docker compose -f infra/docker-compose.yml config

infra-up:
	docker compose -f infra/docker-compose.yml up -d

infra-down:
	docker compose -f infra/docker-compose.yml down

verify-web:
	npm run verify:web

verify-api:
	cd apps/api && test -z "$$(gofmt -l .)" && go vet ./... && go test ./... && go build ./cmd/api

verify-infra:
	docker compose -f infra/docker-compose.yml config

verify: verify-web verify-api verify-infra
