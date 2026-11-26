package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lisieckir/lynk/internal/auth"
	"github.com/lisieckir/lynk/internal/bookmark"
	"github.com/lisieckir/lynk/internal/config"
	"github.com/lisieckir/lynk/internal/database"
)

func bookmarkLookup(t *testing.T, dir string) config.LookupFunc {
	t.Helper()
	dbPath := filepath.Join(dir, "lynk.db")
	return lookupFrom(map[string]string{
		"LYNK_DATA_DIR":      dir,
		"LYNK_DATABASE_PATH": dbPath,
		"LYNK_HTTP_ADDR":     freeHTTPAddr(t),
	})
}

func seedCLIUser(t *testing.T, dir, username string) int64 {
	t.Helper()
	db, err := database.Open(context.Background(), database.Config{Path: filepath.Join(dir, "lynk.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() { _ = db.Close() }()
	info, err := auth.CreateToken(context.Background(), db, username, "cli-test")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return info.UserID
}

func cliBookmarksCount(t *testing.T, dir string, userID int64) int {
	t.Helper()
	db, err := database.Open(context.Background(), database.Config{Path: filepath.Join(dir, "lynk.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM bookmarks WHERE user_id = ?`, userID).Scan(&n); err != nil {
		t.Fatalf("count bookmarks: %v", err)
	}
	return n
}

const cliHTML = `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<DL><p>
<DT><H3>Work</H3><DL><p>
<DT><A HREF="https://cli.example/one" ADD_DATE="1700000000" TAGS="alpha">One</A>
<DD>A note &amp; more
</DL><p>
</DL><p>
`

func TestBookmarksImportRequiresExplicitOwner(t *testing.T) {
	dir := t.TempDir()
	lookup := bookmarkLookup(t, dir)
	ctx := context.Background()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no owner", []string{"bookmarks", "import", "--file", "-"}, "one of --username or --user-id is required"},
		{"both owners", []string{"bookmarks", "import", "--file", "-", "--username", "alice", "--user-id", "1"}, "exactly one of --username or --user-id"},
		{"unknown user", []string{"bookmarks", "import", "--file", "-", "--username", "ghost"}, "not found"},
		{"unknown id", []string{"bookmarks", "import", "--file", "-", "--user-id", "999"}, "not found"},
		{"missing file", []string{"bookmarks", "import", "--username", "alice"}, "--file is required"},
		{"unknown command", []string{"bookmarks", "frobnicate"}, "unknown bookmarks command"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := runWithInput(ctx, tc.args, lookup, strings.NewReader(cliHTML), &out)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestBookmarksImportFromStdinAndFile(t *testing.T) {
	dir := t.TempDir()
	lookup := bookmarkLookup(t, dir)
	userID := seedCLIUser(t, dir, "alice")
	ctx := context.Background()

	var out bytes.Buffer
	if err := runWithInput(ctx, []string{"bookmarks", "import", "--username", "alice", "--file", "-"}, lookup, strings.NewReader(cliHTML), &out); err != nil {
		t.Fatalf("stdin import: %v", err)
	}
	if want := "import: parsed=1 created=1 merged=0 skipped=0 failed=0\n"; out.String() != want {
		t.Errorf("summary = %q, want %q", out.String(), want)
	}
	if got := cliBookmarksCount(t, dir, userID); got != 1 {
		t.Fatalf("bookmarks = %d, want 1", got)
	}

	source := filepath.Join(dir, "source.html")
	if err := os.WriteFile(source, []byte(cliHTML), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	out.Reset()
	if err := runWithInput(ctx, []string{"bookmarks", "import", "--user-id", "1", "--file", source}, lookup, strings.NewReader(""), &out); err != nil {
		t.Fatalf("file import: %v", err)
	}
	after, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read source after import: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("import overwrote its source file")
	}
	if got := cliBookmarksCount(t, dir, userID); got != 1 {
		t.Fatalf("duplicate import created rows: %d, want 1", got)
	}
	if !strings.Contains(out.String(), "merged=1") {
		t.Errorf("summary = %q, want merged=1", out.String())
	}
}

func TestBookmarksImportMalformedIsAtomic(t *testing.T) {
	dir := t.TempDir()
	lookup := bookmarkLookup(t, dir)
	userID := seedCLIUser(t, dir, "alice")
	var out bytes.Buffer
	bad := `<DL><A HREF="https://ok.example">OK</A><A>missing href</A></DL>`
	err := runWithInput(context.Background(), []string{"bookmarks", "import", "--username", "alice", "--file", "-"}, lookup, strings.NewReader(bad), &out)
	if err == nil || !strings.Contains(err.Error(), "malformed HTML") {
		t.Fatalf("error = %v, want malformed HTML", err)
	}
	if got := cliBookmarksCount(t, dir, userID); got != 0 {
		t.Fatalf("malformed import wrote %d rows, want 0", got)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty on failure", out.String())
	}
}

func TestBookmarksImportLimits(t *testing.T) {
	dir := t.TempDir()
	lookup := bookmarkLookup(t, dir)
	seedCLIUser(t, dir, "alice")
	ctx := context.Background()
	base := []string{"bookmarks", "import", "--username", "alice", "--file", "-"}
	for _, tc := range []struct {
		flag string
		want string
	}{
		{"--max-bytes=0", "--max-bytes must be between"},
		{"--max-bookmarks=0", "--max-bookmarks must be between"},
		{"--max-bytes=99999999999", "--max-bytes must be between"},
	} {
		t.Run(tc.flag, func(t *testing.T) {
			var out bytes.Buffer
			err := runWithInput(ctx, append(append([]string{}, base...), tc.flag), lookup, strings.NewReader(cliHTML), &out)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	var out bytes.Buffer
	err := runWithInput(ctx, append(append([]string{}, base...), "--max-bytes=8"), lookup, strings.NewReader(cliHTML), &out)
	if err == nil || !strings.Contains(err.Error(), "limit exceeded") {
		t.Fatalf("byte limit error = %v, want limit exceeded", err)
	}
}

func TestBookmarksExportStdoutAndFile(t *testing.T) {
	dir := t.TempDir()
	lookup := bookmarkLookup(t, dir)
	userID := seedCLIUser(t, dir, "alice")
	ctx := context.Background()
	db, err := database.Open(ctx, database.Config{Path: filepath.Join(dir, "lynk.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	svc, err := bookmark.NewService(db)
	if err != nil {
		t.Fatalf("service: %v", err)
	}
	if _, err := svc.Create(ctx, userID, bookmark.CreateInput{URL: "https://export.example/x?a=1&b=2", Title: "<b>Title</b>", Description: "note & more"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Writing to stdout emits only the document: no summary line can corrupt it.
	var stdout bytes.Buffer
	if err := runWithInput(ctx, []string{"bookmarks", "export", "--username", "alice", "--file", "-"}, lookup, strings.NewReader(""), &stdout); err != nil {
		t.Fatalf("stdout export: %v", err)
	}
	html := stdout.String()
	if !strings.HasPrefix(html, "<!DOCTYPE NETSCAPE-Bookmark-file-1>\n") || !strings.HasSuffix(html, "</DL><p>\n") {
		t.Errorf("stdout is not a complete Netscape document:\n%s", html)
	}
	if strings.Contains(html, "export:") || strings.Contains(html, "<b>Title</b>") {
		t.Errorf("stdout export has a summary or unescaped title:\n%s", html)
	}

	dest := filepath.Join(dir, "export.html")
	stdout.Reset()
	if err := runWithInput(ctx, []string{"bookmarks", "export", "--user-id", "1", "--file", dest}, lookup, strings.NewReader(""), &stdout); err != nil {
		t.Fatalf("file export: %v", err)
	}
	if want := "export: filter=active bookmarks=1\n"; stdout.String() != want {
		t.Errorf("summary = %q, want %q", stdout.String(), want)
	}
	contents, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if string(contents) != html {
		t.Errorf("file export differs from stdout export:\n%s", contents)
	}

	// No-clobber is explicit and the existing file is left byte-identical.
	stdout.Reset()
	err = runWithInput(ctx, []string{"bookmarks", "export", "--username", "alice", "--file", dest}, lookup, strings.NewReader(""), &stdout)
	if err == nil || !strings.Contains(err.Error(), "--overwrite") {
		t.Fatalf("clobber error = %v, want --overwrite hint", err)
	}
	again, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read export after refusal: %v", err)
	}
	if !bytes.Equal(contents, again) {
		t.Error("failed export modified existing destination")
	}
	stdout.Reset()
	if err := runWithInput(ctx, []string{"bookmarks", "export", "--username", "alice", "--file", dest, "--overwrite"}, lookup, strings.NewReader(""), &stdout); err != nil {
		t.Fatalf("overwrite export: %v", err)
	}

	// An export destination pointing at the database is always refused.
	err = runWithInput(ctx, []string{"bookmarks", "export", "--username", "alice", "--file", filepath.Join(dir, "lynk.db"), "--overwrite"}, lookup, strings.NewReader(""), &stdout)
	if err == nil || !strings.Contains(err.Error(), "must not be the database file") {
		t.Fatalf("database destination error = %v", err)
	}
}

func TestUnknownTopLevelCommandDoesNotStartServer(t *testing.T) {
	dir := t.TempDir()
	lookup := bookmarkLookup(t, dir)
	for _, command := range []string{"bookmark", "bogus"} {
		t.Run(command, func(t *testing.T) {
			var out bytes.Buffer
			err := runWithInput(context.Background(), []string{command, "export", "--file", "-"}, lookup, strings.NewReader(""), &out)
			if err == nil || !strings.Contains(err.Error(), "unknown command") {
				t.Fatalf("error = %v, want unknown command error", err)
			}
			if command == "bookmark" && !strings.Contains(err.Error(), `did you mean "bookmarks"?`) {
				t.Errorf("error = %v, want a suggestion for the singular typo", err)
			}
			if strings.Contains(out.String(), "configuration loaded") {
				t.Errorf("unknown command started the server: %s", out.String())
			}
		})
	}
}

func dbBytes(t *testing.T, dbPath string) []byte {
	t.Helper()
	contents, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read database: %v", err)
	}
	return contents
}

func dbSchema(t *testing.T, dir string) string {
	t.Helper()
	db, err := database.Open(context.Background(), database.Config{Path: filepath.Join(dir, "lynk.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() { _ = db.Close() }()
	var schema strings.Builder
	rows, err := db.Query(`SELECT type, name, sql FROM sqlite_master ORDER BY type, name`)
	if err != nil {
		t.Fatalf("query schema: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var kind, name string
		var sqlText *string
		if err := rows.Scan(&kind, &name, &sqlText); err != nil {
			t.Fatalf("scan schema: %v", err)
		}
		fmt.Fprintf(&schema, "%s:%s:%s\n", kind, name, stringValue(sqlText))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("schema rows: %v", err)
	}
	var integrity, count string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		t.Fatalf("integrity: %v", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM bookmarks`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	fmt.Fprintf(&schema, "integrity=%s bookmarks=%s\n", integrity, count)
	return schema.String()
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func TestBookmarksExportRefusesDatabaseAliases(t *testing.T) {
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	if err := os.Mkdir(realDir, 0o700); err != nil {
		t.Fatalf("mkdir real: %v", err)
	}
	dbPath := filepath.Join(realDir, "lynk.db")
	lookup := lookupFrom(map[string]string{
		"LYNK_DATA_DIR":      realDir,
		"LYNK_DATABASE_PATH": dbPath,
		"LYNK_HTTP_ADDR":     freeHTTPAddr(t),
	})
	userID := seedCLIUser(t, realDir, "alice")
	db, err := database.Open(context.Background(), database.Config{Path: dbPath})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	svc, err := bookmark.NewService(db)
	if err != nil {
		t.Fatalf("service: %v", err)
	}
	if _, err := svc.Create(context.Background(), userID, bookmark.CreateInput{URL: "https://alias.example", Title: "Alias"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	aliasDir := filepath.Join(dir, "alias")
	if err := os.Symlink(realDir, aliasDir); err != nil {
		t.Fatalf("symlink parent: %v", err)
	}
	beforeBytes, beforeSchema := dbBytes(t, dbPath), dbSchema(t, realDir)

	// A symlinked parent makes these spellings look harmless; canonical
	// comparison must still reject the database and every sidecar, with and
	// without --overwrite.
	for _, name := range []string{"lynk.db", "lynk.db-wal", "lynk.db-shm", "lynk.db-journal"} {
		for _, overwrite := range []bool{false, true} {
			t.Run(name+fmt.Sprintf("/overwrite=%v", overwrite), func(t *testing.T) {
				args := []string{"bookmarks", "export", "--username", "alice", "--file", filepath.Join(aliasDir, name)}
				if overwrite {
					args = append(args, "--overwrite")
				}
				var out bytes.Buffer
				err := runWithInput(context.Background(), args, lookup, strings.NewReader(""), &out)
				if err == nil || !strings.Contains(err.Error(), "must not be the database file or its sidecars") {
					t.Fatalf("error = %v, want database alias refusal", err)
				}
			})
		}
	}

	// A hard link is another alias whose spelling is unrelated to the DB path.
	hardLink := filepath.Join(realDir, "hardlink.db")
	if err := os.Link(dbPath, hardLink); err != nil {
		t.Fatalf("hard link: %v", err)
	}
	var out bytes.Buffer
	err = runWithInput(context.Background(), []string{"bookmarks", "export", "--username", "alice", "--file", hardLink, "--overwrite"}, lookup, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "aliases the database") {
		t.Fatalf("hard link error = %v, want alias refusal", err)
	}

	// An existing symlink destination is rejected even when it points at an
	// unrelated file, and that file is left untouched.
	target := filepath.Join(dir, "target.html")
	if err := os.WriteFile(target, []byte("ORIGINAL"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(dir, "link.html")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink destination: %v", err)
	}
	err = runWithInput(context.Background(), []string{"bookmarks", "export", "--username", "alice", "--file", link, "--overwrite"}, lookup, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("symlink destination error = %v", err)
	}
	if contents, err := os.ReadFile(target); err != nil || string(contents) != "ORIGINAL" {
		t.Errorf("symlink target changed: %q (%v)", contents, err)
	}

	// A symlinked parent is still allowed for a safe, new destination.
	out.Reset()
	if err := runWithInput(context.Background(), []string{"bookmarks", "export", "--username", "alice", "--file", filepath.Join(aliasDir, "safe.html")}, lookup, strings.NewReader(""), &out); err != nil {
		t.Fatalf("safe alias export: %v", err)
	}
	if contents, err := os.ReadFile(filepath.Join(realDir, "safe.html")); err != nil || !strings.HasPrefix(string(contents), "<!DOCTYPE NETSCAPE-Bookmark-file-1>") {
		t.Errorf("safe alias export missing: %q (%v)", contents, err)
	}

	if !bytes.Equal(beforeBytes, dbBytes(t, dbPath)) {
		t.Error("database bytes changed during refused exports")
	}
	if got := dbSchema(t, realDir); got != beforeSchema {
		t.Errorf("database schema changed:\nbefore:\n%s\nafter:\n%s", beforeSchema, got)
	}
}

func TestBookmarksExportOptionsErrors(t *testing.T) {
	dir := t.TempDir()
	lookup := bookmarkLookup(t, dir)
	seedCLIUser(t, dir, "alice")
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"missing file", []string{"bookmarks", "export", "--username", "alice"}, "--file is required"},
		{"bad filter", []string{"bookmarks", "export", "--username", "alice", "--file", "-", "--archived", "everything"}, "--archived must be active"},
		{"bad limit", []string{"bookmarks", "export", "--username", "alice", "--file", "-", "--max-records=0"}, "--max-records must be between"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := runWithInput(context.Background(), tc.args, lookup, strings.NewReader(""), &out)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
