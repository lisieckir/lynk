# Lynk technical architecture

Status: Phase 1 foundation in progress. P1-001 through P1-004 are implemented;
Go/SQLite pins are validated on Linux AMD64.

## Goals and constraints

Lynk remains a single Go process with SQLite, server-rendered HTML and no
mandatory external service. The Linkding API adapter is a public compatibility
boundary; internal tables and domain objects are not exposed directly.

The proposed structure follows `SPEC.md`:

```text
cmd/lynk/
internal/
  api/linkding/     HTTP DTOs, routes, auth and compatibility handlers
  auth/             token lifecycle and browser session security
  bookmark/         domain rules and service operations
  tag/
  user/
  database/         SQLite opening, pragmas and migrations
  server/           minimal net/http server and graceful lifecycle
  health/           unauthenticated readiness handler
  search/           FTS5 query construction
  metadata/         bounded SSRF-safe fetcher
  worker/           bounded asynchronous jobs
  web/              HTMX handlers and templates
  config/
migrations/
web/templates/
web/static/
tests/integration/
docs/
```

### Request flow

`net/http` router -> authentication middleware -> Linkding handler -> DTO
decode/encode -> domain service -> repository/transaction -> DTO response.
The web UI uses separate handlers and templates but shares domain services.

The API package must not import HTML handlers. The domain package must not know
about Linkding DTO names. This prevents compatibility fields such as
`website_title: null` from leaking into the database model.

## Components

### HTTP server

Use `net/http`, explicit routes and request contexts. Configure read-header,
read, write and idle timeouts. Serve `/health` without API authentication and
API routes with token authentication. Preserve trailing slash routes as
canonical.

P1-004 registers `GET /health` on a standard `http.ServeMux` through
`internal/server`. The handler in `internal/health` pings the database under a
bounded context and returns only fixed 200/503 JSON bodies; checker errors,
paths and credentials are never serialized. The readiness timeout is
`LYNK_HEALTH_TIMEOUT` in typed configuration (positive Go duration, default
`2s`) and flows through `server.Config.ReadinessTimeout`. Only the exact path
`/health` is canonical: `/health/` is intentionally unregistered and returns a
deterministic 404 without redirect. `cmd/lynk` loads configuration, opens and
migrates the database, then runs the server until context cancellation,
draining requests before closing the database during graceful shutdown. Shutdown
errors and database-close errors are joined and reported together.

### Authentication

Parse `Authorization` case-insensitively and accept exactly the `Token` and
`Bearer` schemes. Use a cryptographically random 40-hex-character token for
wire compatibility, store a SHA-256/HMAC digest plus a short identifying
prefix if operational lookup requires it, and support revocation through a
database row. Never log the raw token or authorization header.

The API token identifies a user; all bookmark/tag queries are scoped to that
user. Browser session authentication is a separate mechanism and requires
CSRF protection.

### Domain and persistence

A bookmark service owns duplicate semantics, tag replacement, archive state and
modified timestamps. The repository uses prepared statements and transactions.
One transaction must cover bookmark/tag relation changes and FTS synchronization.

Use a bounded `database/sql` pool. SQLite still serializes writers, so avoid
large write transactions and configure a busy timeout.

### Search

Use an external-content FTS5 table and explicit triggers or transaction-level
synchronization. Index URL, title, description, notes and a denormalized tag
text column. Search results join back to bookmarks and apply owner/archive
filters outside or alongside the FTS match.

The first implementation should use SQLite FTS5 SQL directly rather than add a
search framework. Query parsing must constrain user input and avoid treating
untrusted syntax as raw SQL.

### Metadata and workers

Bookmark creation returns without waiting for metadata. A bounded worker pool
may fetch metadata after creation. The API `/check/` path needs a bounded
response compatible with the extension; it must never bypass SSRF controls.

The metadata client must enforce HTTP(S)-only URLs, DNS/address checks,
redirect limits, response-size limits and timeouts as required by `SPEC.md`.

### Web UI

Use Go `html/template`, HTMX and plain CSS. Embed or package assets in the
binary where practical. No Node.js runtime is needed for application or
development.

## SQLite driver decision (validated)

**Proposal: use `modernc.org/sqlite`.**

P1-001 pins Go `1.27.4` and `modernc.org/sqlite` `v1.63.0`. P1-003 validated
these pins on Linux AMD64 with FTS5, WAL, foreign keys, busy timeout and the
SQLite online backup API. Exact details are recorded in
[development.md](development.md). Cross-platform validation remains a release
follow-up.

Reasons:

* pure Go, avoiding CGO in the default build;
* works with `database/sql`;
* current upstream documentation/build reports FTS5 enabled;
* supports SQLite online backup APIs through driver connection interfaces;
* suitable for a small static Linux binary and Docker image.

Risks:

* the dependency is larger than a thin CGO wrapper;
* driver-specific backup interfaces must be isolated behind `internal/database`;
* the exact supported Go/SQLite versions are validated on Linux AMD64 and
  should be rechecked for other release targets;
* FTS5 and WAL must be tested on the target build, not assumed from online
  documentation.

Phase 1 must run a smoke test for `PRAGMA compile_options`, `CREATE VIRTUAL
 TABLE ... USING fts5`, WAL mode, foreign keys and backup/restore. These smoke
 tests passed for the current Linux AMD64 target; no change to `SPEC.md` is
 proposed.

## Docker architecture

Use a two-stage Dockerfile:

1. pinned Go builder image, `go mod download`, tests and static/low-dependency
   Linux build;
2. minimal non-root runtime with CA certificates, `/data` as a volume, and
   the Lynk binary.

The runtime exposes the configured HTTP port (default `9090`), has a
container healthcheck against `/health`, and contains no compiler or package
manager. Compose defines exactly one service, one persistent data volume,
restart policy, environment configuration and healthcheck.

The application creates its data directory/database and applies migrations
without overwriting existing data.

## Backup and recovery

Do not copy only the main SQLite file while the process is live with WAL
enabled. Implement a database-aware backup behind `db-backup`, preferably the
SQLite online backup API exposed by the selected driver, or `VACUUM INTO` as a
carefully tested fallback. Verify the produced backup with a separate open and
`PRAGMA integrity_check`.

Restore must require an explicit confirmation variable/argument, stop or
quiesce application writes, validate the backup and preserve the original
until the replacement is accepted.

## Observability and security

Use `log/slog` with request IDs and durations, but redact authorization headers,
tokens, bookmark URLs if policy later requires it, and metadata fetch errors
that could contain secrets. Add structured health information without
revealing credentials or filesystem secrets.

Keep pprof development-only. Apply security headers, request body limits,
CSRF/session controls and rate limiting to the relevant surfaces.

## Required dependencies

### Production Go

* `modernc.org/sqlite` v1.63.0 — SQLite driver and FTS5-capable engine,
  validated on Linux AMD64 in P1-003.
* no web framework; use standard `net/http`, `database/sql`,
  `html/template`, `log/slog`, `crypto`, `embed` and `context`.

### Development/validation

* Go toolchain (`gofmt`, `go test`, `go vet`, `go tool cover`);
* a linter selected in the Makefile plan (default proposal: `golangci-lint`,
  pinned or installed explicitly, not required at runtime);
* Docker and Compose plugin for container checks.

No JavaScript package manager is required.

## Unresolved technical decisions

1. Cross-platform confirmation of Go 1.27.4 and modernc.org/sqlite v1.63.0
   before release targets beyond Linux AMD64.
2. Whether any future backup mode should complement the selected SQLite online
   backup API; the current primary implementation is settled.
3. Whether token lookup uses a keyed digest (HMAC) or SHA-256 plus a unique
   prefix. The wire format and revocation requirement are fixed; storage
   detail is still open.
4. Whether FTS synchronization uses triggers or service-owned writes. Triggers
   provide stronger consistency; explicit service writes are easier to reason
   about in migrations.
5. Exact metadata queue behavior on shutdown and retry policy.
6. Whether API compatibility needs bundles/assets in the initial compatible
   release or only the endpoint families required by `SPEC.md`.

These are proposals/open decisions, not silent changes to the specification.
