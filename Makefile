.PHONY: build test vet fmt run tidy down generate-types help

APP := overload-party-support

build: ## Build Docker image
	docker build -t $(APP) .

test: ## Run tests (Testcontainers; requires Docker running)
	go test ./... -count=1 -race

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy dependencies
	go mod tidy

fmt: ## Format code
	gofmt -s -w .

generate-types: ## Re-generate packages/api-support/openapi_gen.go from data/openapi.yaml (requires oapi-codegen on PATH)
	scripts/generate_types.sh

down: ## Stop the local stack and remove volumes
	HOST_GOMODCACHE=$$(go env GOMODCACHE) docker compose down -v

run: ## Run the full local stack (app + infra) in compose; edit source and restart `support` to reload
	GOWORK=off GOPRIVATE=github.com/kenyamaneko/* go mod download
	HOST_GOMODCACHE=$$(go env GOMODCACHE) docker compose up

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
