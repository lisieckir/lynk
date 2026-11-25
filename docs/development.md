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
after shutdown or serve errors. API and Linkding compatibility tests are
deferred to later tasks.
