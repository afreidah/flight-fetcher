# -------------------------------------------------------------------------------
# Flight Fetcher - Build and Development
#
# Project: Flight Fetcher / Author: Alex Freidah
#
# Go service for polling OpenSky Network and enriching aircraft metadata.
# -------------------------------------------------------------------------------

REGISTRY   ?= registry.munchbox.cc
IMAGE      := flight-fetcher
PLATFORMS  := linux/amd64,linux/arm64

# -------------------------------------------------------------------------
# DEFAULT TARGET
# -------------------------------------------------------------------------

help: ## Display available Make targets
	@echo ""
	@echo "Available targets:"
	@echo ""
	@grep -E '^[a-zA-Z0-9_-]+:.*?## ' Makefile | \
		awk 'BEGIN {FS = ":.*?## "} {printf "  %-20s %s\n", $$1, $$2}'
	@echo ""

# -------------------------------------------------------------------------
# CODE GENERATION
# -------------------------------------------------------------------------

generate: ## Generate sqlc query code and interface mocks
	sqlc generate
	go generate ./...

migration: ## Create a new database migration file
	@read -p "Migration name: " name; \
	last=$$(ls internal/store/migrations/*.sql 2>/dev/null | sed 's/.*\///' | sort -n | tail -1 | grep -oE '^[0-9]+'); \
	next=$$(printf '%05d' $$(( $${last:-0} + 1 ))); \
	file="internal/store/migrations/$${next}_$${name}.sql"; \
	printf -- '-- +goose Up\n\n-- +goose Down\n' > "$$file"; \
	echo "Created $$file"

# -------------------------------------------------------------------------
# DEVELOPMENT
# -------------------------------------------------------------------------

vet: ## Run Go vet static analysis
	go vet ./...

govulncheck: ## Scan Go dependencies for known vulnerabilities
	govulncheck ./...

lint: ## Run Go linter
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1 run ./...

test: ## Run Go tests with coverage
	go test -race -cover ./...

run: ## Build and run the full stack via docker-compose (requires config.hcl)
	docker compose up --build

stop: ## Stop the docker-compose stack
	docker compose down

clean: ## Stop the stack and remove volumes
	docker compose down -v

# -------------------------------------------------------------------------
# DOCKER
# -------------------------------------------------------------------------

push: ## Build and push multi-arch images to registry
	@echo "Building and pushing $(REGISTRY)/$(IMAGE) for $(PLATFORMS)"
	docker buildx build \
	  --pull \
	  --platform $(PLATFORMS) \
	  -f deploy/Dockerfile \
	  -t $(REGISTRY)/$(IMAGE):latest \
	  --output type=image,push=true \
	  .

.PHONY: help generate migration vet govulncheck lint test run stop clean push
.DEFAULT_GOAL := help
