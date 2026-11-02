.PHONY: help up down logs web-install web-dev web-build web-test web-typecheck openapi-lint check

help: ## Show targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

up: ## Start PostgreSQL, MinIO and the web app (docker compose)
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

openapi-lint: ## Validate api/openapi.yaml
	cd web && npx -y @redocly/cli@1 lint ../api/openapi.yaml

check: web-typecheck web-test openapi-lint ## Everything CI runs
