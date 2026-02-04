.PHONY: build install clean test test-integration test-all lint fmt generate setup help

VERSION ?= dev
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

# Go tools versions
GOLANGCI_LINT_VERSION := v1.62.2

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

setup: ## Install development dependencies (linters, formatters, git hooks)
	@echo "Installing Go tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install golang.org/x/tools/cmd/goimports@latest
	go install mvdan.cc/gofumpt@latest
	@echo "Setting up git hooks..."
	@command -v lefthook >/dev/null || (echo "Install lefthook: brew install lefthook" && exit 1)
	lefthook install
	@echo "Setup complete!"

pre-commit-run: ## Run pre-commit hooks manually
	lefthook run pre-commit

pre-push-run: ## Run pre-push hooks manually
	lefthook run pre-push

build: ## Build the CLI binary
	go build $(LDFLAGS) -o notte ./cmd/notte

install: ## Install the CLI to GOPATH/bin
	go install $(LDFLAGS) ./cmd/notte

clean: ## Remove build artifacts
	rm -f notte
	go clean

test: ## Run unit tests
	go test -v -race -short ./...

test-coverage: ## Run unit tests with coverage
	go test -v -race -short -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-integration: ## Run integration tests (requires NOTTE_API_KEY)
	@if [ -z "$(NOTTE_API_KEY)" ]; then \
		echo "Error: NOTTE_API_KEY is required for integration tests"; \
		echo "Usage: NOTTE_API_KEY=your_key make test-integration"; \
		exit 1; \
	fi
	go test -v -tags=integration -timeout=20m ./tests/integration/...

test-all: test test-integration ## Run all tests (unit + integration)

lint: ## Run linters
	golangci-lint run

lint-fix: ## Run linters and fix issues
	golangci-lint run --fix

fmt: ## Format code
	goimports -w .
	gofumpt -w .

generate: ## Generate code (API client, etc.)
	./scripts/generate.sh

.DEFAULT_GOAL := build
