.PHONY: help build snapshot site-build run test lint fmt vet validate tidy install clean

BINARY_NAME := share-with
BIN_DIR := bin
GO ?= GOTELEMETRY=off GOTOOLCHAIN=local go
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X github.com/ghchinoy/share-with/cmd.Commit=$(COMMIT) -X github.com/ghchinoy/share-with/cmd.Date=$(BUILD_DATE)

help: ## Show available Makefile targets
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## Build the share-with binary into bin/share-with
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) .

snapshot: ## Cross-compile local snapshot archives into dist/ via GoReleaser
	GOTELEMETRY=off GOTOOLCHAIN=local goreleaser release --snapshot --clean

site-build: ## Typecheck and build the Astro + Starlight site under ./site
	cd site && ASTRO_TELEMETRY_DISABLED=1 npm run check && ASTRO_TELEMETRY_DISABLED=1 npm run build

run: ## Run share-with from source (pass ARGS="...")
	$(GO) run . $(ARGS)

test: ## Run all unit and CLI contract tests
	$(GO) test -v -race ./...

fmt: ## Format all Go source files with gofmt
	gofmt -s -w .

vet: ## Run go vet static checks
	$(GO) vet ./...

validate: ## Validate Agent Plugin manifests, skill metadata, and portable URL hygiene
	./scripts/validate-plugin.sh

lint: vet validate ## Run go vet, plugin/hygiene validation, and golangci-lint if installed
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found; go vet and plugin validation passed"; \
	fi

tidy: ## Tidy Go module dependencies
	$(GO) mod tidy

install: ## Install share-with to GOPATH/bin
	$(GO) install .

clean: ## Remove compiled binaries and test artifacts
	rm -rf $(BIN_DIR) dist coverage.txt
