# Development and toolchain

Status: bootstrap (P1-001). The versions below are **provisional** pending the
P1-003 SQLite validation; they are not a final architectural decision.

## Module and pinned versions

| Item | Value | Status |
|---|---|---|
| Module path | `github.com/lisieckir/lynk` | set in `go.mod` |
| Go | `1.27.4` | `go 1.27.4` in `go.mod` |
| SQLite driver | `modernc.org/sqlite v1.63.0` | pinned provisionally in `go.mod` |

The driver is kept in the module graph by `cmd/lynk/sqlite_pin.go`, which is
build-tagged `tools` so that no database driver is registered in the current
bootstrap entrypoint. P1-003 replaces that pin with the real `internal/database`
setup and must verify FTS5, WAL, pragmas and backup with these exact versions.
If validation fails, the version decision is reopened; see
[architecture.md](architecture.md) and the P1-001 decision log.

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
go vet ./...
go mod tidy -diff
go mod verify
```

## Tests

P1-001 contains only a bootstrap smoke test that the entrypoint package
compiles and its `main` returns. Domain, SQLite, HTTP and compatibility tests
are deliberately deferred to P1-003, P1-004 and later tasks.
