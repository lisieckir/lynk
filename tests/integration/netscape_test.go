package integration

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lisieckir/lynk/internal/auth"
	"github.com/lisieckir/lynk/internal/bookmark"
	"github.com/lisieckir/lynk/internal/database"
	"github.com/lisieckir/lynk/internal/netscape"
)

// TestNetscapeRoundTripBetweenOwners drives the real migrated SQLite database,
// bookmark service and netscape package end to end. It proves that a full
// export/import cycle preserves fields, stays owner-scoped and does not fetch
// metadata for imported URLs.
func TestNetscapeRoundTripBetweenOwners(t *testing.T) {
	ctx := context.Background()
	sourceDB, err := database.Open(ctx, database.Config{Path: filepath.Join(t.TempDir(), "source.db")})
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	defer func() { _ = sourceDB.Close() }()
	sourceUser, err := auth.CreateToken(ctx, sourceDB, "exporter", "round-trip")
	if err != nil {
		t.Fatalf("create source user: %v", err)
	}
	sourceSvc, err := bookmark.NewService(sourceDB)
	if err != nil {
		t.Fatalf("source service: %v", err)
	}
	added := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	in := bookmark.CreateInput{
		URL:         "https://migration.example/page?z=1&a=2",
		Title:       "Migration & <verify> 😀",
		Description: " hacia el é",
		Tags:        []string{netscape.TagUnread, "folder:Projects/Web", "important"},
		Unread:      true,
		DateAdded:   &added,
	}
	if _, err := sourceSvc.Create(ctx, sourceUser.UserID, in); err != nil {
		t.Fatalf("create source bookmark: %v", err)
	}
	var exported bytes.Buffer
	if err := netscape.Export(ctx, sourceSvc, sourceUser.UserID, &exported, netscape.ExportOptions{Filter: netscape.FilterAll}); err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.Contains(exported.String(), "Projects") {
		t.Fatalf("export lost the folder:\n%s", exported.String())
	}

	targetDB, err := database.Open(ctx, database.Config{Path: filepath.Join(t.TempDir(), "target.db")})
	if err != nil {
		t.Fatalf("open target: %v", err)
	}
	defer func() { _ = targetDB.Close() }()
	importer, err := auth.CreateToken(ctx, targetDB, "importer", "round-trip")
	if err != nil {
		t.Fatalf("create target user: %v", err)
	}
	bystanderID, err := addNetscapeUser(ctx, targetDB, "bystander")
	if err != nil {
		t.Fatalf("create bystander: %v", err)
	}
	targetSvc, err := bookmark.NewService(targetDB)
	if err != nil {
		t.Fatalf("target service: %v", err)
	}
	sum, err := netscape.Import(ctx, targetDB, targetSvc, importer.UserID, strings.NewReader(exported.String()), netscape.ImportOptions{})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if sum.Created != 1 || sum.Merged != 0 {
		t.Fatalf("summary = %+v, want created=1", sum)
	}
	item, err := targetSvc.GetByURL(ctx, importer.UserID, in.URL)
	if err != nil {
		t.Fatalf("imported bookmark: %v", err)
	}
	if item.Title != in.Title || item.Description != strings.TrimSpace(in.Description) || !item.Unread {
		t.Errorf("round trip mismatch: %+v", item)
	}
	if !item.DateAdded.Equal(added) {
		t.Errorf("date_added = %v, want %v", item.DateAdded, added)
	}
	if item.FaviconURL != nil || item.PreviewImageURL != nil || item.WebArchiveSnapshotURL != nil {
		t.Errorf("import scheduled metadata/snapshot work: %+v", item)
	}
	if _, err := targetSvc.GetByURL(ctx, bystanderID, in.URL); !errors.Is(err, bookmark.ErrNotFound) {
		t.Errorf("bystander can read imported bookmark: %v", err)
	}
}

func addNetscapeUser(ctx context.Context, db *sql.DB, username string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.ExecContext(ctx, `INSERT INTO users (username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		username, "!", now, now)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO user_profiles (user_id, theme, bookmark_date_display, bookmark_link_target,
		web_archive_integration, tag_search, enable_sharing, enable_public_sharing, enable_favicons,
		display_url, permanent_notes, search_preferences) VALUES (?, '', '', '', '', 0, 0, 0, 1, 0, '', '{}')`, id)
	return id, err
}

// TestNetscapeImportRejectsOversizedInput confirms the bound is enforced at the
// integration boundary, before any transaction writes rows.
func TestNetscapeImportRejectsOversizedInput(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, database.Config{Path: filepath.Join(t.TempDir(), "bounded.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() { _ = db.Close() }()
	user, err := auth.CreateToken(ctx, db, "owner", "limits")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	svc, err := bookmark.NewService(db)
	if err != nil {
		t.Fatalf("service: %v", err)
	}
	html := `<DL><A HREF="https://bounded.example">OK</A></DL>`
	_, err = netscape.Import(ctx, db, svc, user.UserID, strings.NewReader(html), netscape.ImportOptions{Limits: netscape.Limits{MaxBytes: 8}})
	if !errors.Is(err, netscape.ErrLimitExceeded) {
		t.Fatalf("error = %v, want ErrLimitExceeded", err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM bookmarks WHERE user_id = ?`, user.UserID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("oversized import wrote %d rows, want 0", count)
	}
}
