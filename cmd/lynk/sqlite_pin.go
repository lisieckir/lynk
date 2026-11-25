//go:build tools

package main

// Pin the SQLite driver selected in docs/architecture.md. The blank import is
// build-tagged so `go mod tidy` keeps modernc.org/sqlite in go.mod without
// registering a database/sql driver in the current executable. Actual driver
// setup belongs to the SQLite foundation task (P1-003).
import _ "modernc.org/sqlite"
