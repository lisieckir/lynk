# lynk

Lynk is a lightweight, self-hosted bookmark manager: a single Go process with
SQLite, server-rendered HTML and a Linkding-compatible API. Phase 1 is building
the project foundation; application features are intentionally not implemented
yet.

## Development

The module path is `github.com/lisieckir/lynk`. Go `1.27.4` and
`modernc.org/sqlite` `v1.63.0` are pinned **provisionally**, pending the P1-003
SQLite validation.

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

## Documentation

* [Specification](docs/SPEC.md)
* [Technical architecture](docs/architecture.md)
* [Development and toolchain](docs/development.md)
* [Phase 1 plan](docs/phase1-plan.md)
* [Agent board](docs/board/README.md)
