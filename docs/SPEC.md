# Lynk
## Project Specification & AI Development Guide

**Project:** Lynk  
**Type:** Lightweight, self-hosted bookmark manager  
**Primary language:** Go  
**Database:** SQLite  
**Frontend:** HTMX + Go HTML Templates  
**Deployment:** Docker + Docker Compose  
**Compatibility target:** Linkding API and official browser extension

---

# 1. Project Vision

Lynk is a lightweight, self-hosted bookmark manager designed as a drop-in alternative to Linkding.

The primary goal is to provide the functionality users expect from Linkding while significantly reducing resource consumption and maintaining a simple deployment architecture.

Lynk must prioritize:

1. Low RAM consumption.
2. Low CPU usage.
3. Fast startup.
4. Simple architecture.
5. Reliable data storage.
6. Full compatibility with the official Linkding browser extension.
7. Easy Docker deployment and maintenance.
8. Long-term maintainability.

Lynk should be a single-process application with SQLite as its database.

No microservices or unnecessary infrastructure.

## Core philosophy

- Simple over complex.
- Explicit over magical.
- Standard Go libraries over unnecessary dependencies.
- One binary over multiple runtime environments.
- SQLite over an external database server.
- Server-rendered HTML over a heavy SPA.
- Measurable performance over assumptions.

---

# 2. Critical Requirement: Linkding Compatibility

This is a non-negotiable requirement.

Lynk must be compatible with the official, unmodified Linkding browser extension.

The user must be able to:

1. Install the official Linkding extension.
2. Configure the Lynk server URL.
3. Enter a Lynk API token.
4. Use the extension exactly as with Linkding.
5. Add bookmarks without modifying the extension source code.

The extension must work in Chrome and Firefox.

## 2.1 API compatibility research

Before implementing the API, AI MUST inspect:

- Official Linkding source code: https://github.com/sissbruecker/linkding
- Official browser extension: https://github.com/sissbruecker/linkding-extension
- Official Linkding API documentation: https://linkding.link/api/

Do not implement API behavior based only on assumptions or third-party descriptions.

Inspect the actual source code and determine which endpoints, methods, parameters, authentication mechanisms, response formats and error codes are used by the official extension.

Create a compatibility matrix before implementation.

## 2.2 Expected API functionality

At minimum, investigate and implement the following API functionality:

- List bookmarks.
- Create bookmarks.
- Retrieve bookmark details.
- Update bookmarks.
- Delete bookmarks.
- Check whether a URL already exists.
- Archive bookmarks.
- Unarchive bookmarks.
- List tags.
- Retrieve user profile.
- Search bookmarks.
- Pagination.
- Authentication using API tokens.

Expected endpoint families include:

- `GET /api/bookmarks/`
- `POST /api/bookmarks/`
- `GET /api/bookmarks/archived/`
- `GET /api/bookmarks/check/`
- `GET /api/bookmarks/{id}/`
- `PUT/PATCH /api/bookmarks/{id}/`
- `DELETE /api/bookmarks/{id}/`
- `POST /api/bookmarks/{id}/archive/`
- `POST /api/bookmarks/{id}/unarchive/`
- `GET /api/tags/`
- `GET /api/user/profile/`

Verify every endpoint, HTTP method and response against the current Linkding implementation. The list above is a research starting point, not a substitute for verification.

## 2.3 Authentication

Implement compatible token authentication.

Investigate the official extension's exact authentication behavior and the current Linkding API contract. Support the authentication formats required by the official client; do not assume that multiple schemes are required without verification.

API tokens must be:

- Cryptographically random.
- Stored securely (preferably hashed).
- Revocable.
- Never exposed in application logs.

## 2.4 Response compatibility

Reproduce the original API contract:

- JSON field names.
- Data types.
- Required and optional fields.
- HTTP status codes.
- Error response structure.
- Pagination format.
- Trailing slash behavior.
- Query parameter semantics.

Do not expose internal Lynk database models directly.

Use dedicated API DTOs.

## 2.5 Compatibility testing

Create a dedicated test suite:

`internal/api/linkding/`

Tests must cover:

- Authentication.
- Bookmark creation.
- Bookmark listing.
- Bookmark updates.
- Bookmark deletion.
- URL duplicate handling.
- Tags.
- Pagination.
- Archived bookmarks.
- Error responses.
- Metadata handling.
- Extension-specific requests.

The final acceptance criterion is successful operation of the official, unmodified Linkding extension against Lynk.

---

# 3. Technology Stack

## Backend

- Go.
- Standard `net/http` package.
- Standard `html/template`.
- Go standard logging facilities (`log/slog`).
- Context-based request cancellation.

Use Go's standard HTTP routing wherever practical.

Avoid introducing a large web framework unless there is a demonstrated technical need.

## Database

SQLite.

Preferred initial driver:

`modernc.org/sqlite`

Evaluate its performance, FTS5 support and compatibility before finalizing.

Requirements:

- SQLite WAL mode.
- Foreign key constraints.
- Database migrations.
- Prepared statements.
- Proper transaction management.
- Bounded connection pool.
- Busy timeout.
- Reliable backup procedure.

No PostgreSQL, MySQL or Redis.

## Frontend

- HTMX.
- Go HTML Templates.
- Plain CSS.
- Minimal JavaScript.

Do not use React, Vue, Angular or a separate Node.js runtime.

The application must serve its own static assets.

## Search

SQLite FTS5.

Search should support:

- Bookmark title.
- Description.
- URL.
- Tags.

Investigate appropriate indexing and synchronization with the main bookmark table.

## Metadata extraction

Implement a lightweight metadata fetcher.

Extract:

- HTML title.
- Meta description.
- Open Graph title.
- Open Graph description.
- Favicon URL.

Metadata fetching must run asynchronously.

Never make bookmark creation dependent on successful metadata retrieval.

---

# 4. Architecture

Use a modular monolith.

One Go process, one SQLite database, one Docker container.

Suggested project structure:

```text
lynk/
├── cmd/
│   └── lynk/
│       └── main.go
├── internal/
│   ├── bookmark/
│   ├── tag/
│   ├── user/
│   ├── auth/
│   ├── metadata/
│   ├── search/
│   ├── database/
│   ├── config/
│   ├── api/
│   │   └── linkding/
│   ├── web/
│   └── worker/
├── migrations/
├── web/
│   ├── templates/
│   └── static/
├── tests/
│   └── integration/
├── docs/
│   ├── architecture.md
│   ├── api-compatibility.md
│   └── development.md
├── scripts/
├── Dockerfile
├── compose.yaml
├── Makefile
├── .dockerignore
├── .gitignore
├── .env.example
├── go.mod
└── README.md
```

Keep the architecture practical.

Do not create interfaces for every struct or introduce unnecessary abstraction layers.

Separate business logic, persistence, HTTP handlers and API compatibility adapters.

---

# 5. Functional Scope

## MVP

Implement the following features first:

- Create bookmark.
- Edit bookmark.
- Delete bookmark.
- List bookmarks.
- Bookmark details.
- Automatic metadata extraction.
- Tags.
- Search.
- Read/unread status.
- Archive/unarchive.
- Duplicate URL handling.
- API token authentication.
- Linkding API compatibility.
- Import/export in Netscape HTML format.

## V1

After MVP:

- Responsive interface.
- Dark mode.
- Bookmarklet.
- PWA support.
- REST API token management.
- Keyboard shortcuts.
- Bulk operations.
- Sorting and filtering.
- Improved search.
- Multiple tags per bookmark.

## V2

Optional future features:

- Article reader.
- Page snapshots.
- RSS feeds.
- Public/private sharing.
- Multiple user accounts.
- Browser-specific integrations.
- Automatic tagging.

Do not implement V2 features before the compatibility requirements and MVP are complete.

---

# 6. Security Requirements

Security is particularly important because Lynk will be exposed through a public VPS.

## SSRF protection

The metadata fetcher must protect against Server-Side Request Forgery.

Implement:

- HTTP and HTTPS only.
- Reject localhost.
- Reject private IP ranges.
- Reject loopback addresses.
- Reject link-local addresses.
- Reject cloud metadata addresses.
- Validate DNS resolution.
- Validate resolved addresses again after redirects.
- Limit redirect count.
- Limit response size.
- Set connection and request timeouts.
- Protect against DNS rebinding.

Do not rely solely on validating the URL string.

## Other security requirements

- Secure API token storage.
- Password hashing using an appropriate modern password hashing algorithm.
- CSRF protection for browser sessions.
- Secure session cookies.
- Security headers.
- CORS allow-lists must be explicit: no wildcard origin, no credentialed
  wildcard, and preflight restricted to configured methods and headers.
  `Authorization` is granted only when explicitly configured.
- Input validation.
- SQL injection protection.
- No secrets in logs.
- Non-root Docker execution.
- Minimal Docker image.
- Configurable rate limiting.

The web UI operational controls are configured with
`LYNK_LOGIN_RATE_LIMIT_MAX`, `LYNK_LOGIN_RATE_LIMIT_WINDOW`,
`LYNK_WEB_MAX_SESSIONS` and `LYNK_TRUST_PROXY_HEADERS`. Safe defaults reject
untrusted forwarding headers and bound both login attempts and in-memory
sessions.

---

# 7. Performance Requirements

Low resource consumption is a primary product objective.

Initial performance targets:

| Metric | Target |
|---|---:|
| Idle RAM | 10–20 MB |
| Normal RAM | 20–40 MB |
| Metadata fetching | Below 64 MB |
| External services | 0 |
| Database | SQLite |
| Application processes | 1 |

These are engineering targets, not guaranteed measurements.

Measure actual memory consumption throughout development.

Use:

- `runtime.ReadMemStats`.
- `net/http/pprof` in development.
- Docker container statistics.
- Go benchmarks.
- Load testing.

Do not expose pprof publicly in production.

## Memory optimization

- Limit worker pool size.
- Avoid unbounded goroutines.
- Limit HTTP response body size.
- Avoid unnecessary caching.
- Avoid loading large page contents into memory.
- Configure SQLite cache deliberately.
- Use streaming where appropriate.
- Configure HTTP server timeouts.

Never sacrifice security or correctness solely to reduce a few megabytes of RAM.

---

# 8. Docker and Deployment

Docker is the target production deployment method.

The application should build into a small, self-contained image.

## Dockerfile

Use a multi-stage build:

1. Go build stage.
2. Minimal runtime stage.

Requirements:

- Reproducible build.
- Static binary where practical.
- Non-root runtime user.
- CA certificates for HTTPS metadata fetching.
- No development tools in production.
- Embedded or copied static assets.
- Healthcheck support.

Evaluate whether a minimal distroless or Alpine runtime is most appropriate.

## Docker Compose

Provide a simple `compose.yaml`.

Requirements:

- One Lynk service.
- Persistent data volume.
- Configurable HTTP port.
- Configurable environment.
- Restart policy.
- Healthcheck.
- No unnecessary services.

Suggested environment variables:

```env
LYNK_ENV=production
LYNK_HTTP_ADDR=:9090
LYNK_DATA_DIR=/data
LYNK_DATABASE_PATH=/data/lynk.db
LYNK_LOG_LEVEL=info
LYNK_BASE_URL=https://lynk.example.com
# Browser CORS is opt-in: no origins are allowed by default.
LYNK_CORS_ALLOWED_ORIGINS=
LYNK_CORS_ALLOWED_METHODS=GET,POST
LYNK_CORS_ALLOWED_HEADERS=Content-Type
LYNK_CORS_MAX_AGE=600
```

The application must be able to initialize its database automatically.

Existing data must never be overwritten during normal startup.

---

# 9. Makefile – Central Project Interface

The Makefile is a mandatory part of the project.

All common development, testing, building, Docker and maintenance operations must be available through Makefile targets.

The user should not need to remember long Go or Docker commands.

Run `make help` to see all available commands.

## 9.1 Development

```bash
make help
make init
make deps
make fmt
make vet
make lint
make run
make dev
```

Expected behavior:

- `help`: Display available targets.
- `init`: Initialize the development environment.
- `deps`: Download and verify Go dependencies.
- `fmt`: Format Go code.
- `vet`: Run go vet.
- `lint`: Run static analysis.
- `run`: Run application locally.
- `dev`: Start development workflow.

Do not introduce a Node.js runtime for development.

If file watching is required, use a lightweight Go-based tool or provide an optional development-only tool.

## 9.2 Testing

```bash
make test
make test-race
make test-cover
make test-integration
make test-compat
make benchmark
```

Expected behavior:

- `test`: Run unit tests.
- `test-race`: Run Go race detector.
- `test-cover`: Generate coverage report.
- `test-integration`: Run integration tests.
- `test-compat`: Run Linkding API compatibility tests.
- `benchmark`: Run performance benchmarks.

## 9.3 Building

```bash
make build
make build-linux
make clean
make version
```

Expected behavior:

- `build`: Build local binary.
- `build-linux`: Build Linux AMD64 binary.
- `clean`: Remove generated artifacts.
- `version`: Display application version and build information.

Use linker flags for version and build metadata.

## 9.4 Docker

```bash
make docker-build
make docker-up
make docker-down
make docker-restart
make docker-logs
make docker-ps
make docker-shell
make docker-pull
make docker-clean
```

Expected behavior:

- `docker-build`: Build image.
- `docker-up`: Build and start the application.
- `docker-down`: Stop containers without deleting persistent data.
- `docker-restart`: Restart application.
- `docker-logs`: Follow application logs.
- `docker-ps`: Show container status.
- `docker-shell`: Open a shell if the runtime image supports it; otherwise provide a suitable debugging alternative.
- `docker-pull`: Pull configured image.
- `docker-clean`: Remove generated containers and images without deleting persistent data.

## 9.5 Database and maintenance

```bash
make db-info
make db-migrate
make db-backup
make db-restore
make health
```

Expected behavior:

- `db-info`: Display database information.
- `db-migrate`: Apply pending migrations.
- `db-backup`: Create a consistent SQLite backup.
- `db-restore`: Restore from a specified backup after explicit confirmation.
- `health`: Check application health.

Backups must be safe with SQLite WAL enabled.

Never use a simple live copy of the database file as the default backup strategy.

## 9.6 Release and validation

```bash
make check
make release
make docker-release
```

Expected behavior:

- `check`: Format, vet, lint and run tests.
- `release`: Run all release checks and build production binary.
- `docker-release`: Validate and build production Docker image.

## 9.7 Makefile implementation requirements

- Use `.PHONY` for non-file targets.
- Provide sensible default target.
- Include a configurable `VERSION`.
- Include configurable `DOCKER_IMAGE`.
- Include configurable `DOCKER_TAG`.
- Include configurable `COMPOSE`.
- Use variables rather than duplicating command strings.
- Fail immediately when a command fails.
- Print readable status messages.
- Never delete persistent application data during ordinary cleanup.
- Provide help descriptions for every public target.
- Make commands work from the project root.

The Makefile should be documented and maintained alongside the application.

---

# 10. Database and Backup Strategy

SQLite must use WAL mode.

Provide migrations from the beginning.

Database schema should support:

- Users.
- API tokens.
- Bookmarks.
- Tags.
- Bookmark/tag relationships.
- Search index.

Create appropriate indexes for common queries.

Implement a reliable backup strategy using SQLite's backup functionality or an equivalent consistent backup mechanism.

The backup must account for WAL files.

Backups should be written to a configurable directory.

---

# 11. Development Workflow for AI

AI coding agents must follow these rules.

## General rules

1. Read this specification before implementing anything.
2. Inspect the existing project before making changes.
3. Do not generate the entire application in one response.
4. Work in small, independently verifiable stages.
5. Write tests alongside implementation.
6. Run relevant tests after every meaningful change.
7. Update documentation when architecture changes.
8. Never silently change architectural decisions.
9. Avoid unnecessary dependencies.
10. Explain significant design decisions.

## Go-specific rules

- Follow standard Go conventions.
- Prefer clear, idiomatic code.
- Handle errors explicitly.
- Propagate contexts.
- Avoid global mutable state.
- Avoid unnecessary interfaces.
- Keep functions reasonably small.
- Use structured logging.
- Write table-driven tests where appropriate.
- Use dependency injection only where it provides a clear benefit.

## Dependency policy

Before adding a dependency:

1. Explain why it is needed.
2. Evaluate the standard library alternative.
3. Evaluate maintenance and security.
4. Consider its memory and binary-size impact.

Do not introduce dependencies just for convenience.

---

# 12. Implementation Phases

AI must implement Lynk in the following order.

## Phase 0 – Research and technical design

- Inspect Linkding API.
- Inspect official browser extension.
- Document all extension API calls.
- Create compatibility matrix.
- Evaluate SQLite driver.
- Verify FTS5 support.
- Define database schema.
- Define Docker architecture.
- Define Makefile structure.

Deliverable: technical design document.

Do not start implementing application features before completing this phase.

## Phase 1 – Project foundation

- Initialize Go module.
- Create directory structure.
- Create configuration system.
- Set up logging.
- Set up SQLite.
- Implement migrations.
- Implement health endpoint.
- Create Makefile.
- Create Dockerfile.
- Create compose.yaml.

Deliverable: a running empty Lynk application in Docker.

## Phase 2 – Core domain

- Bookmark model.
- Repository.
- Service layer.
- Tag model.
- Database operations.
- Unit and integration tests.

Deliverable: working bookmark domain.

## Phase 3 – Linkding API

- Authentication.
- API DTOs.
- Endpoint handlers.
- Pagination.
- Error handling.
- Compatibility tests.

Deliverable: working Linkding-compatible API.

## Phase 4 – Browser extension

- Configure official extension.
- Test real extension requests.
- Fix behavioral differences.
- Validate all critical workflows.

Deliverable: unmodified official extension working with Lynk.

This phase is a release blocker.

## Phase 5 – Web interface

- Bookmark list.
- Bookmark creation.
- Editing.
- Deletion.
- Tags.
- Search.
- Archive.
- Responsive layout.

Deliverable: usable web application.

## Phase 6 – Metadata and background workers

- Metadata fetching.
- SSRF protection.
- Worker pool.
- Timeouts and size limits.
- Failure handling.

Deliverable: asynchronous metadata extraction.

## Phase 7 – Import/export and maintenance

- Netscape HTML import.
- Netscape HTML export.
- Database backup.
- Database restore.
- Operational documentation.

Deliverable: migration-ready application.

## Phase 8 – Performance and release

- Benchmarking.
- Memory profiling.
- Docker image optimization.
- Security review.
- Compatibility regression tests.
- Release documentation.

Deliverable: production-ready Lynk.

---

# 13. Definition of Done

Lynk is considered ready for its first stable release when:

- It runs in Docker Compose.
- It uses SQLite.
- It has persistent storage.
- It provides a working web interface.
- It supports bookmark CRUD.
- It supports tags and search.
- It supports API token authentication.
- It passes Linkding API compatibility tests.
- The official unmodified Linkding extension works.
- It supports Netscape HTML import/export.
- It has a reliable backup mechanism.
- It has a documented Makefile.
- It has no mandatory external services.
- Its resource consumption has been measured.
- Its security requirements have been reviewed.

---

# 14. First Task for AI

Do not implement the application yet.

Start by completing Phase 0.

Inspect the official Linkding backend and browser extension repositories.

Prepare:

1. `docs/api-compatibility.md`
2. `docs/architecture.md`
3. `docs/database-schema.md`
4. `docs/makefile-plan.md`
5. A list of required dependencies.
6. A list of unresolved technical decisions.
7. An implementation plan broken into small tasks.

For every API endpoint, document:

- HTTP method.
- URL.
- Authentication requirements.
- Query parameters.
- Request JSON.
- Response JSON.
- HTTP status codes.
- Error behavior.
- Whether the official extension uses it.

Clearly distinguish verified behavior from assumptions.

Present the research findings before starting Phase 1.

**The most important objective: Lynk must be a lightweight application that behaves like Linkding where compatibility matters, while remaining independent in its internal architecture.**
