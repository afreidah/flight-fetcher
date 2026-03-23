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

run: ## Run locally (requires config.hcl)
	go run ./cmd/server -config config.hcl

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

.PHONY: help vet govulncheck lint test run push
.DEFAULT_GOAL := help
