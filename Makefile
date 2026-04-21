# go-fit Makefile — ergonomics layer over common dev/ops tasks.
#
# Run `make help` for a list of targets.

SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

# --- Project metadata ---------------------------------------------------------
BINARY       := api
PKG          := ./cmd/api
BIN_DIR      := bin
COVER_FILE   := coverage.out

# Version/build metadata propagated to `go build -ldflags` and docker builds.
GIT_SHA      := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION      ?= dev

# Docker image coordinates — override on the CLI for local pushes, e.g.
#   make docker-build IMAGE=ghcr.io/me/go-fit TAG=v1.2.3
IMAGE        ?= go-fit
TAG          ?= local

# Compose files
COMPOSE       := docker compose
COMPOSE_PROD  := $(COMPOSE) -f docker-compose.yml -f docker-compose.prod.yml

# Test DB DSN (see docker-compose.yml test_db service).
TEST_DATABASE_URL ?= postgres://postgres:postgres@localhost:5433/postgres?sslmode=disable

# --- Help ---------------------------------------------------------------------
.PHONY: help
help: ## Show this help
	@awk 'BEGIN{FS=":.*?## "; printf "\nUsage: make \033[36m<target>\033[0m\n\n"} \
	     /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# --- Go dev loop --------------------------------------------------------------
.PHONY: run
run: ## Run the API locally via `go run`
	go run $(PKG)

.PHONY: build
build: ## Build the API binary into bin/
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build \
		-trimpath \
		-ldflags="-s -w -X main.version=$(VERSION) -X main.commit=$(GIT_SHA) -X main.buildDate=$(BUILD_DATE)" \
		-o $(BIN_DIR)/$(BINARY) $(PKG)

.PHONY: tidy
tidy: ## Run `go mod tidy`
	go mod tidy

.PHONY: fmt
fmt: ## Format code with gofmt
	gofmt -s -w .

.PHONY: vet
vet: ## Run `go vet`
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run ./...

# --- Tests --------------------------------------------------------------------
.PHONY: test
test: ## Run unit tests with race detector
	go test -race -count=1 ./...

.PHONY: test-integration
test-integration: ## Run integration tests against the test_db compose service
	DATABASE_URL='$(TEST_DATABASE_URL)' APP_ENV=development \
		go test -race -count=1 -tags=integration ./...

.PHONY: cover
cover: ## Generate a coverage report (HTML)
	go test -race -count=1 -coverprofile=$(COVER_FILE) ./...
	go tool cover -func=$(COVER_FILE) | tail -1
	go tool cover -html=$(COVER_FILE) -o coverage.html
	@echo "Coverage report: $(PWD)/coverage.html"

# --- Docker -------------------------------------------------------------------
.PHONY: docker-build
docker-build: ## Build the production docker image
	docker build \
		--target final \
		--build-arg GIT_SHA=$(GIT_SHA) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg VERSION=$(VERSION) \
		-t $(IMAGE):$(TAG) \
		.

.PHONY: docker-run
docker-run: ## Run the production docker image locally (requires DATABASE_URL env)
	docker run --rm -it \
		-p 8080:8080 \
		-e APP_ENV=production \
		-e PORT=8080 \
		-e DATABASE_URL \
		$(IMAGE):$(TAG)

# --- Compose (dev) ------------------------------------------------------------
.PHONY: compose-up
compose-up: ## Bring up dev stack (app + pg16) with hot reload
	$(COMPOSE) up --build -d
	@echo "App:  http://localhost:8080/health"
	@echo "DB:   postgres://postgres:postgres@localhost:5432/postgres"

.PHONY: compose-down
compose-down: ## Tear down dev stack (preserves named volumes)
	$(COMPOSE) down

.PHONY: compose-down-volumes
compose-down-volumes: ## Tear down dev stack AND delete named volumes (destructive)
	$(COMPOSE) down -v

.PHONY: compose-logs
compose-logs: ## Tail logs from the dev stack
	$(COMPOSE) logs -f --tail=200

# --- Compose (prod-like) ------------------------------------------------------
.PHONY: compose-prod-up
compose-prod-up: ## Bring up prod-like stack (distroless image) — requires DATABASE_URL
	$(COMPOSE_PROD) up --build -d

.PHONY: compose-prod-down
compose-prod-down: ## Tear down prod-like stack
	$(COMPOSE_PROD) down

# --- Migrations (optional — app runs goose on boot) ---------------------------
.PHONY: migrate-up
migrate-up: ## Run goose migrations up against $(DATABASE_URL)
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir ./migrations postgres "$$DATABASE_URL" up

.PHONY: migrate-down
migrate-down: ## Roll back the last goose migration against $(DATABASE_URL)
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir ./migrations postgres "$$DATABASE_URL" down

# --- Housekeeping -------------------------------------------------------------
.PHONY: clean
clean: ## Remove build artefacts and coverage output
	rm -rf $(BIN_DIR) tmp $(COVER_FILE) coverage.html
