// Package database opens Lynk's SQLite database, configures connection-scoped
// pragmas and applies tracked migrations.
//
// The SQLite driver is an implementation detail of this package. Other packages
// should receive the resulting *sql.DB rather than depending on the driver.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Register the pins from docs/development.md: modernc.org/sqlite v1.63.0.
	_ "modernc.org/sqlite"
)

const (
	driverName = "sqlite"

	// defaultBusyTimeout matches docs/database-schema.md.
	defaultBusyTimeout = 5 * time.Second

	// SQLite serializes writers, so a small bounded pool is enough. WAL allows
	// readers to run alongside the single writer.
	defaultMaxOpenConns = 4
	defaultMaxIdleConns = 2
)

// Config is the minimal database configuration needed by the foundation task.
// A full environment/config system is implemented separately.
type Config struct {
	// Path is an absolute filesystem path to the SQLite database file. It is
	// treated strictly as a path: DSN/URI syntax is rejected to keep config
	// values from injecting SQL pragmas or ATTACH statements.
	Path string

	// BusyTimeout is applied to every connection. Zero selects 5s; negative
	// values are rejected.
	BusyTimeout time.Duration

	// MaxOpenConns and MaxIdleConns bound the pool. Zero selects conservative
	// defaults; negative values are rejected.
	MaxOpenConns int
	MaxIdleConns int
}

// Open validates the path, creates its parent directory, opens a bounded
// database/sql pool, verifies connection pragmas and applies pending migrations.
// New database files are created with mode 0600; existing files, including
// their permissions, are never changed or overwritten.
func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	path, err := validatePath(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("database path: %w", err)
	}
	busyTimeout, maxOpen, maxIdle, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}
	if err := ensureParentDir(path); err != nil {
		return nil, err
	}
	created, err := createPrivateFile(path)
	if err != nil {
		return nil, err
	}

	dsn := fmt.Sprintf("%s?_busy_timeout=%d&_foreign_keys=1&_journal_mode=WAL",
		path, busyTimeout.Milliseconds())
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)

	// sql.Open is lazy; Ping forces a real connection so path and pragma
	// errors are reported before the caller can serve traffic.
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect database: %w", err)
	}
	if created {
		// Enforce the exact mode even if a restrictive umask altered the
		// mode passed to O_CREATE. Existing files are never chmod'ed.
		if err := os.Chmod(path, 0o600); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("set database file permissions: %w", err)
		}
	}
	if err := verifyPragmas(ctx, db, busyTimeout); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	return db, nil
}

func normalizeConfig(cfg Config) (time.Duration, int, int, error) {
	busyTimeout := cfg.BusyTimeout
	if busyTimeout == 0 {
		busyTimeout = defaultBusyTimeout
	}
	if busyTimeout < 0 || busyTimeout.Milliseconds() == 0 {
		return 0, 0, 0, fmt.Errorf("database busy timeout must be at least 1ms")
	}

	maxOpen := cfg.MaxOpenConns
	if maxOpen == 0 {
		maxOpen = defaultMaxOpenConns
	}
	maxIdle := cfg.MaxIdleConns
	if maxIdle == 0 {
		maxIdle = defaultMaxIdleConns
	}
	if maxOpen < 1 || maxIdle < 0 {
		return 0, 0, 0, fmt.Errorf("invalid connection pool settings: max open=%d, max idle=%d", maxOpen, maxIdle)
	}
	if maxIdle > maxOpen {
		return 0, 0, 0, fmt.Errorf("max idle connections (%d) exceeds max open connections (%d)", maxIdle, maxOpen)
	}
	return busyTimeout, maxOpen, maxIdle, nil
}

// validatePath accepts only absolute, plain filesystem paths. Driver DSNs are
// powerful (a _pragma value can execute arbitrary SQL), so config input must
// never be interpreted as one.
func validatePath(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("path is empty")
	}
	if !filepath.IsAbs(raw) {
		return "", fmt.Errorf("path %q must be absolute", raw)
	}
	if strings.HasPrefix(strings.ToLower(raw), "file:") || raw == ":memory:" {
		return "", fmt.Errorf("path %q uses DSN/URI syntax; provide a filesystem path", raw)
	}
	if strings.ContainsAny(raw, "?#") {
		return "", fmt.Errorf("path %q contains DSN/URI characters", raw)
	}
	for _, r := range raw {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("path %q contains a control character", raw)
		}
	}

	path := filepath.Clean(raw)
	if path == string(filepath.Separator) {
		return "", errors.New("path is a directory")
	}
	info, err := os.Lstat(path)
	switch {
	case err == nil:
		// A symlink is rejected too: a config value should not silently
		// redirect the database to another file.
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("path %q is not a regular file", path)
		}
	case errors.Is(err, os.ErrNotExist):
		// Fresh database: the parent directory is created by Open.
	default:
		return "", fmt.Errorf("inspect path: %w", err)
	}
	return path, nil
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create database directory %q: %w", dir, err)
	}
	return nil
}

// createPrivateFile atomically creates a new database file with mode 0600 and
// reports whether this call created it. Existing files, including their
// permissions, are left untouched.
func createPrivateFile(path string) (bool, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if !errors.Is(err, os.ErrExist) {
			return false, fmt.Errorf("create database file: %w", err)
		}
		info, statErr := os.Lstat(path)
		if statErr != nil {
			return false, fmt.Errorf("inspect existing database file: %w", statErr)
		}
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("path %q is not a regular file", path)
		}
		return false, nil
	}
	if err := f.Close(); err != nil {
		return false, fmt.Errorf("close new database file: %w", err)
	}
	return true, nil
}

func verifyPragmas(ctx context.Context, db *sql.DB, wantBusyTimeout time.Duration) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for pragma verification: %w", err)
	}
	defer conn.Close()
	return verifyPragmasOnConn(ctx, conn, wantBusyTimeout)
}

// verifyPragmasOnConn checks the settings that must hold on every pooled
// connection. They are applied by DSN parameters at connection open time, not
// by Exec on a single connection, so pooled connections cannot miss them.
func verifyPragmasOnConn(ctx context.Context, conn *sql.Conn, wantBusyTimeout time.Duration) error {
	var foreignKeys int
	if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		return fmt.Errorf("read PRAGMA foreign_keys: %w", err)
	}
	if foreignKeys != 1 {
		return fmt.Errorf("PRAGMA foreign_keys = %d, want 1", foreignKeys)
	}

	var journalMode string
	if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return fmt.Errorf("read PRAGMA journal_mode: %w", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		return fmt.Errorf("PRAGMA journal_mode = %q, want wal", journalMode)
	}

	var busyTimeout int64
	if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		return fmt.Errorf("read PRAGMA busy_timeout: %w", err)
	}
	if want := wantBusyTimeout.Milliseconds(); busyTimeout != want {
		return fmt.Errorf("PRAGMA busy_timeout = %d, want %d", busyTimeout, want)
	}
	return nil
}
