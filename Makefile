.PHONY: test lint build bench clean help install-lint

GOLANGCI_LINT_BIN := $(shell go env GOPATH)/bin/golangci-lint

# Default target
all: lint test build

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

## test: Run all tests
test:
go test -v ./...

install-lint:
	GOTOOLCHAIN=go1.26.3 go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2

## lint: Run golangci-lint
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	elif [ -x "$(GOLANGCI_LINT_BIN)" ]; then \
		"$(GOLANGCI_LINT_BIN)" run; \
	else \
		echo "golangci-lint is not installed. Run 'make install-lint' first."; \
		exit 1; \
	fi

## build: Build the router and config-server binaries
build:
	go build -o bin/router ./cmd/server
	go build -o bin/config-server ./cmd/config-server

## bench: Run benchmarks
bench:
	go test -bench=. -benchmem ./pkg/...

## clean: Remove built binaries
clean:
	rm -rf bin/
