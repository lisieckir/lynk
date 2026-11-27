package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// P2-012 adds users/roles/session-epoch migrations. These tests pin the
// migration contracts that protect existing installations: applying again is a
// no-op, a changed applied migration is rejected, a failed migration rolls
// back atomically, and a pre-P2-012 database promotes its first user to admin.

func TestP212MigrateIsIdempotentAndRejectsChangedMigration(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, testConfig(t))

	var before string
	if err := db.QueryRowContext(ctx,
		`SELECT applied_at FROM schema_migrations WHERE version = 4`).Scan(&before); err != nil {
		t.Fatalf("read migration 4 before re-run: %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	var after string
	if err := db.QueryRowContext(ctx,
		`SELECT applied_at FROM schema_migrations WHERE version = 4`).Scan(&after); err != nil {
		t.Fatalf("read migration 4 after re-run: %v", err)
	}
	if after != before {
		t.Errorf("migration 4 applied_at changed from %q to %q on re-run", before, after)
	}
	if got := countRows(t, db, `SELECT count(*) FROM schema_migrations`); got != 4 {
		t.Errorf("migration rows after re-run = %d, want 4", got)
	}
	for _, column := range []string{"role", "disabled", "session_epoch"} {
		if got := countRows(t, db, `SELECT count(*) FROM pragma_table_info('users') WHERE name = ?`, column); got != 1 {
			t.Errorf("users column %q count = %d, want 1", column, got)
		}
	}

	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("no embedded migrations")
	}
	changed := migrations[len(migrations)-1]
	changed.checksum = strings.Repeat("0", len(changed.checksum))
	if _, err := migrationApplied(ctx, db, changed); err == nil ||
		!strings.Contains(err.Error(), "has changed since it was applied") {
		t.Fatalf("changed applied migration error = %v, want checksum rejection", err)
	}
	if got := countRows(t, db, `SELECT count(*) FROM schema_migrations`); got != 4 {
		t.Errorf("rejected migration changed schema_migrations: %d rows", got)
	}
}

func TestP212ApplyMigrationRollsBackOnFailure(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, testConfig(t))
	broken := migration{
		version:  900,
		name:     "broken_probe",
		sql:      "CREATE TABLE p212_rollback_probe(id INTEGER); INSERT INTO missing_table VALUES (1);",
		checksum: strings.Repeat("1", 64),
	}
	if err := applyMigration(ctx, db, broken); err == nil {
		t.Fatal("applyMigration accepted a broken migration")
	}
	if got := countRows(t, db,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'p212_rollback_probe'`); got != 0 {
		t.Errorf("broken migration left its table behind: %d", got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM schema_migrations WHERE version = 900`); got != 0 {
		t.Errorf("broken migration left a tracking row behind: %d", got)
	}
}

func TestP212LegacyUsersMigrateToRolesAndSessionEpoch(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig(t)
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o700); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if len(migrations) < 4 {
		t.Fatalf("embedded migrations = %d, want at least 4", len(migrations))
	}

	// Reconstruct the state of a database that predates P2-012: only the
	// initial schema and FTS triggers are applied, and users have no role.
	dsn := fmt.Sprintf("%s?_busy_timeout=%d&_foreign_keys=1&_journal_mode=WAL", cfg.Path, defaultBusyTimeout.Milliseconds())
	legacy, err := sql.Open(driverName, dsn)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	if _, err := legacy.ExecContext(ctx, createMigrationsTable); err != nil {
		_ = legacy.Close()
		t.Fatalf("create legacy migration tracker: %v", err)
	}
	for _, m := range migrations[:2] {
		if _, err := legacy.ExecContext(ctx, m.sql); err != nil {
			_ = legacy.Close()
			t.Fatalf("apply legacy migration %d: %v", m.version, err)
		}
		if _, err := legacy.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name, checksum) VALUES (?, ?, ?)`,
			m.version, m.name, m.checksum); err != nil {
			_ = legacy.Close()
			t.Fatalf("record legacy migration %d: %v", m.version, err)
		}
	}
	for _, user := range []struct {
		id   int
		name string
	}{{1, "legacy-admin"}, {2, "legacy-user"}} {
		if _, err := legacy.ExecContext(ctx,
			`INSERT INTO users (id, username, password_hash, created_at, updated_at) VALUES (?, ?, '!', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
			user.id, user.name); err != nil {
			_ = legacy.Close()
			t.Fatalf("insert legacy user: %v", err)
		}
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	db := openTestDB(t, cfg) // Open applies the pending P2-012 migrations.
	for _, tc := range []struct {
		id       int
		username string
		role     string
	}{
		{1, "legacy-admin", "admin"},
		{2, "legacy-user", "user"},
	} {
		var role string
		var disabled, epoch int
		if err := db.QueryRowContext(ctx,
			`SELECT role, disabled, session_epoch FROM users WHERE id = ?`, tc.id).Scan(&role, &disabled, &epoch); err != nil {
			t.Fatalf("read migrated user %d: %v", tc.id, err)
		}
		if role != tc.role || disabled != 0 || epoch != 0 {
			t.Errorf("migrated user %d = (%q, %d, %d), want (%q, 0, 0)",
				tc.id, role, disabled, epoch, tc.role)
		}
	}
	if got := countRows(t, db, `SELECT count(*) FROM schema_migrations`); got != 4 {
		t.Errorf("migration rows after upgrade = %d, want 4", got)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate after upgrade: %v", err)
	}
	if got := countRows(t, db, `SELECT count(*) FROM schema_migrations`); got != 4 {
		t.Errorf("migration rows after re-run = %d, want 4", got)
	}
	// New users keep the safe defaults introduced by the migration.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, created_at, updated_at) VALUES ('new-user', '!', '2026-01-02T00:00:00Z', '2026-01-02T00:00:00Z')`); err != nil {
		t.Fatalf("insert new user: %v", err)
	}
	var role string
	var disabled, epoch int
	if err := db.QueryRowContext(ctx,
		`SELECT role, disabled, session_epoch FROM users WHERE username = 'new-user'`).Scan(&role, &disabled, &epoch); err != nil {
		t.Fatalf("read new user: %v", err)
	}
	if role != "user" || disabled != 0 || epoch != 0 {
		t.Errorf("new user = (%q, %d, %d), want (user, 0, 0)", role, disabled, epoch)
	}
}
