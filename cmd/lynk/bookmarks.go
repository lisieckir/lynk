package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lisieckir/lynk/internal/bookmark"
	"github.com/lisieckir/lynk/internal/config"
	"github.com/lisieckir/lynk/internal/database"
	"github.com/lisieckir/lynk/internal/netscape"
	"github.com/lisieckir/lynk/internal/user"
)

const bookmarksUsage = "usage: lynk bookmarks import --file PATH --username NAME | --user-id ID [--max-bytes N] [--max-bookmarks N] [--continue-on-error]\n" +
	"       lynk bookmarks export --file PATH --username NAME | --user-id ID [--archived active|archived|all] [--overwrite] [--max-records N]"

func runBookmarksCmd(ctx context.Context, args []string, lookup config.LookupFunc, in io.Reader, out io.Writer) error {
	if len(args) == 0 {
		return errors.New(bookmarksUsage)
	}
	switch args[0] {
	case "import":
		return runBookmarksImport(ctx, args[1:], lookup, in, out)
	case "export":
		return runBookmarksExport(ctx, args[1:], lookup, out)
	default:
		return fmt.Errorf("unknown bookmarks command %q\n%s", args[0], bookmarksUsage)
	}
}

// resolveOwner enforces explicit ownership. There is no default account and
// nothing in an HTML file can select the owner.
func resolveOwner(ctx context.Context, db *sql.DB, username string, userID int64) (user.Identity, error) {
	hasName, hasID := strings.TrimSpace(username) != "", userID > 0
	switch {
	case hasName && hasID:
		return user.Identity{}, errors.New("provide exactly one of --username or --user-id")
	case hasName:
		identity, err := user.LookupByUsername(ctx, db, username)
		if err != nil {
			if errors.Is(err, user.ErrNotFound) {
				return user.Identity{}, fmt.Errorf("user %q not found", username)
			}
			return user.Identity{}, err
		}
		return identity, nil
	case hasID:
		identity, err := user.LookupByID(ctx, db, userID)
		if err != nil {
			if errors.Is(err, user.ErrNotFound) {
				return user.Identity{}, fmt.Errorf("user id %d not found", userID)
			}
			return user.Identity{}, err
		}
		return identity, nil
	default:
		return user.Identity{}, errors.New("one of --username or --user-id is required")
	}
}

func runBookmarksImport(ctx context.Context, args []string, lookup config.LookupFunc, in io.Reader, out io.Writer) (retErr error) {
	fs := flag.NewFlagSet("bookmarks import", flag.ContinueOnError)
	fs.SetOutput(out)
	username := fs.String("username", "", "Explicit owner username (required unless --user-id is given)")
	userID := fs.Int64("user-id", 0, "Explicit owner user id (required unless --username is given)")
	source := fs.String("file", "", "Netscape HTML path, or - for stdin (required)")
	maxBytes := fs.Int64("max-bytes", netscape.DefaultMaxBytes, "Maximum input size in bytes")
	maxBookmarks := fs.Int("max-bookmarks", netscape.DefaultMaxBookmarks, "Maximum number of bookmarks to import")
	continueOnError := fs.Bool("continue-on-error", false, "Skip records with invalid URLs instead of aborting")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q\n%s", fs.Arg(0), bookmarksUsage)
	}
	if strings.TrimSpace(*source) == "" {
		return errors.New("--file is required\n" + bookmarksUsage)
	}
	if *maxBytes <= 0 || *maxBytes > netscape.HardMaxBytes {
		return fmt.Errorf("--max-bytes must be between 1 and %d", netscape.HardMaxBytes)
	}
	if *maxBookmarks <= 0 || *maxBookmarks > netscape.HardMaxBookmarks {
		return fmt.Errorf("--max-bookmarks must be between 1 and %d", netscape.HardMaxBookmarks)
	}

	cfg, err := config.LoadFrom(lookup)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	db, err := database.Open(ctx, database.Config{Path: cfg.DatabasePath})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("close database: %w", cerr)
		}
	}()
	identity, err := resolveOwner(ctx, db, *username, *userID)
	if err != nil {
		return err
	}
	svc, err := bookmark.NewService(db)
	if err != nil {
		return fmt.Errorf("bookmark service: %w", err)
	}

	reader := in
	if *source != "-" {
		src, openErr := os.Open(*source) // read-only: import never overwrites its source
		if openErr != nil {
			return fmt.Errorf("open import source: %w", openErr)
		}
		defer func() {
			if cerr := src.Close(); cerr != nil && retErr == nil {
				retErr = fmt.Errorf("close import source: %w", cerr)
			}
		}()
		reader = src
	}

	sum, err := netscape.Import(ctx, db, svc, identity.ID, reader, netscape.ImportOptions{
		Limits: netscape.Limits{
			MaxBytes:     *maxBytes,
			MaxBookmarks: *maxBookmarks,
		},
		ContinueOnError: *continueOnError,
	})
	if err != nil {
		return fmt.Errorf("import bookmarks: %w", err)
	}
	_, err = fmt.Fprintf(out, "import: parsed=%d created=%d merged=%d skipped=%d failed=%d\n",
		sum.Parsed, sum.Created, sum.Merged, sum.Skipped, sum.Failed)
	return err
}

func runBookmarksExport(ctx context.Context, args []string, lookup config.LookupFunc, out io.Writer) (retErr error) {
	fs := flag.NewFlagSet("bookmarks export", flag.ContinueOnError)
	fs.SetOutput(out)
	username := fs.String("username", "", "Explicit owner username (required unless --user-id is given)")
	userID := fs.Int64("user-id", 0, "Explicit owner user id (required unless --username is given)")
	destination := fs.String("file", "", "Netscape HTML path, or - for stdout (required)")
	archived := fs.String("archived", string(netscape.FilterActive), "Which rows to export: active, archived or all")
	overwrite := fs.Bool("overwrite", false, "Explicitly replace an existing destination file")
	maxRecords := fs.Int("max-records", netscape.MaxExportRecords, "Maximum number of bookmarks to export")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q\n%s", fs.Arg(0), bookmarksUsage)
	}
	if strings.TrimSpace(*destination) == "" {
		return errors.New("--file is required\n" + bookmarksUsage)
	}
	switch netscape.ArchiveFilter(*archived) {
	case netscape.FilterActive, netscape.FilterArchived, netscape.FilterAll:
	default:
		return fmt.Errorf("--archived must be active, archived or all")
	}
	if *maxRecords <= 0 || *maxRecords > netscape.HardMaxExportRecords {
		return fmt.Errorf("--max-records must be between 1 and %d", netscape.HardMaxExportRecords)
	}

	cfg, err := config.LoadFrom(lookup)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	db, err := database.Open(ctx, database.Config{Path: cfg.DatabasePath})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("close database: %w", cerr)
		}
	}()
	identity, err := resolveOwner(ctx, db, *username, *userID)
	if err != nil {
		return err
	}
	svc, err := bookmark.NewService(db)
	if err != nil {
		return fmt.Errorf("bookmark service: %w", err)
	}

	written := 0
	opts := netscape.ExportOptions{
		Filter:     netscape.ArchiveFilter(*archived),
		MaxRecords: *maxRecords,
		Written:    &written,
	}
	if *destination == "-" {
		if err := netscape.Export(ctx, svc, identity.ID, out, opts); err != nil {
			return fmt.Errorf("export bookmarks: %w", err)
		}
		return nil
	}

	path, err := prepareDestination(cfg.DatabasePath, *destination, *overwrite)
	if err != nil {
		return err
	}
	// A non-overwriting export uses O_EXCL so the destination cannot be
	// replaced silently. --overwrite writes a sibling temp file and renames it
	// into place, so a failed export never truncates the old file.
	tempPath := ""
	var created *os.File
	if *overwrite {
		created, err = os.CreateTemp(filepath.Dir(path), ".lynk-export-*")
		if err != nil {
			return fmt.Errorf("create temporary export file: %w", err)
		}
		tempPath = created.Name()
	} else {
		created, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return fmt.Errorf("open export destination: %w", err)
		}
	}
	committed, closed := false, false
	cleanupPath := path
	if tempPath != "" {
		cleanupPath = tempPath
	}
	defer func() {
		if !closed {
			_ = created.Close()
		}
		if !committed {
			_ = os.Remove(cleanupPath)
		}
	}()
	if err := netscape.Export(ctx, svc, identity.ID, created, opts); err != nil {
		return fmt.Errorf("export bookmarks: %w", err)
	}
	if err := created.Sync(); err != nil {
		return fmt.Errorf("sync export destination: %w", err)
	}
	if err := created.Close(); err != nil {
		return fmt.Errorf("close export destination: %w", err)
	}
	closed = true
	if tempPath != "" {
		if err := os.Rename(tempPath, path); err != nil {
			return fmt.Errorf("replace export destination: %w", err)
		}
	}
	committed = true
	_, err = fmt.Fprintf(out, "export: filter=%s bookmarks=%d\n", *archived, written)
	return err
}

// prepareDestination rejects overwriting the database (including its WAL and
// shared-memory sidecars) and refuses an existing file unless --overwrite was
// explicit. It returns the absolute path used for the atomic write.
func prepareDestination(dbPath, destination string, overwrite bool) (string, error) {
	abs, err := filepath.Abs(destination)
	if err != nil {
		return "", fmt.Errorf("export destination: %w", err)
	}
	dbAbs, err := filepath.Abs(dbPath)
	if err != nil {
		return "", fmt.Errorf("database path: %w", err)
	}
	for _, protected := range []string{dbAbs, dbAbs + "-wal", dbAbs + "-shm"} {
		if abs == protected {
			return "", errors.New("export destination must not be the database file")
		}
	}
	info, err := os.Lstat(abs)
	switch {
	case err == nil:
		if !info.Mode().IsRegular() {
			return "", errors.New("export destination is not a regular file")
		}
		if !overwrite {
			return "", errors.New("export destination already exists; pass --overwrite to replace it")
		}
	case errors.Is(err, os.ErrNotExist):
	default:
		return "", fmt.Errorf("inspect export destination: %w", err)
	}
	return abs, nil
}
