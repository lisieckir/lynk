package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testConfig(t *testing.T) Config {
	t.Helper()
	return Config{Path: filepath.Join(t.TempDir(), "data", "lynk.db")}
}

func openTestDB(t *testing.T, cfg Config) *sql.DB {
	t.Helper()
	db, err := Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Open(%+v): %v", cfg, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func insertFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	const stamp = "2026-01-02T03:04:05Z"
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, created_at, updated_at)
		VALUES (1, 'alice', 'not-a-real-hash', ?, ?)`, stamp, stamp); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO bookmarks (
			id, user_id, url, url_normalized, title, description, notes, tags,
			is_archived, unread, shared, date_added, date_modified
		) VALUES
			(1, 1, 'https://example.com/a', 'https://example.com/a',
			 'Alpha Title', 'Alpha description', 'alpha notes', 'alpha', 0, 0, 0, ?, ?),
			(2, 1, 'https://example.com/b', 'https://example.com/b',
			 'Second Title', 'Second description', 'beta notes', 'beta', 0, 0, 0, ?, ?)`,
		stamp, stamp, stamp, stamp); err != nil {
		t.Fatalf("insert bookmarks: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO tags (id, user_id, name, date_added) VALUES (1, 1, 'alpha', ?)`, stamp); err != nil {
		t.Fatalf("insert tag: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO bookmark_tags (bookmark_id, tag_id) VALUES (1, 1)`); err != nil {
		t.Fatalf("insert bookmark tag: %v", err)
	}
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	return n
}

func TestOpenFreshAndReopenPreservesData(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig(t)

	db := openTestDB(t, cfg)
	if err := verifyPragmas(ctx, db, defaultBusyTimeout); err != nil {
		t.Fatalf("pragmas on fresh database: %v", err)
	}
	info, err := os.Stat(cfg.Path)
	if err != nil {
		t.Fatalf("stat new database: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("database is not a regular file: %s", info.Mode())
	}
	dirInfo, err := os.Stat(filepath.Dir(cfg.Path))
	if err != nil {
		t.Fatalf("stat database directory: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("database directory mode = %o, want 700", got)
	}
	if _, err := os.Stat(cfg.Path + "-wal"); err != nil {
		t.Errorf("WAL file was not created: %v", err)
	}

	insertFixture(t, db)
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	reopened := openTestDB(t, cfg)
	if err := verifyPragmas(ctx, reopened, defaultBusyTimeout); err != nil {
		t.Fatalf("pragmas after reopen: %v", err)
	}
	if got := countRows(t, reopened, `SELECT count(*) FROM bookmarks`); got != 2 {
		t.Errorf("bookmarks after reopen = %d, want 2", got)
	}
	var title string
	if err := reopened.QueryRowContext(ctx, `SELECT title FROM bookmarks WHERE id = 1`).Scan(&title); err != nil {
		t.Fatalf("read bookmark after reopen: %v", err)
	}
	if title != "Alpha Title" {
		t.Errorf("title after reopen = %q, want %q", title, "Alpha Title")
	}
	if got := countRows(t, reopened, `SELECT count(*) FROM schema_migrations`); got != 2 {
		t.Errorf("migration rows after reopen = %d, want 2", got)
	}
}

func TestOpenCreatesPrivateDatabaseFile(t *testing.T) {
	cfg := testConfig(t)
	_ = openTestDB(t, cfg)
	info, err := os.Stat(cfg.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("new database file mode = %o, want 600", got)
	}
}

func TestOpenPreservesExistingDatabasePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.db")
	if err := os.WriteFile(path, nil, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}

	db := openTestDB(t, Config{Path: path})
	if err := verifyPragmas(context.Background(), db, defaultBusyTimeout); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, db, `SELECT count(*) FROM schema_migrations`); got != 2 {
		t.Errorf("migration rows on existing database = %d, want 2", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Errorf("existing database file mode = %o, want 640 (unchanged)", got)
	}
}

func TestValidatePathRejectsUnsafeValues(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target.db")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link.db")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	cases := []struct {
		name string
		path string
	}{
		{"empty", ""},
		{"relative", filepath.Join("data", "lynk.db")},
		{"root", string(filepath.Separator)},
		{"memory DSN", ":memory:"},
		{"file URI", "file:" + filepath.Join(base, "uri.db")},
		{"query string", filepath.Join(base, "q.db") + "?mode=ro"},
		{"fragment", filepath.Join(base, "f.db") + "#frag"},
		{"NUL byte", filepath.Join(base, "nul.db") + "\x00"},
		{"newline", filepath.Join(base, "nl.db") + "\n"},
		{"directory", base},
		{"symlink", link},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validatePath(tc.path)
			if err == nil {
				t.Fatalf("validatePath(%q) = %q, want error", tc.path, got)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lynk.db")
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{"negative busy timeout", Config{Path: path, BusyTimeout: -time.Second}, "busy timeout"},
		{"sub-millisecond busy timeout", Config{Path: path, BusyTimeout: time.Microsecond}, "busy timeout"},
		{"negative max open", Config{Path: path, MaxOpenConns: -1}, "connection pool"},
		{"negative max idle", Config{Path: path, MaxIdleConns: -1}, "connection pool"},
		{"idle above open", Config{Path: path, MaxOpenConns: 1, MaxIdleConns: 2}, "max idle"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Open(context.Background(), tc.cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Open error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestPragmasOnEveryPooledConnection(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig(t)
	cfg.BusyTimeout = 1500 * time.Millisecond
	cfg.MaxOpenConns = 4
	cfg.MaxIdleConns = 1
	db := openTestDB(t, cfg)

	if got := db.Stats().MaxOpenConnections; got != cfg.MaxOpenConns {
		t.Fatalf("MaxOpenConnections = %d, want %d", got, cfg.MaxOpenConns)
	}

	const workers = 4
	acquired := make(chan struct{}, workers)
	release := make(chan struct{})
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := db.Conn(ctx)
			if err != nil {
				errs <- err
				acquired <- struct{}{}
				return
			}
			defer func() { _ = conn.Close() }()
			err = verifyPragmasOnConn(ctx, conn, cfg.BusyTimeout)
			acquired <- struct{}{}
			<-release // Hold all connections at once to force separate physical ones.
			errs <- err
		}()
	}
	for i := 0; i < workers; i++ {
		<-acquired
	}
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("pooled connection pragma check: %v", err)
		}
	}
}

func TestForeignKeysEnforcedAndTransactionRollback(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, testConfig(t))

	_, err := db.ExecContext(ctx, `
		INSERT INTO bookmarks (
			user_id, url, url_normalized, date_added, date_modified
		) VALUES (999, 'https://example.com/x', 'https://example.com/x', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	if err == nil || !strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		t.Fatalf("foreign key violation error = %v, want FOREIGN KEY constraint failed", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, created_at, updated_at)
		VALUES (2, 'bob', 'hash', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO bookmarks (
			id, user_id, url, url_normalized, title, tags, date_added, date_modified
		) VALUES (3, 2, 'https://example.com/rollback', 'https://example.com/rollback', 'Rollback', 'rollback', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	if got := countRows(t, db, `SELECT count(*) FROM users`); got != 0 {
		t.Errorf("users after rollback = %d, want 0", got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM bookmarks`); got != 0 {
		t.Errorf("bookmarks after rollback = %d, want 0", got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM bookmark_search WHERE bookmark_search MATCH 'rollback'`); got != 0 {
		t.Errorf("FTS rows after rollback = %d, want 0", got)
	}

	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, created_at, updated_at)
		VALUES (3, 'carol', 'hash', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, db, `SELECT count(*) FROM users`); got != 1 {
		t.Errorf("users after commit = %d, want 1", got)
	}
}

func TestFTS5Smoke(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, testConfig(t))

	rows, err := db.QueryContext(ctx, `PRAGMA compile_options`)
	if err != nil {
		t.Fatal(err)
	}
	fts5Enabled := false
	for rows.Next() {
		var option string
		if err := rows.Scan(&option); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToUpper(option), "ENABLE_FTS5") {
			fts5Enabled = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if !fts5Enabled {
		t.Fatal("driver build has no ENABLE_FTS5 compile option")
	}

	insertFixture(t, db)
	if got := countRows(t, db, `SELECT count(*) FROM bookmark_search WHERE bookmark_search MATCH 'alpha'`); got != 1 {
		t.Errorf("FTS match alpha = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM bookmark_search WHERE bookmark_search MATCH 'notes'`); got != 2 {
		t.Errorf("FTS match notes = %d, want 2", got)
	}

	if _, err := db.ExecContext(ctx, `
		UPDATE bookmarks
		SET title = 'Gamma Title', description = 'Gamma description',
		    notes = 'gamma notes', tags = 'gamma'
		WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, db, `SELECT count(*) FROM bookmark_search WHERE bookmark_search MATCH 'alpha'`); got != 0 {
		t.Errorf("FTS match alpha after update = %d, want 0", got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM bookmark_search WHERE bookmark_search MATCH 'gamma'`); got != 1 {
		t.Errorf("FTS match gamma after update = %d, want 1", got)
	}

	if err := RebuildSearchIndex(ctx, db); err != nil {
		t.Fatalf("RebuildSearchIndex: %v", err)
	}
	if got := countRows(t, db, `SELECT count(*) FROM bookmark_search WHERE bookmark_search MATCH 'gamma'`); got != 1 {
		t.Errorf("FTS match gamma after rebuild = %d, want 1", got)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM bookmarks WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, db, `SELECT count(*) FROM bookmark_search WHERE bookmark_search MATCH 'gamma'`); got != 0 {
		t.Errorf("FTS match gamma after delete = %d, want 0", got)
	}
}

func TestMigrationIdempotence(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig(t)
	db := openTestDB(t, cfg)

	for i := 0; i < 3; i++ {
		if err := Migrate(ctx, db); err != nil {
			t.Fatalf("Migrate run %d: %v", i+1, err)
		}
	}
	if got := countRows(t, db, `SELECT count(*) FROM schema_migrations`); got != 2 {
		t.Fatalf("migration rows = %d, want 2", got)
	}
	rows, err := db.QueryContext(ctx,
		`SELECT version, name, checksum FROM schema_migrations ORDER BY version`,
	)
	if err != nil {
		t.Fatal(err)
	}
	wantNames := map[int]string{1: "initial", 2: "bookmark_tag_fts_triggers"}
	for rows.Next() {
		var version int
		var name, checksum string
		if err := rows.Scan(&version, &name, &checksum); err != nil {
			t.Fatal(err)
		}
		if wantName, ok := wantNames[version]; !ok || name != wantName || len(checksum) != 64 {
			t.Errorf("migration record = (%d, %q, %q)", version, name, checksum)
		}
		delete(wantNames, version)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if len(wantNames) != 0 {
		t.Errorf("missing migration records: %v", wantNames)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestDB(t, cfg)
	if got := countRows(t, reopened, `SELECT count(*) FROM schema_migrations`); got != 2 {
		t.Fatalf("migration rows after restart = %d, want 2", got)
	}

	if _, err := reopened.ExecContext(ctx, `UPDATE schema_migrations SET checksum = 'changed' WHERE version = 1`); err != nil {
		t.Fatal(err)
	}
	err = Migrate(ctx, reopened)
	if err == nil || !strings.Contains(err.Error(), "has changed since it was applied") {
		t.Fatalf("Migrate with changed checksum error = %v, want checksum error", err)
	}
}

func TestMigrateFailsClearlyOnUnexpectedTrackingTable(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "broken.db")
	raw, err := sql.Open(driverName, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = Open(ctx, Config{Path: path})
	if err == nil || !strings.Contains(err.Error(), "schema_migrations") {
		t.Fatalf("Open error = %v, want schema_migrations failure", err)
	}
}
