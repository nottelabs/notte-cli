.PHONY: build install clean test test-integration test-all lint fmt generate check setup help

VERSION ?= dev
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

# Go tools versions (use latest to match CI)
GOLANGCI_LINT_VERSION := latest

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

setup: ## Install development dependencies (linters, formatters, git hooks)
	@echo "Installing Go tools..."
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install golang.org/x/tools/cmd/goimports@latest
	go install mvdan.cc/gofumpt@latest
	@echo "Setting up git hooks..."
	@command -v lefthook >/dev/null || (echo "Install lefthook: brew install lefthook" && exit 1)
	lefthook install
	@echo "Setup complete!"

pre-commit-run: ## Run pre-commit hooks manually (full lint on all code)
	@echo "Running full lint on all code..."
	@golangci-lint run 2>&1 | grep -v "Failed to persist facts to cache" || (echo "Linting failed" && exit 1)
	@echo "Checking formatting..."
	@goimports -l . | grep . && (echo "Run 'make fmt' to fix formatting" && goimports -l . && exit 1) || exit 0
	@echo "Building..."
	@go build ./cmd/notte
	@echo "✓ All checks passed"

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
	@golangci-lint run 2>&1 | grep -v "Failed to persist facts to cache" || true

lint-fix: ## Run linters and fix issues
	golangci-lint run --fix

fmt: ## Format code
	goimports -w .
	gofumpt -w .

generate: ## Generate code (API client, etc.)
	./scripts/generate.sh

check: ## Verify generated code is up to date (fails if `make generate` would produce a diff)
	@echo "Checking for local changes in generated files..."
	@git diff --quiet -- internal/api/client.gen.go internal/api/property_names.gen.go 'internal/cmd/*_flags.gen.go' || \
		(echo "Error: generated files have uncommitted local changes. Commit or stash them before running 'make check'." && exit 2)
	@echo "Running code generation..."
	@./scripts/generate.sh >/dev/null
	@echo "Checking for diffs in generated files..."
	@git diff --exit-code -- internal/api/client.gen.go internal/api/property_names.gen.go 'internal/cmd/*_flags.gen.go' || \
		(echo "Generated code is out of date. Run 'make generate' and commit the changes." && exit 1)
	@echo "✓ Generated code is up to date"

.DEFAULT_GOAL := build
