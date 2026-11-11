.PHONY: help build run test lint fmt vet tidy install clean

BINARY_NAME := share-with
BIN_DIR := bin
GO ?= GOTELEMETRY=off GOTOOLCHAIN=local go

help: ## Show available Makefile targets
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## Build the share-with binary into bin/share-with
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME) .

run: ## Run share-with from source (pass ARGS="...")
	$(GO) run . $(ARGS)

test: ## Run all unit and CLI contract tests
	$(GO) test -v -race ./...

fmt: ## Format all Go source files with gofmt
	gofmt -s -w .

vet: ## Run go vet static checks
	$(GO) vet ./...

lint: vet ## Run golangci-lint if installed, falling back to go vet
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found; go vet passed"; \
	fi

tidy: ## Tidy Go module dependencies
	$(GO) mod tidy

install: ## Install share-with to GOPATH/bin
	$(GO) install .

clean: ## Remove compiled binaries and test artifacts
	rm -rf $(BIN_DIR) dist coverage.txt
