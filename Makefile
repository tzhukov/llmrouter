.PHONY: test lint build bench clean help

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

## lint: Run golangci-lint
lint:
	golangci-lint run

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
