# --- Variables ---
APP         := bootforge
CMD         := ./cmd/bootforge
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_DIRTY   := $(shell test -z "$$(git status --porcelain 2>/dev/null)" && echo false || echo true)
BUILD_TYPE  ?= debug
LDFLAGS     := -X bootforge/internal/buildinfo.Version=$(VERSION) \
               -X bootforge/internal/buildinfo.GitCommit=$(GIT_COMMIT) \
               -X bootforge/internal/buildinfo.GitDirty=$(GIT_DIRTY) \
               -X bootforge/internal/buildinfo.BuildType=$(BUILD_TYPE)
CONFIG      ?= ./testdata/config/valid/minimal

# --- Build ---
.PHONY: build release clean

build:                          ## Build debug binary
	go build -ldflags "$(LDFLAGS)" -o $(APP) $(CMD)

release:                        ## Build release binary (stripped, optimized)
	BUILD_TYPE=release go build -ldflags "-s -w $(LDFLAGS)" -o $(APP) $(CMD)

clean:                          ## Remove binary and build artifacts
	rm -f $(APP)
	go clean -cache -testcache

# --- Development ---
.PHONY: run dev validate init

run: build                      ## Build and run server
	./$(APP) serve --config $(CONFIG)

dev: build                      ## Build and run server with debug logging
	./$(APP) serve --config $(CONFIG) --debug

validate: build                 ## Validate config without starting
	./$(APP) validate --config $(CONFIG)

init: build                     ## Scaffold a new config directory
	./$(APP) init --dir /tmp/bootforge-dev

# --- Quality ---
.PHONY: test test-race test-v lint vet check

test:                           ## Run all tests
	go test ./...

test-race:                      ## Run all tests with race detector
	go test -race ./...

test-v:                         ## Run all tests verbose
	go test -v -race ./...

lint:                           ## Run linter
	golangci-lint run

vet:                            ## Run go vet
	go vet ./...

check: vet test-race            ## Run all checks (vet + tests)

# --- Convenience ---
.PHONY: deps tidy version help

deps:                           ## Download dependencies
	go mod download

tidy:                           ## Tidy go.mod
	go mod tidy

version: build                  ## Show version info
	./$(APP) version

help:                           ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
