# -------------------------------------------------------------------------------
# Flight Fetcher - Build and Development
#
# Project: Flight Fetcher / Author: Alex Freidah
#
# Go service for polling OpenSky Network and enriching aircraft metadata.
# -------------------------------------------------------------------------------

REGISTRY   ?= registry.munchbox.cc
IMAGE      := flight-fetcher
VERSION    := $(shell cat .version)
PLATFORMS  := linux/amd64,linux/arm64
GO_LDFLAGS := -s -w -X main.Version=$(VERSION)

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

test: ## Run unit tests with coverage (skips integration tests)
	go test -short -race -cover ./...

test-integration: ## Run integration tests (requires Docker)
	go test -race -count=1 -timeout=5m ./internal/store/...

build: ## Build the flight-fetcher binary
	CGO_ENABLED=0 go build -ldflags="$(GO_LDFLAGS)" -o flight-fetcher ./cmd/server

run: ## Build and run the full stack via docker-compose (requires config.hcl)
	docker compose up --build -d
	@echo ""
	@echo "  Dashboard:  http://localhost:8080"
	@echo "  Grafana:    http://localhost:13000"
	@echo "  Prometheus: http://localhost:19090"
	@echo ""
	@echo "  Logs: docker compose logs -f flight-fetcher"
	@echo ""

stop: ## Stop the docker-compose stack
	docker compose down

clean: ## Stop the stack, remove volumes, and remove the binary
	docker compose down -v
	rm -f flight-fetcher

# -------------------------------------------------------------------------
# DOCKER
# -------------------------------------------------------------------------

push: ## Build and push multi-arch images to registry
	@echo "Building and pushing $(REGISTRY)/$(IMAGE):$(VERSION) for $(PLATFORMS)"
	docker buildx build \
	  --pull \
	  --platform $(PLATFORMS) \
	  --build-arg VERSION=$(VERSION) \
	  -f deploy/Dockerfile \
	  -t $(REGISTRY)/$(IMAGE):$(VERSION) \
	  -t $(REGISTRY)/$(IMAGE):latest \
	  --output type=image,push=true \
	  .

# -------------------------------------------------------------------------
# RELEASE
# -------------------------------------------------------------------------

changelog: ## Generate CHANGELOG.md from git history
	git cliff -o CHANGELOG.md

release: ## Tag and push to trigger a GitHub Release (reads .version)
	git tag $(VERSION)
	git push origin $(VERSION)

.PHONY: help generate migration vet govulncheck lint test build run stop clean push changelog release
.DEFAULT_GOAL := help
