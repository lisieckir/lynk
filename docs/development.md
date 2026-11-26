# Development and toolchain

Status: P2-007 release validation is accepted; P2-008 closes the CORS/OPTIONS
gap behind an explicit opt-in policy. The versions below were validated by the
P1-003 smoke tests (FTS5, WAL, foreign keys, busy timeout and online backup)
and again by the P2-007 matrix; no driver-version change was made.

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

## Browser CORS (P2-008)

`internal/cors` implements a fail-closed policy for `/api` routes only.
`LYNK_CORS_ALLOWED_ORIGINS` is a comma-separated list of exact `http(s)://host[:port]`
origins; it is empty by default, so no browser origin receives
`Access-Control-*` headers until an operator opts in. Wildcard origins,
wildcard methods/headers, paths, query strings and user info are rejected at
configuration load time. `LYNK_CORS_ALLOWED_METHODS` (default `GET,POST`) and
`LYNK_CORS_ALLOWED_HEADERS` (default `Content-Type`) gate preflight;
`Authorization` must be listed explicitly to allow token-bearing browser
clients. `LYNK_CORS_MAX_AGE` (default `600`, maximum `86400` seconds) emits
`Access-Control-Max-Age`. The policy never emits `Access-Control-Allow-Origin: *`
or `Access-Control-Allow-Credentials`: Lynk authenticates with an
`Authorization` token, not cookies.

A preflight is answered with `204` only for an allowed origin, method and set
of headers, without requiring a token. Disallowed origins, methods or headers
get `403` with an empty body and no CORS headers; ordinary requests from
disallowed origins continue through the normal auth path but carry no CORS
headers, so a browser cannot read them while non-browser clients and the
official extension keep working. Responses vary on `Origin` (and preflight
responses on `Access-Control-Request-Method`/`-Headers`). `GET /health` stays
public, outside the CORS wrapper, with its original body and status.

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

## P2-007 validation (2026-11-26)

Validated in the Linux AMD64 worktree at `main` `8333591` with Go
`go1.27.4-X:nodwarf5 linux/amd64`, Docker Engine `29.10.0`, Compose `5.8.0`,
and the pinned `golangci-lint v2.15.0` kept outside the repository. The
worktree was fast-forwarded from `74d40b5`; no production code was changed.
Scripts and raw logs were kept under `/tmp/opencode/p2-007/`; the board task
[P2-007](board/tasks/P2-007-release-e2e.md) is the sign-off record.

### Go, lint, compatibility and build matrix

`/tmp/opencode/p2-007/go-matrix.sh` ran every step with a separate log and a
summary in `/tmp/opencode/p2-007/go-summary.txt`:

| Check | Result |
|---|---|
| `gofmt -l .` | PASS; no files reported |
| `go vet ./...` | PASS |
| `go build ./...` | PASS |
| `go test ./...` | PASS; includes `internal/worker`, `internal/metadata`, `tests/integration` |
| `go test -race ./...` | PASS; includes `tests/integration` |
| `make test-compat` | PASS |
| `make test-integration` | PASS |
| `make LINT_TOOL=<pinned v2.15.0> check` | PASS; `0 issues` |
| `go mod tidy -diff`, `go mod verify` | PASS; no diff, all modules verified |
| `make LINT_TOOL=<pinned v2.15.0> release` | PASS |

### Docker/Compose live validation

`/tmp/opencode/p2-007/docker-matrix.sh` used an isolated
`COMPOSE_PROJECT_NAME=lynk_p2007`, host port `19090`, `LYNK_ENV=production`
and `LYNK_BASE_URL=http://localhost:19090`. The token was created with
`lynk token create` and is not recorded in documentation.

| Step | Result |
|---|---|
| `docker compose config`, `make docker-release` | PASS |
| `docker compose up -d --build` | PASS; image `sha256:37433b97848a696f81b69e725014483c65f371862a00ccbbc25c07dbc2eec212`, 30,470,977 bytes |
| health wait / `make health` / `docker compose ps` | PASS; container `healthy`, `GET /health` → `{"status":"ok"}`, `0.0.0.0:19090->9090/tcp` |
| API smoke with token | `profile 200`, `tags 200`, no token `401`, three `POST /api/bookmarks/` → `201`, first create `0.0055s`, `check` → `200` in `0.7218s`, paginated list `count=3`, relative `next` |
| `docker compose restart lynk` | PASS; same CID healthy again, detail `200`, logs show graceful shutdown and restart |
| `docker compose down` + `docker compose up -d --build` | PASS; new CID healthy, `profile 200`, `detail 200`, `count=3` with the same marker URLs, `/data/lynk.db` present and non-empty |
| `docker stats --no-stream` | 7.766 MiB idle RSS after recreate |
| final `down` + volume removal | PASS; isolated environment removed |

### Firefox 161.0 with official extension 1.15.1

The official XPI was installed and active locally, so there was no missing-XPI
blocker. The environment had no `geckodriver`, `xdotool`, `xte`, `ydotool`,
`wmctrl`, `scrot`, `Xvfb` or Python Selenium/Playwright, so a GUI click path
was not used. Instead, an isolated profile with a copy of the XPI was started
with:

```sh
firefox --headless --new-instance --profile /tmp/opencode/p2-007/ff-profile \
  --remote-debugging-port 9223 --start-debugger-server ws:6000 \
  --remote-allow-hosts 127.0.0.1,localhost
```

Firefox logged `WebDriver BiDi listening` and `Started devtools server on
6000`; BiDi reported `browserName=firefox`, `browserVersion=161.0`,
`moz:headless=true`; RDP `listAddons` found the linkding extension as a
debuggable web extension. A logging proxy on `127.0.0.1:19091` forwarded all
extension traffic to the container on `19090`. The extension performed:

```text
GET  /api/tags/?limit=5000                 -> 200
GET  /api/user/profile/                    -> 200
GET  /api/bookmarks/check/?url=<encoded>   -> 200
POST /api/bookmarks/?disable_scraping      -> 201  (form submit)
POST /api/bookmarks/?disable_scraping      -> 201  (duplicate merge)
GET  /api/bookmarks/check/?url=<encoded>   -> 200  (bookmark.id=1)
GET  /api/bookmarks/?q=p2-007-firefox&limit=5 -> 200
DELETE /api/bookmarks/1/                   -> 204
```

Every request carried `Authorization: Token <redacted>`; the proxy log
redacted the token. POST and DELETE carried an `Origin` of
`moz-extension://e85c6b9b-e56d-4d6c-b28f-f4fe8904501b`. The popup's
`saveState` was `success` with an empty `errorMessage`; the check response had
nullable metadata and `auto_tags=[]`; after delete the API returned `count=0`.
Raw evidence: `/tmp/opencode/p2-007/firefox-e2e.log` and
`/tmp/opencode/p2-007/proxy-requests.log`.

### CORS/OPTIONS finding (resolved by P2-008)

The proxy passed `Authorization` through and pagination links stayed relative,
but an `OPTIONS /api/bookmarks/` preflight with `Origin: https://example.com`
returned `401` without a token and `405` (`Allow: GET, POST`) with a token. A
simple `GET` with `Origin` and token returned `200` with no `Access-Control-*`
headers. P2-008 subsequently added a fail-closed, opt-in CORS policy with
token-free preflight only for configured origins/methods/headers. The official
extension remains supported, and browser clients require explicit
`LYNK_CORS_ALLOWED_ORIGINS` configuration. See
[api-compatibility.md](api-compatibility.md#proxy-cors-and-options-p2-007).

### Metadata worker note

`POST` with `disable_scraping` returned `201` in `0.0055s`, so metadata work
did not block create. `go test -race ./...`, the worker's
`StopIdempotent`/`EnqueueAfterStop` tests and the live restart's clean
`shutdown complete` all passed. There is no dedicated `goleak` assertion, so
"no leak" is observational rather than a formal test.
