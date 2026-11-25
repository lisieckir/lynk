# Makefile plan

Status: Phase 0 design; `Makefile` is intentionally not created in this phase.

The Makefile is the central, documented project interface. Commands run from
the repository root, use variables rather than duplicated strings, fail fast,
and never remove persistent application data during ordinary cleanup.

## Variables

Proposed defaults:

```make
APP            := lynk
VERSION        ?= dev
DOCKER_IMAGE   ?= ghcr.io/lisieckir/lynk
DOCKER_TAG     ?= $(VERSION)
COMPOSE        ?= docker compose
GO             ?= go
GOFLAGS        ?=
CGO_ENABLED    ?= 0
BIN_DIR        ?= bin
DIST_DIR       ?= dist
DATA_DIR       ?= .data
DATABASE_PATH  ?= $(DATA_DIR)/lynk.db
BACKUP_DIR     ?= $(DATA_DIR)/backups
LDFLAGS        := -X main.version=$(VERSION)
```

`DOCKER_IMAGE`, `DOCKER_TAG`, `COMPOSE`, `VERSION`, database paths and tools
must be overridable from the command line/environment.

## Public targets

Every public target gets a `## description` comment and appears in `make help`.
The default target is `help`.

### Development

| Target | Planned behavior |
|---|---|
| `help` | Parse/display public target descriptions |
| `init` | Create local non-production directories, verify Go/Docker tools and run safe initialization |
| `deps` | `go mod download` and `go mod verify` |
| `fmt` | `gofmt` over Go sources |
| `vet` | `go vet ./...` |
| `lint` | Run the pinned/configured Go linter; fail if unavailable rather than silently skipping |
| `run` | Build/run locally with configurable environment and data path |
| `dev` | Run a development workflow using Go tooling only; file watching is optional, not Node-based |

### Testing

| Target | Planned behavior |
|---|---|
| `test` | `go test ./...` |
| `test-race` | `go test -race ./...` |
| `test-cover` | Write coverage profile/report under generated output |
| `test-integration` | Run integration tests, with explicit opt-in if Docker/network is needed |
| `test-compat` | Run `internal/api/linkding/` compatibility tests |
| `benchmark` | `go test -bench=. -benchmem ./...` for benchmark packages |

`test-compat` must include extension-shaped requests and assert status,
headers, JSON fields, pagination and duplicate-POST behavior.

### Building and release

| Target | Planned behavior |
|---|---|
| `build` | Build local binary with `VERSION` linker metadata |
| `build-linux` | Build Linux AMD64 binary using the selected SQLite/CGO policy |
| `clean` | Remove `bin/`, `dist/` and generated coverage only |
| `version` | Print version/build metadata without requiring a database |
| `check` | `fmt`, `vet`, `lint`, `test` in a fail-fast sequence |
| `release` | All checks plus production binary build |

### Docker

| Target | Planned behavior |
|---|---|
| `docker-build` | Build `$(DOCKER_IMAGE):$(DOCKER_TAG)` |
| `docker-up` | Compose build/start |
| `docker-down` | Compose stop/down without volume deletion |
| `docker-restart` | Restart the Lynk service |
| `docker-logs` | Follow service logs |
| `docker-ps` | Show service status |
| `docker-shell` | Open a shell when runtime supports it; otherwise print a debugging alternative |
| `docker-pull` | Pull configured image |
| `docker-clean` | Remove project containers/images only; never volumes/data |
| `docker-release` | Run release checks and production image validation/build |

`docker-down` must not use `--volumes`. `docker-clean` must not use broad
prune commands.

### Database and maintenance

| Target | Planned behavior |
|---|---|
| `db-info` | Report path, migration version, SQLite version, journal mode and integrity status without secrets |
| `db-migrate` | Apply pending migrations safely |
| `db-backup` | Use the application/SQLite backup mechanism; write timestamped backup to configurable directory and verify it |
| `db-restore` | Require an explicit confirmation variable/flag, validate input and preserve old database until replacement succeeds |
| `health` | Query `/health` at the configured address and fail on non-2xx |

Backups must include WAL state through SQLite's backup API or an equivalent
consistent mechanism, never a default `cp database.db backup.db`.

## Implementation conventions

Use `.PHONY` for all non-file targets. Define reusable command variables
(`GO`, `COMPOSE`, image/tag, build flags), use shell strictness where
appropriate, and print concise `==> ...` status messages. A missing optional
tool must be a visible failure for targets that require it.

The Makefile should not install tools implicitly into the host or mutate
production data. `init` may create local directories but must not create or
reset a production database.

## Decisions still requiring confirmation

1. The exact linter and pinning strategy (binary, Dockerized tool, or CI-only).
2. Whether `make dev` runs a simple `go run` or an optional Go file watcher.
3. Whether `build-linux` remains `CGO_ENABLED=0` after modernc driver smoke
   tests.
4. Exact restore confirmation interface.

These are implementation details for Phase 1, not changes to `SPEC.md`.
