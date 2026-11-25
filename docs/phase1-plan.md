# Phase 1 implementation plan

Phase 1 starts only after the Phase 0 design is accepted. Tasks are ordered
to remain small and independently verifiable.

## 1. Bootstrap and toolchain

1. Select and pin the Go version and `modernc.org/sqlite` version.
2. Initialize `go.mod` with only the selected production dependency.
3. Add `.gitignore`, `.dockerignore` and a minimal command entry point.
4. Add the initial Makefile skeleton and make `make help`, `make deps`,
   `make fmt`, `make vet` and `make test` work.
5. Record the exact upstream revisions and driver versions in development
   documentation.

Acceptance: a clean checkout can download dependencies and run the empty
binary/tests through Makefile targets.

## 2. Configuration and logging

1. Define typed configuration loaded from environment with documented
   defaults:
   `LYNK_ENV`, `LYNK_HTTP_ADDR`, `LYNK_DATA_DIR`,
   `LYNK_DATABASE_PATH`, `LYNK_LOG_LEVEL`, `LYNK_BASE_URL`.
2. Validate paths, address and production-safe defaults.
3. Set up `log/slog` with structured fields and secret redaction.
4. Add context cancellation and graceful HTTP shutdown.

Acceptance: invalid configuration fails before serving; SIGTERM shuts down
cleanly; no configured secret is emitted in logs.

## 3. SQLite foundation

1. Open the database through `database/sql` and the selected driver.
2. Configure and verify foreign keys, WAL and busy timeout.
3. Set a bounded connection pool appropriate for SQLite.
4. Add a migration table and the first schema migration from
   `docs/database-schema.md`.
5. Add driver smoke tests for FTS5, WAL, foreign keys, timestamps and a
   transaction rollback.
6. Add an isolated backup proof-of-concept test using the chosen mechanism.

Acceptance: a fresh database initializes without overwriting existing data;
reopening preserves data and pragmas; FTS5 creation/query succeeds; migration
failure leaves a clear error.

## 4. Health endpoint

1. Add `GET /health`.
2. Report application readiness and database connectivity.
3. Keep the endpoint free of authentication and sensitive internals.
4. Add unit/integration tests for healthy and unavailable database states.

Acceptance: the endpoint works locally and in the eventual Compose healthcheck
with deterministic 2xx/5xx behavior.

## 5. Container and Compose foundation

1. Add a multi-stage Dockerfile with pinned builder/runtime choices.
2. Run the runtime as a non-root user with CA certificates and `/data`.
3. Add `compose.yaml` with one service, persistent volume, environment,
   restart policy, port mapping and healthcheck.
4. Add Makefile Docker targets and verify that down/clean preserve data.
5. Run the empty application in Docker and restart it against the same volume.

Acceptance: `make docker-up` starts a healthy container, data survives a
restart, and the image contains no development toolchain.

## 6. Phase 1 validation

1. Run `make check`, `make test-race` and `make docker-release`.
2. Measure binary/image size and baseline idle memory.
3. Confirm no application-feature or API behavior was implemented prematurely.
4. Update architecture/development documentation with measured results.

Phase 1 deliverable: a running empty Lynk application in Docker, with a
migrated SQLite database and health endpoint.

## Deferred to later phases

Bookmark CRUD, API DTOs/handlers, token management, FTS application queries,
metadata fetching, web UI and extension end-to-end tests are intentionally not
part of Phase 1. The compatibility matrix is the input for Phase 3 and the
real-extension validation in Phase 4.
