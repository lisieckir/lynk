package netscape

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
)

func newTestDB(t *testing.T, usernames ...string) (*sql.DB, map[string]int64, *bookmark.Service) {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, database.Config{Path: filepath.Join(t.TempDir(), "lynk.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ids := make(map[string]int64, len(usernames))
	for i, name := range usernames {
		if i == 0 {
			info, err := auth.CreateToken(ctx, db, name, "import-export-test")
			if err != nil {
				t.Fatalf("create first user %s: %v", name, err)
			}
			ids[name] = info.UserID
			continue
		}
		ids[name] = addUser(t, ctx, db, name)
	}
	svc, err := bookmark.NewService(db)
	if err != nil {
		t.Fatalf("bookmark service: %v", err)
	}
	return db, ids, svc
}

// addUser creates an additional owner because auth.CreateToken only bootstraps
// the very first user; later callers must already exist.
func addUser(t *testing.T, ctx context.Context, db *sql.DB, username string) int64 {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.ExecContext(ctx, `INSERT INTO users (username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		username, "!", now, now)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("user id: %v", err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO user_profiles (user_id, theme, bookmark_date_display, bookmark_link_target,
		web_archive_integration, tag_search, enable_sharing, enable_public_sharing, enable_favicons,
		display_url, permanent_notes, search_preferences) VALUES (?, '', '', '', '', 0, 0, 0, 1, 0, '', '{}')`, id)
	if err != nil {
		t.Fatalf("insert profile: %v", err)
	}
	return id
}

func countRows(t *testing.T, db *sql.DB, userID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM bookmarks WHERE user_id = ?`, userID).Scan(&n); err != nil {
		t.Fatalf("count bookmarks: %v", err)
	}
	return n
}

func exportHTML(t *testing.T, svc *bookmark.Service, userID int64, opts ExportOptions) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Export(context.Background(), svc, userID, &buf, opts); err != nil {
		t.Fatalf("Export: %v", err)
	}
	return buf.String()
}

func TestExportEscapesAndRoundTrips(t *testing.T) {
	_, ids, svc := newTestDB(t, "alice")
	ctx := context.Background()
	added, modified := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	original := bookmark.CreateInput{
		URL:          `https://example.com/path?a=1&b="x"`,
		Title:        `<script>alert("x")</script> é 😀`,
		Description:  `<img src=x onerror=alert(1)> & Café`,
		Tags:         []string{`tag&two`, `tag<one>`},
		DateAdded:    &added,
		DateModified: &modified,
	}
	if _, err := svc.Create(ctx, ids["alice"], original); err != nil {
		t.Fatalf("Create: %v", err)
	}
	html := exportHTML(t, svc, ids["alice"], ExportOptions{Filter: FilterAll})
	for _, forbidden := range []string{"<script>", "<img ", `&b="x"`, `tag&two`} {
		if strings.Contains(html, forbidden) {
			t.Errorf("export contains unescaped %q:\n%s", forbidden, html)
		}
	}
	for _, wanted := range []string{"&lt;script&gt;", "&lt;img ", "&amp;b=", "&lt;one&gt;"} {
		if !strings.Contains(html, wanted) {
			t.Errorf("export missing escaped %q:\n%s", wanted, html)
		}
	}
	records, err := Parse(strings.NewReader(html), Limits{})
	if err != nil {
		t.Fatalf("Parse exported HTML: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	got := records[0]
	if got.URL != original.URL || got.Title != original.Title || got.Description != original.Description {
		t.Errorf("round trip mismatch:\n URL=%q\n Title=%q\n Description=%q", got.URL, got.Title, got.Description)
	}
	if !equalStrings(got.Tags, original.Tags) {
		t.Errorf("Tags = %q, want %q", got.Tags, original.Tags)
	}
	if got.DateAdded == nil || !got.DateAdded.Equal(added) || got.DateModified == nil || !got.DateModified.Equal(modified) {
		t.Errorf("dates = %v/%v, want %v/%v", got.DateAdded, got.DateModified, added, modified)
	}
}

func TestExportDeterministicOrdering(t *testing.T) {
	_, idsA, svcA := newTestDB(t, "alice")
	_, idsB, svcB := newTestDB(t, "alice")
	ctx := context.Background()
	base := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	type row struct {
		url string
		day int
	}
	rows := []row{{"https://one.example", 1}, {"https://two.example", 2}, {"https://three.example", 3}}
	for i, r := range rows {
		when := base.Add(time.Duration(r.day) * 24 * time.Hour)
		in := bookmark.CreateInput{URL: r.url, Title: r.url, DateAdded: &when}
		if _, err := svcA.Create(ctx, idsA["alice"], in); err != nil {
			t.Fatalf("Create A %d: %v", i, err)
		}
	}
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		when := base.Add(time.Duration(r.day) * 24 * time.Hour)
		in := bookmark.CreateInput{URL: r.url, Title: r.url, DateAdded: &when}
		if _, err := svcB.Create(ctx, idsB["alice"], in); err != nil {
			t.Fatalf("Create B %d: %v", i, err)
		}
	}
	htmlA := exportHTML(t, svcA, idsA["alice"], ExportOptions{Filter: FilterAll})
	htmlB := exportHTML(t, svcB, idsB["alice"], ExportOptions{Filter: FilterAll})
	if htmlA != htmlB {
		t.Errorf("export is not deterministic:\n--- A ---\n%s\n--- B ---\n%s", htmlA, htmlB)
	}
	// Export repeatedly from the same database is byte-identical too.
	if again := exportHTML(t, svcA, idsA["alice"], ExportOptions{Filter: FilterAll}); again != htmlA {
		t.Error("repeated export differs")
	}
}

func TestExportArchiveFilterAndReservedTags(t *testing.T) {
	_, ids, svc := newTestDB(t, "alice")
	ctx := context.Background()
	mk := func(url string, archived, unread bool) {
		in := bookmark.CreateInput{URL: url, Title: url, IsArchived: archived, Unread: unread}
		if _, err := svc.Create(ctx, ids["alice"], in); err != nil {
			t.Fatalf("Create %s: %v", url, err)
		}
	}
	mk("https://active.example", false, false)
	mk("https://archived.example", true, false)
	mk("https://unread.example", false, true)

	active := exportHTML(t, svc, ids["alice"], ExportOptions{})
	if !strings.Contains(active, "active.example") || !strings.Contains(active, "unread.example") || strings.Contains(active, "archived.example") {
		t.Errorf("active filter produced wrong rows:\n%s", active)
	}
	archived := exportHTML(t, svc, ids["alice"], ExportOptions{Filter: FilterArchived})
	if !strings.Contains(archived, "archived.example") || strings.Contains(archived, "active.example") {
		t.Errorf("archived filter produced wrong rows:\n%s", archived)
	}
	all := exportHTML(t, svc, ids["alice"], ExportOptions{Filter: FilterAll})
	for _, want := range []string{"lynk:archived", "lynk:unread", "active.example", "archived.example", "unread.example"} {
		if !strings.Contains(all, want) {
			t.Errorf("full export missing %q:\n%s", want, all)
		}
	}
}

func TestExportHasNoSecretsAndBoundsRecords(t *testing.T) {
	db, ids, svc := newTestDB(t, "alice")
	ctx := context.Background()
	var prefix string
	if err := db.QueryRow(`SELECT token_prefix FROM api_tokens WHERE user_id = ? LIMIT 1`, ids["alice"]).Scan(&prefix); err != nil {
		t.Fatalf("read token prefix: %v", err)
	}
	if _, err := db.Exec(`UPDATE user_profiles SET permanent_notes = ? WHERE user_id = ?`, "PROFILE-SECRET-42", ids["alice"]); err != nil {
		t.Fatalf("set profile note: %v", err)
	}
	for _, url := range []string{"https://one.example", "https://two.example"} {
		if _, err := svc.Create(ctx, ids["alice"], bookmark.CreateInput{URL: url, Title: url}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	html := exportHTML(t, svc, ids["alice"], ExportOptions{Filter: FilterAll})
	for _, secret := range []string{prefix, "PROFILE-SECRET-42", "token_digest", "password_hash", filepath.Base("lynk.db")} {
		if strings.Contains(html, secret) {
			t.Errorf("export leaked %q:\n%s", secret, html)
		}
	}
	var buf bytes.Buffer
	err := Export(ctx, svc, ids["alice"], &buf, ExportOptions{Filter: FilterAll, MaxRecords: 1})
	if !errors.Is(err, ErrLimitExceeded) {
		t.Errorf("export limit: got %v, want ErrLimitExceeded", err)
	}
}

func TestImportAtomicDuplicateAndOwnerIsolation(t *testing.T) {
	db, ids, svc := newTestDB(t, "alice", "bob")
	ctx := context.Background()
	bad := `<DL><A HREF="https://ok.example">OK</A><A HREF="javascript:alert(1)">Bad</A></DL>`
	_, err := Import(ctx, db, svc, ids["alice"], strings.NewReader(bad), ImportOptions{})
	if !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("strict import: got %v, want ErrInvalidURL", err)
	}
	if got := countRows(t, db, ids["alice"]); got != 0 {
		t.Fatalf("strict import wrote %d rows, want 0", got)
	}
	sum, err := Import(ctx, db, svc, ids["alice"], strings.NewReader(bad), ImportOptions{ContinueOnError: true})
	if err != nil {
		t.Fatalf("ContinueOnError import: %v", err)
	}
	if sum.Created != 1 || sum.Failed != 1 || sum.Parsed != 2 {
		t.Errorf("summary = %+v, want created=1 failed=1 parsed=2", sum)
	}
	if got := countRows(t, db, ids["alice"]); got != 1 {
		t.Fatalf("import wrote %d rows, want 1", got)
	}
	if got := countRows(t, db, ids["bob"]); got != 0 {
		t.Fatalf("import wrote %d rows for another owner, want 0", got)
	}
	if _, err := svc.GetByURL(ctx, ids["bob"], "https://ok.example"); !errors.Is(err, bookmark.ErrNotFound) {
		t.Errorf("bob can see alice's bookmark: %v", err)
	}

	duplicates := `<DL><A HREF="https://dup.example">First</A><A HREF="https://dup.example">Second</A></DL>`
	sum, err = Import(ctx, db, svc, ids["alice"], strings.NewReader(duplicates), ImportOptions{})
	if err != nil {
		t.Fatalf("duplicate import: %v", err)
	}
	if sum.Created != 1 || sum.Merged != 1 {
		t.Errorf("duplicate summary = %+v, want created=1 merged=1", sum)
	}
	if got := countRows(t, db, ids["alice"]); got != 2 {
		t.Fatalf("duplicate import wrote %d rows, want 2", got)
	}
	item, err := svc.GetByURL(ctx, ids["alice"], "https://dup.example")
	if err != nil {
		t.Fatalf("GetByURL: %v", err)
	}
	if item.Title != "Second" {
		t.Errorf("merged title = %q, want %q (service merge semantics)", item.Title, "Second")
	}
}

func TestFullRoundTripAllFields(t *testing.T) {
	sourceDB, idsSource, sourceSvc := newTestDB(t, "alice")
	ctx := context.Background()
	added := time.Date(2023, 5, 6, 7, 8, 9, 0, time.UTC)
	modified := added.Add(24 * time.Hour)
	originals := []bookmark.CreateInput{
		{
			URL:          "https://one.example/work?b=2&a=1",
			Title:        "One & <Two> 😀",
			Description:  "Line one\nLine two & café",
			Tags:         []string{folderPrefix + "Work/Inbox", "alpha", "beta"},
			IsArchived:   false,
			Unread:       true,
			DateAdded:    &added,
			DateModified: &modified,
		},
		{
			URL:        "https://two.example/archive",
			Title:      "Archived",
			Tags:       []string{folderPrefix + "Archive", "lynk:archived"},
			IsArchived: true,
			DateAdded:  &added,
		},
	}
	for i, in := range originals {
		if _, err := sourceSvc.Create(ctx, idsSource["alice"], in); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}
	html := exportHTML(t, sourceSvc, idsSource["alice"], ExportOptions{Filter: FilterAll})

	targetDB, idsTarget, targetSvc := newTestDB(t, "bob", "carol")
	sum, err := Import(ctx, targetDB, targetSvc, idsTarget["bob"], strings.NewReader(html), ImportOptions{})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if sum.Created != 2 || sum.Merged != 0 || sum.Failed != 0 {
		t.Errorf("summary = %+v, want created=2", sum)
	}
	if got := countRows(t, targetDB, idsTarget["bob"]); got != 2 {
		t.Fatalf("bob has %d rows, want 2", got)
	}
	if got := countRows(t, targetDB, idsTarget["carol"]); got != 0 {
		t.Fatalf("carol has %d rows, want 0", got)
	}
	for _, want := range originals {
		item, err := targetSvc.GetByURL(ctx, idsTarget["bob"], want.URL)
		if err != nil {
			t.Fatalf("GetByURL %s: %v", want.URL, err)
		}
		if item.Title != want.Title || item.Description != want.Description {
			t.Errorf("round trip for %s: title=%q description=%q", want.URL, item.Title, item.Description)
		}
		if item.IsArchived != want.IsArchived || item.Unread != want.Unread {
			t.Errorf("round trip for %s: archived=%v unread=%v", want.URL, item.IsArchived, item.Unread)
		}
		if !item.DateAdded.Equal(*want.DateAdded) {
			t.Errorf("round trip for %s: date_added=%v want %v", want.URL, item.DateAdded, want.DateAdded)
		}
		if item.FaviconURL != nil || item.PreviewImageURL != nil {
			t.Errorf("import fetched metadata for %s: %v / %v", want.URL, item.FaviconURL, item.PreviewImageURL)
		}
	}
	wantTags := []string{"alpha", "beta", `folder:Work/Inbox`}
	item, err := targetSvc.GetByURL(ctx, idsTarget["bob"], originals[0].URL)
	if err != nil {
		t.Fatalf("GetByURL: %v", err)
	}
	if !equalStrings(item.Tags, wantTags) {
		t.Errorf("Tags = %q, want %q", item.Tags, wantTags)
	}

	again, err := Import(ctx, targetDB, targetSvc, idsTarget["bob"], strings.NewReader(html), ImportOptions{})
	if err != nil {
		t.Fatalf("second Import: %v", err)
	}
	if again.Merged != 2 || again.Created != 0 || countRows(t, targetDB, idsTarget["bob"]) != 2 {
		t.Errorf("re-import = %+v, want merged=2 with no new rows", again)
	}
	_ = sourceDB
}
