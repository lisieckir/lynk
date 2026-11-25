package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Migration SQL lives beside this package so it can be reviewed against
// docs/database-schema.md and applied without a separate deployment step.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

const createMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY CHECK (version > 0),
    name       TEXT NOT NULL,
    checksum   TEXT NOT NULL,
    applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
)
`

type migration struct {
	version  int
	name     string
	sql      string
	checksum string
}

// Migrate applies embedded migrations in parsed numeric version order and
// records each one in schema_migrations. Applied migrations are skipped; a
// changed migration body is an error rather than a silent schema rewrite.
// Every migration and its tracking row commit together.
func Migrate(ctx context.Context, db *sql.DB) error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, createMigrationsTable); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	for _, m := range migrations {
		applied, err := migrationApplied(ctx, db, m)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyMigration(ctx, db, m); err != nil {
			return err
		}
	}
	return nil
}

// RebuildSearchIndex repopulates the external-content FTS5 table from
// bookmarks. It is the explicit rebuild path required by
// docs/database-schema.md; normal writes are kept in sync by migration triggers.
func RebuildSearchIndex(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "INSERT INTO bookmark_search(bookmark_search) VALUES('rebuild')"); err != nil {
		return fmt.Errorf("rebuild bookmark_search: %w", err)
	}
	return nil
}

func loadMigrations() ([]migration, error) {
	return loadMigrationsFrom(migrationFiles)
}

// loadMigrationsFrom parses every NNN_name.sql file and orders by the parsed
// integer version. Lexicographic filename order would put 0010 before 0002.
func loadMigrationsFrom(files fs.FS) ([]migration, error) {
	names, err := fs.Glob(files, "migrations/*.sql")
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	if len(names) == 0 {
		return nil, errors.New("no embedded migrations found")
	}

	migrations := make([]migration, 0, len(names))
	seen := make(map[int]string, len(names))
	for _, file := range names {
		base := strings.TrimSuffix(path.Base(file), ".sql")
		parts := strings.SplitN(base, "_", 2)
		if len(parts) != 2 || parts[1] == "" {
			return nil, fmt.Errorf("invalid migration filename %q: want NNN_name.sql", file)
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("invalid migration version in %q: %q", file, parts[0])
		}
		if previous, ok := seen[version]; ok {
			return nil, fmt.Errorf("duplicate migration version %d: %s and %s", version, previous, file)
		}
		seen[version] = file

		body, err := fs.ReadFile(files, file)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", file, err)
		}
		if strings.TrimSpace(string(body)) == "" {
			return nil, fmt.Errorf("migration %q is empty", file)
		}
		sum := sha256.Sum256(body)
		migrations = append(migrations, migration{
			version:  version,
			name:     parts[1],
			sql:      string(body),
			checksum: hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})
	return migrations, nil
}

func migrationApplied(ctx context.Context, db *sql.DB, m migration) (bool, error) {
	var name, checksum string
	err := db.QueryRowContext(ctx,
		`SELECT name, checksum FROM schema_migrations WHERE version = ?`, m.version,
	).Scan(&name, &checksum)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("check schema_migrations for migration %d (%s): %w", m.version, m.name, err)
	case name != m.name || checksum != m.checksum:
		return false, fmt.Errorf(
			"migration %d has changed since it was applied (recorded %s/%s, embedded %s/%s); refusing to rewrite applied schema",
			m.version, name, checksum, m.name, m.checksum,
		)
	default:
		return true, nil
	}
}

func applyMigration(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d (%s): %w", m.version, m.name, err)
	}
	defer tx.Rollback() //nolint:errcheck // Commit below is authoritative.

	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return fmt.Errorf("apply migration %d (%s): %w", m.version, m.name, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name, checksum) VALUES (?, ?, ?)`,
		m.version, m.name, m.checksum,
	); err != nil {
		return fmt.Errorf("record migration %d (%s): %w", m.version, m.name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d (%s): %w", m.version, m.name, err)
	}
	return nil
}
