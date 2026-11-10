.PHONY: start build all test lint vuln cov clean test-events test-traefik

GO := go
APP := ./cmd/tuby/
APP_NAME := tuby
VERSION_PACKAGE := git.nexacloud.dev/MrSnakeDoc/tuby/internal/version
VERSION ?= dev
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GO_CACHE ?= $(HOME)/.cache/go/build
LINT_CACHE ?= $(HOME)/.cache/golangci-lint
LDFLAGS := -X $(VERSION_PACKAGE).Version=$(VERSION) \
	-X $(VERSION_PACKAGE).Commit=$(GIT_COMMIT) \
	-X $(VERSION_PACKAGE).BuildDate=$(BUILD_DATE)
TUBY_LOG_LEVEL=debug

# RUNTIME VARIABLES
TUBY_PORT ?= 8090
TUBY_ALLOWED_CIDRS ?= *

build:
	@$(GO) build -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME) $(APP)

start:
	@TUBY_PORT=$(TUBY_PORT) TUBY_LOG_LEVEL=$(TUBY_LOG_LEVEL) TUBY_ALLOWED_CIDRS=$(TUBY_ALLOWED_CIDRS) $(GO) run $(APP)

all: vuln lint test

test:
	@echo "🧪 Running tests..."
	@GOCACHE=$(GO_CACHE) $(GO) test -count=1 ./...

vuln:
	@echo "🔍 Running vulnerability scan..."
	@govulncheck ./...
	@echo $?

lint:
	@echo "Running linters..."
	@GOCACHE=$(GO_CACHE) GOLANGCI_LINT_CACHE=$(LINT_CACHE) golangci-lint run --config .golangci.yml

clean:
	@echo "🧹 Cleaning binaries and coverage..."
	@rm -rf bin coverage.out coverage.html

cov:
	@echo "🧪 Running tests with coverage..."
	@$(GO) test ./... -covermode=atomic -coverprofile=coverage.out
	@$(GO) tool cover -func=coverage.out | tail -n1
	@$(GO) tool cover -html=coverage.out -o coverage.html
