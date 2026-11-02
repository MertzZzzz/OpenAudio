.PHONY: help up down logs web-install web-dev web-build web-test web-typecheck openapi-lint check api-run api-build api-test api-test-integration api-lint

help: ## Show targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

TEST_DATABASE_URL ?= postgres://openaudio:openaudio@localhost:5432/openaudio_test?sslmode=disable

up: ## Start PostgreSQL, MinIO, API and web (docker compose)
	docker compose up -d --build

down: ## Stop the stack
	docker compose down

logs: ## Tail compose logs
	docker compose logs -f

web-install: ## Install frontend dependencies
	cd web && npm ci

web-dev: ## Run the frontend dev server (mock API) on :5173
	cd web && npm run dev

web-build: ## Production build of the frontend
	cd web && npm run build

web-test: ## Run frontend unit/component tests
	cd web && npm test

web-typecheck: ## Strict TypeScript check
	cd web && npm run typecheck

api-run: ## Run the API locally (needs PostgreSQL; see .env.example)
	cd api && go run ./cmd/api

api-build: ## Build the API binary into api/bin/api
	cd api && go build -o bin/api ./cmd/api

api-test: ## Go unit + HTTP tests (Postgres tests are skipped)
	cd api && go test -race ./...

api-test-integration: ## Go tests including PostgreSQL integration tests
	cd api && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -race ./...

api-lint: ## gofmt + go vet
	cd api && test -z "$$(gofmt -l .)" && go vet ./...

openapi-lint: ## Validate api/openapi.yaml
	cd web && npx -y @redocly/cli@1 lint ../api/openapi.yaml

check: api-lint api-test web-typecheck web-test openapi-lint ## Everything CI runs (without DB)
