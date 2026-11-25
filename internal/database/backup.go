package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	sqlite "modernc.org/sqlite"
)

// onlineBackupper is the driver-specific capability used for a consistent
// copy. The unexported connection type still satisfies an interface whose
// method returns the exported *sqlite.Backup type.
type onlineBackupper interface {
	NewBackup(string) (*sqlite.Backup, error)
}

const backupStepPages = 100

// Backup writes a consistent SQLite online backup of db to dstPath. It does
// not copy the live database file: the backup API accounts for uncheckpointed
// WAL pages. An existing destination is never overwritten.
func Backup(ctx context.Context, db *sql.DB, dstPath string) error {
	path, err := validatePath(dstPath)
	if err != nil {
		return fmt.Errorf("backup destination: %w", err)
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("backup destination %q already exists", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect backup destination: %w", err)
	}
	if err := ensureParentDir(path); err != nil {
		return err
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire source connection for backup: %w", err)
	}
	defer conn.Close()

	err = conn.Raw(func(raw any) error {
		backupper, ok := raw.(onlineBackupper)
		if !ok {
			return fmt.Errorf("SQLite driver %T does not support the online backup API", raw)
		}
		backup, err := backupper.NewBackup(path)
		if err != nil {
			return fmt.Errorf("start online backup: %w", err)
		}
		for {
			if err := ctx.Err(); err != nil {
				return errors.Join(err, backup.Finish())
			}
			more, err := backup.Step(backupStepPages)
			if err != nil {
				return errors.Join(
					fmt.Errorf("online backup step: %w", err),
					backup.Finish(),
				)
			}
			if !more {
				break
			}
		}
		if err := backup.Finish(); err != nil {
			return fmt.Errorf("finish online backup: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return VerifyBackup(ctx, path)
}

// VerifyBackup opens a backup independently and checks SQLite's page-level
// integrity check. It deliberately uses a plain path (no pragma DSN) so the
// check does not change the backup's settings.
func VerifyBackup(ctx context.Context, path string) error {
	db, err := sql.Open(driverName, path)
	if err != nil {
		return fmt.Errorf("open backup %q: %w", path, err)
	}
	defer db.Close()

	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("integrity check on backup %q: %w", path, err)
	}
	if result != "ok" {
		return fmt.Errorf("integrity check on backup %q: %s", path, result)
	}
	return nil
}
