# Development and toolchain

Status: P1-004 health endpoint implemented. The versions below were validated
by the P1-003 smoke tests (FTS5, WAL, foreign keys, busy timeout and online
backup); no `SPEC.md` or driver-version change was made.

## Module and pinned versions

| Item | Value | Status |
|---|---|---|
| Module path | `github.com/lisieckir/lynk` | set in `go.mod` |
| Go | `1.27.4` | `go 1.27.4` in `go.mod` |
| SQLite driver | `modernc.org/sqlite v1.63.0` | unchanged pin, validated by P1-003 |

The driver is registered and used by `internal/database`, which opens it through
`database/sql`, applies pragmas on every pooled connection, runs tracked
migrations and provides the consistent backup helper. The former build-tagged
`cmd/lynk/sqlite_pin.go` pin was removed: it existed only to keep the
provisional dependency in the module graph before the real package existed.
The entrypoint now loads configuration, opens and migrates the database, and
serves `GET /health` through a bounded `net/http` server with graceful
shutdown. `LYNK_HEALTH_TIMEOUT` is the only Phase 1 addition to the SPEC
environment list: it accepts a positive Go duration, defaults to `2s`, and is
passed through `server.Config.ReadinessTimeout` to the readiness ping. API and
UI handlers remain later-phase tasks.

If a future validation fails, the version decision must be reopened explicitly;
see [architecture.md](architecture.md) and the P1-001 decision log.

## Reproducibility

Use Go 1.27.4 for formatting, testing, vetting and building. The `go` directive
in `go.mod` already declares the minimum toolchain version and is sufficient
for this bootstrap. A `toolchain` directive is intentionally **not** added: it
could make Go switch toolchains depending on `GOTOOLCHAIN` and network access,
and the validation environment uses a custom `go1.27.4-X:nodwarf5` build.
Revisit only if a supported target toolchain requires it; do not add directives
the toolchain does not support.

## Common commands

```sh
gofmt -l .
go build ./...
go test ./...
go test -race ./...
go vet ./...
go mod tidy -diff
go mod verify
```

## Tests

The P1-001 entrypoint smoke test remains in `cmd/lynk`, alongside a live
startup/health/shutdown test. P1-003 adds focused SQLite tests in
`internal/database`: fresh open and reopen without data loss, path and
permission handling, pragma verification on pooled connections, migration
ordering/idempotence/drift detection, transaction rollback and foreign keys,
FTS5 smoke and tag-trigger synchronization, and a WAL-aware online-backup proof
of concept. P1-004 adds handler tests in `internal/health` and integration
tests in `internal/server` for healthy and unavailable databases, configured
readiness timeouts, typed-nil checkers, response redaction, canonical `/health`
versus `/health/` routing, and cleanup paths that attempt database close even
after shutdown or serve errors. P2-002 adds real SQLite auth tests for token
generation/digest lookup, revocation, CLI bootstrap and HTTP middleware.
P2-003/P2-004 add the Linkding API compatibility tests in
`internal/api/linkding`, run by `make test-compat`, covering routing,
authentication, the bookmark/tag/profile/check/search/pagination contracts and
the update, archive and delete mutations.

P2-005 adds `tests/integration/`, the extension request replay suite run by
`make test-integration`. Both suites drive a real migrated SQLite database
from `internal/database` through `httptest` handlers — never a mocked store
and never a live port. The Makefile target runs `go test ./tests/integration/...`
whenever the directory exists, so the package is part of `go test ./...`,
`go test -race ./...` and `make check` as well.

## Phase 1 validation (2026-11-25)

Validated in the Linux AMD64 worktree with Go
`go1.27.4-X:nodwarf5 linux/amd64`:

| Check | Result |
|---|---|
| `gofmt -l .` | PASS; no files reported |
| `go build ./...`, `go test ./...`, `go test -race ./...`, `go vet ./...` | PASS |
| `go mod tidy -diff`, `go mod verify` | PASS; all modules verified |
| `make check` with default linter | BLOCKED/FAIL; `golangci-lint` is not installed |
| `make check` with `golangci-lint v2.15.0` | PASS; 0 issues after fixing 14 findings |
| `make LINT_TOOL=none check` | PASS with lint explicitly disabled; this does not validate lint |
| `make LINT_TOOL=none release` | PASS |
| `make LINT_TOOL=none docker-release` | PASS; image built successfully with lint explicitly disabled |
| `docker compose config`, Compose live health/restart/recreate | PASS; healthcheck and `GET /health` returned `{"status":"ok"}` |
| Dockerfile/Compose static assertions | PASS (multi-stage and digest-pinned images, non-root runtime, healthchecks, single service, persistent volume and restart policy) |

`make init`, `deps`, `fmt`, `vet`, `test`, `test-race`, `test-cover`,
`test-integration`, `test-compat`, `benchmark`, `build`, `build-linux`,
`version`, `db-info` and `db-backup` were also invoked. Integration and
compatibility targets reported no tests because their Phase 2/3 directories
are not present. `db-migrate` is not a standalone command yet; migrations run
automatically when the application opens the database. The backup target used
the installed `sqlite3` CLI's online `.backup` operation and passed
`PRAGMA integrity_check`.

### Baseline measurements

| Measurement | Result |
|---|---:|
| `bin/lynk` | 17,466,972 bytes |
| static Linux AMD64 `dist/lynk-linux-amd64` | 17,383,407 bytes |
| application RSS | 17,868 KiB (single `ps` sample immediately after `/health` became ready) |
| Docker image size | 29,371,518 bytes |
| Docker container memory | 5.328–5.375 MiB RSS in idle samples |

The live smoke returned `{"status":"ok"}`. A fresh database applied migrations
1 and 2; `PRAGMA journal_mode` returned `wal`, and `PRAGMA integrity_check`
returned `ok`. The app was stopped and reopened against the same database;
a validation row remained present. A SQLite online backup of that reopened
database retained the row and also passed `integrity_check`. The repository's
database tests additionally cover pooled-connection pragmas, migration
idempotence/order/drift, rollback, foreign keys, FTS5 and backup behavior.

### Host Docker validation

On a host with Docker Engine and the Compose plugin installed, run from the
repository root:

```sh
make LINT_TOOL=none docker-release
docker compose config
make docker-up
make health
docker compose ps
docker compose restart lynk
make health
docker compose down
```

The live run also recreated the service after `docker compose down`: a marker
and the 98,304-byte SQLite database remained in the named `lynk_lynk_data`
volume, and the container returned healthy again. `make check` without the
lint now passes without a waiver. Static Dockerfile/Compose inspection was not
used as a substitute for building and running the container.

## P2-005 validation (2026-11-26)

Validated in the Linux AMD64 worktree with Go `go1.27.4-X:nodwarf5` and the
pinned `golangci-lint v2.15.0` kept outside the repository at
`/tmp/opencode/golangci-lint-v2.15.0/golangci-lint-2.15.0-linux-amd64/golangci-lint`
(same path and version as P1-007; `git status` stayed clean of tooling).

| Check | Result |
|---|---|
| `gofmt -l .` | PASS; no files reported |
| `go vet ./...`, `go build ./...` | PASS |
| `go test ./...` | PASS; `tests/integration` runs with the rest of the module |
| `go test -race ./...` | PASS, including `tests/integration` |
| `make test-compat` | PASS; `internal/api/linkding` replay of the field-parity and tag-pagination contracts |
| `make test-integration` | PASS; runs `go test ./tests/integration/...` (4 extension replay tests) |
| `make LINT_TOOL=<pinned golangci-lint v2.15.0> check` | PASS; fmt, vet, lint (0 issues), test |
| `go mod tidy -diff`, `go mod verify` | PASS; no diff, all modules verified |

`make test-integration` no longer reports a missing directory: it executes the
`tests/integration` package whenever it exists, and a failing replay test
fails the target.
