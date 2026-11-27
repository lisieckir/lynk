# lynk

Lynk is a lightweight, self-hosted bookmark manager: a single Go process with
SQLite, server-rendered HTML and a Linkding-compatible API. The MVP includes
bookmark CRUD/search/tags, the official Linkding extension contract, metadata
fetching, a web UI, Netscape import/export and Docker deployment.

## Development

The module path is `github.com/lisieckir/lynk`. Go `1.27.4` and
`modernc.org/sqlite` `v1.63.0` are validated pins for the current Linux AMD64
release target.

```sh
go build ./...
go test ./...
go vet ./...
```

## Bookmark import/export

Netscape bookmark HTML can be migrated with explicit ownership:

```sh
lynk bookmarks import --username alice --file bookmarks.html
lynk bookmarks import --user-id 1 --file -
lynk bookmarks export --username alice --file exported.html
lynk bookmarks export --user-id 1 --file -
```

See [development.md](docs/development.md#netscape-html-importexport-p2-010)
for the tag/folder and archived/unread mapping, bounds, atomicity and clobber
policy.

## Accounts and tokens (web)

* On a fresh database, open the web UI to register the first admin account (registration closes after the first account). Registration is rate-limited.
* Users can sign in with username/password or a legacy API token; API tokens can be managed under **Tokens** and work with the official Linkding extension (`Authorization: Token <token>`).
* Admins can manage users and tokens from the **Admin** panel. Admins cannot disable the last active admin (lockout protection). Password resets and user disable/enable invalidate existing web sessions (epoch-based).

## Documentation

* [Specification](docs/SPEC.md)
* [Technical architecture](docs/architecture.md)
* [Development and toolchain](docs/development.md)
* [Phase 1 plan](docs/phase1-plan.md)
* [Agent board](docs/board/README.md)
