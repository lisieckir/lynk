# ---------------------------------------------------------------------------
# Lynk — Makefile: the project-root interface (SPEC 9 in docs/SPEC.md).
#
# - `make help` is the default target and lists every public target.
# - All non-file targets are .PHONY; commands use variables, not duplicated
#   strings, and print readable `==> ...` status lines.
# - Fail-fast: the first failing command stops the target.
# - Persistent data (database, backups, compose volumes) is never deleted by
#   clean targets — only generated artifacts (bin/, dist/, coverage files).
# ---------------------------------------------------------------------------

APP                 := lynk
VERSION             ?= dev
DOCKER_IMAGE        ?= ghcr.io/lisieckir/lynk
DOCKER_TAG          ?= $(VERSION)
DOCKER_SERVICE      ?= lynk
COMPOSE             ?= docker compose
GO                  ?= go
GOFLAGS             ?=
CGO_ENABLED         ?= 0
LINT_TOOL           ?= golangci-lint
BIN_DIR             ?= bin
DIST_DIR            ?= dist
DATA_DIR            ?= .data
DATABASE_PATH       ?= $(DATA_DIR)/lynk.db
BACKUP_DIR          ?= $(DATA_DIR)/backups
COVER_PROFILE       ?= $(DIST_DIR)/coverage.txt
COVER_HTML          ?= $(DIST_DIR)/coverage.html
LYNK_ENV            ?= development
LYNK_HTTP_ADDR      ?= :9090
LYNK_DATABASE_PATH  ?= $(DATABASE_PATH)
LYNK_LOG_LEVEL      ?= info
LYNK_BASE_URL       ?= http://localhost:9090
HEALTH_TIMEOUT      ?= 5
DEV_ENV             ?= development
DB_RESTORE_FROM     ?=
DB_RESTORE_CONFIRM  ?=
DB_STOP_HOOK        ?=
LDFLAGS             := -X main.version=$(VERSION)
GIT_COMMIT          ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)

.DEFAULT_GOAL := help

# ---------------------------------------------------------------------------
# Development (SPEC 9.1)
# ---------------------------------------------------------------------------

help: ## Display this help message (default target)
	@awk 'BEGIN { printf "Lynk Makefile: project interface (default target: help)\n\nTargets:\n" } /^[a-zA-Z0-9_.-]+:.*## / { n=$$1; sub(/:$$/, "", n); d=substr($$0, index($$0, "## ")+3); printf "  %-26s %s\n", n, d }' $(MAKEFILE_LIST)
init: ## Initialize the development environment: local dirs and tool verification
	@mkdir -p "$(BIN_DIR)" "$(DIST_DIR)" "$(DATA_DIR)" "$(BACKUP_DIR)" && echo "==> init: created $(BIN_DIR), $(DIST_DIR), $(DATA_DIR), $(BACKUP_DIR)" && { command -v "$(GO)" >/dev/null 2>&1 || { echo "FAIL: $(GO) not found in PATH" >&2; exit 1; }; $(GO) version; } && { { set -- $(COMPOSE); command -v "$$1" >/dev/null 2>&1 && echo "==> init: $(COMPOSE) available" || echo "==> init: $(COMPOSE) not found yet; docker targets fail until Docker is installed"; } }

deps: ## Download and verify Go dependencies (go mod download && go mod verify)
	@echo "==> deps: downloading modules"
	@$(GO) mod download
	@echo "==> deps: verifying modules"
	@$(GO) mod verify

fmt: ## Format Go sources (go fmt ./...)
	@echo "==> fmt: formatting Go sources"
	@$(GO) fmt ./...

vet: ## Run go vet ./...
	@echo "==> vet: running go vet ./..."
	@$(GO) vet ./...

lint: ## Run static analysis with the configured Go linter (default: golangci-lint)
	@if [ -z "$(LINT_TOOL)" ]; then echo "FAIL: lint: LINT_TOOL is empty; set LINT_TOOL to a linter name or 'none'" >&2; exit 1; fi; if [ "$(LINT_TOOL)" = "none" ]; then echo "==> lint: disabled (LINT_TOOL=none)"; exit 0; fi; command -v "$(LINT_TOOL)" >/dev/null 2>&1 || { echo "FAIL: $(LINT_TOOL) not found in PATH" >&2; exit 1; }; echo "==> lint: running $(LINT_TOOL)"; $(LINT_TOOL) run ./...

run: ## Build and run lynk locally with development defaults (port 9090)
	@$(MAKE) build
	@echo "==> run: starting $(BIN_DIR)/$(APP) (LYNK_ENV=$(LYNK_ENV), LYNK_DATA_DIR=$(DATA_DIR), LYNK_HTTP_ADDR=$(LYNK_HTTP_ADDR), LYNK_DATABASE_PATH=$(LYNK_DATABASE_PATH))"
	@LYNK_ENV="$(LYNK_ENV)" LYNK_DATA_DIR="$(DATA_DIR)" LYNK_HTTP_ADDR="$(LYNK_HTTP_ADDR)" LYNK_DATABASE_PATH="$(LYNK_DATABASE_PATH)" LYNK_LOG_LEVEL="$(LYNK_LOG_LEVEL)" LYNK_BASE_URL="$(LYNK_BASE_URL)" "$(BIN_DIR)/$(APP)"

dev: ## Start the development workflow: build and run with the development environment
	@echo "==> dev: starting development workflow (DEV_ENV=$(DEV_ENV))"
	@LYNK_ENV="$(DEV_ENV)" $(MAKE) run

# ---------------------------------------------------------------------------
# Testing (SPEC 9.2)
# ---------------------------------------------------------------------------

test: ## Run unit tests (go test ./...)
	@echo "==> test: running go test ./..."
	@$(GO) test ./...

test-race: ## Run tests with the Go race detector (go test -race ./...)
	@echo "==> test-race: running go test -race ./..."
	@$(GO) test -race ./...

test-cover: ## Generate a coverage profile in the generated output dir
	@mkdir -p "$(DIST_DIR)"
	@echo "==> test-cover: generating coverage profile $(COVER_PROFILE)"
	@$(GO) test -coverprofile="$(COVER_PROFILE)" ./...

test-integration: ## Run integration tests in tests/integration (when present)
	@if [ -d tests/integration ]; then \
		echo "==> test-integration: running $(GO) test ./tests/integration/..."; \
		$(GO) test ./tests/integration/...; \
	else \
		echo "==> test-integration: tests/integration is not present yet; nothing to run" >&2; \
	fi

test-compat: ## Run Linkding API compatibility tests in internal/api/linkding (when present)
	@if [ -d internal/api/linkding ]; then \
		echo "==> test-compat: running $(GO) test ./internal/api/linkding/..."; \
		$(GO) test ./internal/api/linkding/...; \
	else \
		echo "==> test-compat: internal/api/linkding is not present yet (Phase 3); nothing to run" >&2; \
	fi

benchmark: ## Run Go benchmarks (go test -bench=. -benchmem ./...)
	@echo "==> benchmark: running $(GO) test -bench=. -benchmem ./..."
	@$(GO) test -bench=. -benchmem ./...

# ---------------------------------------------------------------------------
# Building and release (SPEC 9.3)
# ---------------------------------------------------------------------------

build: ## Build the local binary bin/lynk with VERSION linker metadata
	@echo "==> build: compiling $(APP)"
	@$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o "$(BIN_DIR)/$(APP)" ./cmd/lynk
	@echo "==> build: wrote $(BIN_DIR)/$(APP)"

build-linux: ## Build the Linux/amd64 binary dist/lynk-linux-amd64 (CGO_ENABLED=0)
	@echo "==> build-linux: compiling $(APP) for linux/amd64"
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o "$(DIST_DIR)/$(APP)-linux-amd64" ./cmd/lynk
	@echo "==> build-linux: wrote $(DIST_DIR)/$(APP)-linux-amd64"

clean: ## Remove generated artifacts only: bin/, dist/, coverage files (data preserved)
	@for d in "$(BIN_DIR)" "$(DIST_DIR)"; do \
		case "$$d" in ""|"."|".."|"/") echo "==> clean: skipping empty/unsafe path '$$d'" >&2; continue;; esac; \
		rm -rf "$$d" && echo "==> clean: removed '$$d'"; \
	done
	@rm -f "$(COVER_PROFILE)" "$(COVER_HTML)"
	@echo "==> clean: done; data in $(DATA_DIR) untouched"

version: ## Print version and build metadata (no database required)
	@echo "app:     $(APP)"
	@echo "version: $(VERSION)"
	@echo "commit:  $(GIT_COMMIT)"
	@echo "go:      $(shell $(GO) version 2>/dev/null)"
	@echo "cgo:     $(CGO_ENABLED)"

# ---------------------------------------------------------------------------
# Docker (SPEC 9.4) — Dockerfile/compose.yaml are Phase 1 deliverables, so
# these targets fail clearly while the files are missing.
# ---------------------------------------------------------------------------

docker-build: ## Build image ghcr.io/lisieckir/lynk:dev
	@command -v docker >/dev/null 2>&1 || { echo "FAIL: docker not found in PATH" >&2; exit 1; }
	@[ -f Dockerfile ] || { echo "FAIL: Dockerfile not found (delivered by Phase 1)" >&2; exit 1; }
	@echo "==> docker-build: building $(DOCKER_IMAGE):$(DOCKER_TAG)"
	@docker build -t "$(DOCKER_IMAGE):$(DOCKER_TAG)" .

docker-up: ## Compose build and start the lynk service
	@command -v docker >/dev/null 2>&1 || { echo "FAIL: docker not found in PATH" >&2; exit 1; }
	@[ -f compose.yaml ] || { echo "FAIL: compose.yaml not found (delivered by Phase 1)" >&2; exit 1; }
	@echo "==> docker-up: starting $(COMPOSE) up -d --build"
	@$(COMPOSE) up -d --build

docker-down: ## Compose stop without deleting persistent data (no --volumes)
	@command -v docker >/dev/null 2>&1 || { echo "FAIL: docker not found in PATH" >&2; exit 1; }
	@[ -f compose.yaml ] || { echo "FAIL: compose.yaml not found; nothing to stop yet" >&2; exit 1; }
	@echo "==> docker-down: stopping $(COMPOSE) down (volumes/data preserved)"
	@$(COMPOSE) down

docker-restart: ## Restart the lynk service
	@command -v docker >/dev/null 2>&1 || { echo "FAIL: docker not found in PATH" >&2; exit 1; }
	@[ -f compose.yaml ] || { echo "FAIL: compose.yaml not found" >&2; exit 1; }
	@echo "==> docker-restart: restarting $(COMPOSE) restart"
	@$(COMPOSE) restart

docker-logs: ## Follow application logs (docker compose logs --follow)
	@command -v docker >/dev/null 2>&1 || { echo "FAIL: docker not found in PATH" >&2; exit 1; }
	@[ -f compose.yaml ] || { echo "FAIL: compose.yaml not found" >&2; exit 1; }
	@$(COMPOSE) logs --follow

docker-ps: ## Show container status (docker compose ps)
	@command -v docker >/dev/null 2>&1 || { echo "FAIL: docker not found in PATH" >&2; exit 1; }
	@[ -f compose.yaml ] || { echo "FAIL: compose.yaml not found" >&2; exit 1; }
	@$(COMPOSE) ps

docker-shell: ## Open an interactive shell in the lynk container when the runtime supports it
	@command -v docker >/dev/null 2>&1 || { echo "FAIL: docker not found in PATH" >&2; exit 1; }
	@[ -f compose.yaml ] || { echo "FAIL: compose.yaml not found" >&2; exit 1; }
	@for s in sh ash bash; do \
		if $(COMPOSE) exec "$(DOCKER_SERVICE)" "$$s" -c 'true' >/dev/null 2>&1; then \
			echo "==> docker-shell: opening $$s shell"; \
			$(COMPOSE) exec -it "$(DOCKER_SERVICE)" "$$s"; \
			exit 0; \
		fi; \
	done; \
	echo "FAIL: docker-shell: no interactive shell available on $(DOCKER_SERVICE); fallback: docker exec -it \$$(docker ps -qf name=$(DOCKER_SERVICE)) sh" >&2; \
	exit 1

docker-pull: ## Pull the configured image ghcr.io/lisieckir/lynk:dev
	@command -v docker >/dev/null 2>&1 || { echo "FAIL: docker not found in PATH" >&2; exit 1; }
	@echo "==> docker-pull: pulling $(DOCKER_IMAGE):$(DOCKER_TAG)"
	@docker pull "$(DOCKER_IMAGE):$(DOCKER_TAG)"

docker-clean: ## Remove project containers/images only; volumes and data are preserved
	@command -v docker >/dev/null 2>&1 || { echo "FAIL: docker not found in PATH" >&2; exit 1; }
	@[ -f compose.yaml ] || { echo "FAIL: compose.yaml not found; nothing to clean" >&2; exit 1; }
	@echo "==> docker-clean: removing project containers/images (volumes/data preserved)"
	@$(COMPOSE) down --rmi local

# ---------------------------------------------------------------------------
# Database and maintenance (SPEC 9.5)
# ---------------------------------------------------------------------------

db-info: ## Display database path, SQLite version, migrations, journal mode and integrity
	@command -v sqlite3 >/dev/null 2>&1 || { echo "FAIL: sqlite3 not found in PATH" >&2; exit 1; }
	@if [ -f "$(DATABASE_PATH)" ]; then \
		echo "==> db-info: path: $(DATABASE_PATH)"; \
		echo "==> db-info: sqlite version: $$(sqlite3 -readonly "$(DATABASE_PATH)" 'SELECT sqlite_version();' 2>/dev/null)"; \
		echo "==> db-info: migrations applied (max version): $$(sqlite3 -readonly "$(DATABASE_PATH)" 'SELECT COALESCE(MAX(version),0) FROM schema_migrations;' 2>/dev/null || echo 0)"; \
		echo "==> db-info: journal mode: $$(sqlite3 -readonly "$(DATABASE_PATH)" 'PRAGMA journal_mode;' 2>/dev/null || echo n/a)"; \
		echo "==> db-info: integrity_check: $$(sqlite3 -readonly "$(DATABASE_PATH)" 'PRAGMA integrity_check;' 2>/dev/null)"; \
	else \
		echo "==> db-info: $(DATABASE_PATH) does not exist yet; run the app to create it"; \
	fi

db-migrate: ## Apply pending migrations (fails while the lynk entrypoint is unwired)
	@echo "==> db-migrate: database $(DATABASE_PATH)"
	@echo "FAIL: db-migrate is not wired to the lynk entrypoint yet; internal/database.Migrate applies pending migrations when the app opens the database — start the app with: make run" >&2
	@exit 1

db-backup: ## Create a timestamped consistent backup in .data/backups using the SQLite online backup API and verify it
	@command -v sqlite3 >/dev/null 2>&1 || { echo "FAIL: sqlite3 not found in PATH" >&2; exit 1; }
	@[ -f "$(DATABASE_PATH)" ] || { echo "FAIL: $(DATABASE_PATH) not found; run the app first to create the database" >&2; exit 1; }
	@mkdir -p "$(BACKUP_DIR)" && echo "==> db-backup: writing to $(BACKUP_DIR)"
	@backup="$(BACKUP_DIR)/lynk-$(shell date +%Y%m%d-%H%M%S).db" && \
		sqlite3 "$(DATABASE_PATH)" ".backup $$backup" && \
		echo "==> db-backup: wrote $$backup" && \
		test "$$(sqlite3 "$$backup" 'PRAGMA integrity_check')" = ok && \
		echo "==> db-backup: verified backup (integrity_check = ok)"

db-restore: ## Restore .data/lynk.db from a backup; requires DB_RESTORE_FROM and DB_RESTORE_CONFIRM=yes
	@if [ -z "$(DB_RESTORE_FROM)" ]; then echo "FAIL: db-restore needs DB_RESTORE_FROM=<backup path>" >&2; exit 1; fi; \
	if [ "$(DB_RESTORE_CONFIRM)" != "yes" ]; then echo "FAIL: db-restore requires explicit confirmation: DB_RESTORE_CONFIRM=yes (stop writers first — see DB_STOP_HOOK)" >&2; exit 1; fi; \
	command -v sqlite3 >/dev/null 2>&1 || { echo "FAIL: sqlite3 not found in PATH" >&2; exit 1; }; \
	[ -f "$(DB_RESTORE_FROM)" ] || { echo "FAIL: backup $(DB_RESTORE_FROM) not found" >&2; exit 1; }; \
	echo "==> db-restore: validating backup $(DB_RESTORE_FROM) with PRAGMA integrity_check"; \
	rc="$$(sqlite3 -readonly "$(DB_RESTORE_FROM)" 'PRAGMA integrity_check;' 2>/dev/null)" && [ "$$rc" = "ok" ] || { echo "FAIL: backup $(DB_RESTORE_FROM) failed integrity_check (got: $$rc)" >&2; exit 1; }; \
	if [ -f "$(DATABASE_PATH)" ]; then \
		ts="$$(date +%Y%m%d-%H%M%S)" && \
		safety="$(DATABASE_PATH).restore-safety-$$ts.db" && \
		echo "==> db-restore: creating live-db safety backup via sqlite3 .backup: $$safety" && \
		sqlite3 "$(DATABASE_PATH)" ".backup $$safety" && \
		rc2="$$(sqlite3 -readonly "$$safety" 'PRAGMA integrity_check;')" && [ "$$rc2" = "ok" ] && echo "==> db-restore: safety backup verified (integrity_check = ok)" || { echo "FAIL: safety backup failed or not ok" >&2; exit 1; }; \
	fi; \
	hook="$(DB_STOP_HOOK)"; if [ -n "$$hook" ]; then echo "==> db-restore: running DB_STOP_HOOK before replacement"; fi; hook=$${hook:-true}; $$hook || { echo "FAIL: DB_STOP_HOOK failed; refusing to replace the database while writers may be active" >&2; exit 1; }; \
	for s in "$(DATABASE_PATH)-wal" "$(DATABASE_PATH)-shm" "$(DATABASE_PATH)-journal"; do if [ -f "$$s" ]; then rm -f "$$s" && echo "==> db-restore: removed stale sidecar $$s"; fi; done; \
	echo "==> db-restore: replacing $(DATABASE_PATH) with $(DB_RESTORE_FROM)"; \
	cp "$(DB_RESTORE_FROM)" "$(DATABASE_PATH)" && echo "==> db-restore: restored $(DATABASE_PATH) from $(DB_RESTORE_FROM)"

health: ## Query /health at the configured address and fail on non-2xx
	@command -v curl >/dev/null 2>&1 || { echo "FAIL: curl not found in PATH" >&2; exit 1; }
	@addr="$(LYNK_HTTP_ADDR)" && case "$$addr" in ":"*) addr="localhost$$addr";; esac && \
		url="http://$$addr/health" && echo "==> health: GET $$url (timeout $(HEALTH_TIMEOUT)s)" && \
		curl -fsS --max-time "$(HEALTH_TIMEOUT)" "$$url" -o /dev/null && echo "==> health: ok" || \
		{ echo "FAIL: health check failed for $$url" >&2; exit 1; }

# ---------------------------------------------------------------------------
# Check and release (SPEC 9.6)
# ---------------------------------------------------------------------------

check: fmt vet lint test ## Format, vet, lint and run tests (fail-fast; prerequisites in order)

release: check build ## Run all release checks and build the production binary

docker-release: check docker-build ## Validate and build the production Docker image

# ---------------------------------------------------------------------------

.PHONY: help init deps fmt vet lint run dev \
	test test-race test-cover test-integration test-compat benchmark \
	build build-linux clean version check release \
	docker-build docker-up docker-down docker-restart docker-logs docker-ps docker-shell docker-pull docker-clean docker-release \
	db-info db-migrate db-backup db-restore health
