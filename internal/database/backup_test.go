package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupIsConsistentWithUncheckpointedWAL(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig(t)
	db := openTestDB(t, cfg)
	insertFixture(t, db)

	// Keep the committed data in WAL: nothing here checkpoints it.
	walInfo, err := os.Stat(cfg.Path + "-wal")
	if err != nil {
		t.Fatalf("stat WAL file: %v", err)
	}
	if walInfo.Size() == 0 {
		t.Fatal("WAL file is empty; test would not prove WAL-aware backup")
	}

	backupPath := filepath.Join(t.TempDir(), "backups", "lynk.db")
	if err := Backup(ctx, db, backupPath); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if err := VerifyBackup(ctx, backupPath); err != nil {
		t.Fatalf("VerifyBackup: %v", err)
	}

	backup, err := sql.Open(driverName, backupPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backup.Close() })
	var result string
	if err := backup.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&result); err != nil {
		t.Fatal(err)
	}
	if result != "ok" {
		t.Fatalf("backup integrity_check = %q, want ok", result)
	}
	if got := countRows(t, backup, `SELECT count(*) FROM bookmarks`); got != 2 {
		t.Errorf("bookmarks in backup = %d, want 2", got)
	}
	if got := countRows(t, backup, `SELECT count(*) FROM bookmark_search WHERE bookmark_search MATCH 'alpha'`); got != 1 {
		t.Errorf("FTS rows in backup = %d, want 1", got)
	}
	if got := countRows(t, backup, `SELECT count(*) FROM schema_migrations`); got != 3 {
		t.Errorf("migration rows in backup = %d, want 3", got)
	}
	var title string
	if err := backup.QueryRowContext(ctx, `SELECT title FROM bookmarks WHERE id = 1`).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "Alpha Title" {
		t.Errorf("title in backup = %q, want %q", title, "Alpha Title")
	}
}

func TestBackupDoesNotOverwriteExistingFile(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, testConfig(t))
	backupPath := filepath.Join(t.TempDir(), "existing.db")
	if err := os.WriteFile(backupPath, []byte("previous backup"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Backup(ctx, db, backupPath)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Backup error = %v, want already exists", err)
	}
	content, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "previous backup" {
		t.Fatalf("existing backup was modified: %q", content)
	}
}

func TestBackupRejectsDSNPath(t *testing.T) {
	db := openTestDB(t, testConfig(t))
	err := Backup(context.Background(), db, "file:/tmp/lynk-backup.db?mode=ro")
	if err == nil || !strings.Contains(err.Error(), "backup destination") {
		t.Fatalf("Backup error = %v, want backup destination error", err)
	}
}
