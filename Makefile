.PHONY: build install clean test lint fmt generate setup

VERSION ?= dev
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

setup:
	@command -v lefthook >/dev/null || (echo "Install lefthook: brew install lefthook" && exit 1)
	lefthook install

build:
	go build $(LDFLAGS) -o notte ./cmd/notte

install:
	go install $(LDFLAGS) ./cmd/notte

clean:
	rm -f notte
	go clean

test:
	go test -v ./...

lint:
	golangci-lint run

fmt:
	goimports -w .
	gofumpt -w .

generate:
	./scripts/generate.sh

.DEFAULT_GOAL := build
