.PHONY: start startm build all test race vet lint vuln cov clean test-events test-traefik

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
TUBY_NAVIRE_ENABLED=true
TUBY_NAVIRE_URL=http://10.70.80.2:8080
TUBY_NOTIFICATION_MESSAGES=["Congratulations! You built an entire app to avoid YouTube's algorithm, only to become your own personal disappointment engine. Impressive!","Another video! Drop everything! It's not like you had anything meaningful to do with your miserable existence anyway.","You escaped YouTube's recommendations just to build a system that recommends wasting your life. Genius! Your last two brain cells must be so proud.","Someone uploaded a video! Quick, abandon your responsibilities! It's not like you've ever been particularly good at handling them.","Your dopamine dealer has restocked. And look at you, crawling back like the pathetic little addict you are.","Another notification! Because apparently your empty existence needs constant reminders that strangers on the internet are more productive than you.","You spent three days building this shit just to receive notifications about people who actually accomplish things. The irony is fucking beautiful.","Your last two brain cells are fighting over who gets to watch this video. Neither has the qualifications.","Congratulations! You've successfully automated your own procrastination. Your parents must be so proud of what you've become.","Even YouTube's algorithm gave up trying to understand your pathetic viewing habits. Fortunately, you're stupid enough to do its job yourself.","Someone made another video about something you'll forget in ten minutes. But sure, let's pretend this is an important event in your life.","Your to-do list is rotting in a corner while you eagerly wait for strangers to upload more excuses for your spectacular lack of ambition.","Breaking news! Another human being has achieved something while you've been busy engineering increasingly sophisticated ways to do absolutely nothing.","You removed the algorithm to protect your attention span. Unfortunately, there's nothing left to protect. That ship sailed years ago.","Another video has arrived! Your productivity was already on life support, but apparently you felt the need to pull the fucking plug.","Your watch history is becoming a psychological case study. Scientists are baffled that someone can consume this much content and learn so little.","You could be doing something useful right now. But no, your self-hosted stupidity delivery system has another exciting announcement.","Congratulations! You've replaced corporate surveillance with locally hosted idiocy. Privacy achieved. Dignity not found.","Your future self called. He's furious, unemployed, and wondering why you spent so much time watching videos instead of fixing your miserable life.","Another notification from Tuby! Because nothing says 'I've got my shit together' quite like building infrastructure to manage your own inability to stop procrastinating."]


# RUNTIME VARIABLES
TUBY_PORT ?= 8090
TUBY_ALLOWED_CIDRS ?= *

build:
	@$(GO) build -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME) $(APP)

startm: export TUBY_PORT := $(TUBY_PORT)
startm: export TUBY_LOG_LEVEL := $(TUBY_LOG_LEVEL)
startm: export TUBY_ALLOWED_CIDRS := $(TUBY_ALLOWED_CIDRS)
startm: export TUBY_NAVIRE_ENABLED := $(TUBY_NAVIRE_ENABLED)
startm: export TUBY_NAVIRE_URL := $(TUBY_NAVIRE_URL)
startm: export TUBY_NOTIFICATION_MESSAGES := $(TUBY_NOTIFICATION_MESSAGES)
startm:
	@$(GO) run $(APP)

start: export TUBY_PORT := $(TUBY_PORT)
start: export TUBY_LOG_LEVEL := $(TUBY_LOG_LEVEL)
start: export TUBY_ALLOWED_CIDRS := $(TUBY_ALLOWED_CIDRS)
start: export TUBY_NAVIRE_ENABLED := $(TUBY_NAVIRE_ENABLED)
start: export TUBY_NAVIRE_URL := $(TUBY_NAVIRE_URL)
start:
	@$(GO) run $(APP)

all: vuln lint test race vet

test:
	@echo "🧪 Running tests..."
	@GOCACHE=$(GO_CACHE) $(GO) test -count=1 ./...

race:
	@echo "🧪 Running race tests..."
	@GOCACHE=$(GO_CACHE) $(GO) test -race -count=1 ./...

vet:
	@echo "🔎 Running go vet..."
	@GOCACHE=$(GO_CACHE) $(GO) vet ./...

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
