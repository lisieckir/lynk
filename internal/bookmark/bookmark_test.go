package bookmark

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/lisieckir/lynk/internal/database"
)

func testService(t *testing.T) (*Service, *sql.DB) {
	return testServiceWithPool(t, 4)
}

func testServiceWithPool(t *testing.T, maxOpen int) (*Service, *sql.DB) {
	t.Helper()
	db, err := database.Open(context.Background(), database.Config{Path: filepath.Join(t.TempDir(), "test.db"), MaxOpenConns: maxOpen, MaxIdleConns: maxOpen})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Format(time.RFC3339Nano)
	for id, name := range map[int]string{1: "alice", 2: "bob"} {
		if _, err := db.Exec(`INSERT INTO users(id,username,password_hash,created_at,updated_at) VALUES(?,?, 'hash', ?,?)`, id, name, now, now); err != nil {
			t.Fatal(err)
		}
	}
	s, err := NewService(db)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC) }
	return s, db
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{" HTTPS://Example.COM:443/path///?q=1#section ", "https://example.com:443/path?q=1#section"},
		{"http://EXAMPLE.com:80/a/", "http://example.com:80/a"},
		{"http://EXAMPLE.com:0/a", "http://example.com/a"},
		{"https://example.com:8443/a", "https://example.com:8443/a"},
		{"https://USER:PASS@Example.COM/a///?z=hello+world&a=&a=2#Frag", "https://USER:PASS@example.com/a?a=&a=2&z=hello%20world#Frag"},
		{"https://user:@Example.com/a", "https://user@example.com/a"},
		{"https://:pass@Example.com/a", "https://example.com/a"},
		{"https://example.com/?b=two%2Fthree&a=one", "https://example.com?a=one&b=two/three"},
		{"https://example.com/?a=%ZZ", "https://example.com?a=%25ZZ"},
		{"https://example.com/a;b///?x=1", "https://example.com/a;b?x=1"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := NormalizeURL(tt.in)
			if err != nil || got != tt.want {
				t.Fatalf("NormalizeURL()=(%q,%v), want %q", got, err, tt.want)
			}
		})
	}
}

func TestValidateURLSchemes(t *testing.T) {
	tests := []struct {
		url string
		ok  bool
	}{
		{"http://example.com", true},
		{"https://example.com", true},
		{"ftp://example.com/file", true},
		{"ftps://example.com/file", true},
		{"javascript:alert(1)", false},
		{"data:text/plain,hello", false},
		{"file:///tmp/file", false},
		{"https:///missing-host", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err == nil) != tt.ok {
				t.Fatalf("ValidateURL(%q) error=%v, want accepted=%v", tt.url, err, tt.ok)
			}
		})
	}
}

func TestCreateMergeOwnershipAndCollision(t *testing.T) {
	s, _ := testService(t)
	ctx := context.Background()
	first, err := s.Create(ctx, 1, CreateInput{URL: "HTTPS://Example.com:443/x#one", Title: "first", Tags: []string{"one"}})
	if err != nil {
		t.Fatal(err)
	}
	merged, err := s.Create(ctx, 1, CreateInput{URL: "https://example.com:443/x#one", Title: "merged", Tags: []string{"two"}})
	if err != nil {
		t.Fatal(err)
	}
	if merged.ID != first.ID || merged.Title != "merged" || len(merged.Tags) != 1 || merged.Tags[0] != "two" {
		t.Fatalf("duplicate create did not merge: %+v", merged)
	}
	other, err := s.Create(ctx, 2, CreateInput{URL: "https://example.com/x", Title: "other"})
	if err != nil {
		t.Fatal(err)
	}
	if other.ID == merged.ID {
		t.Fatal("different users shared bookmark")
	}
	collision, err := s.Create(ctx, 1, CreateInput{URL: "https://example.com/other"})
	if err != nil {
		t.Fatal(err)
	}
	newURL := "https://example.com:443/x#one"
	_, err = s.Update(ctx, 1, collision.ID, UpdateInput{URL: &newURL})
	if !errors.Is(err, ErrURLConflict) {
		t.Fatalf("Update collision error = %v", err)
	}
	if _, err = s.Get(ctx, 2, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user get error = %v", err)
	}
}

func TestPatchTagReplacementFTSRollbackArchiveDelete(t *testing.T) {
	s, db := testService(t)
	ctx := context.Background()
	b, err := s.Create(ctx, 1, CreateInput{URL: "https://example.com", Title: "hello", Tags: []string{"old", "keep"}})
	if err != nil {
		t.Fatal(err)
	}
	title := "updated"
	badTags := []string{"new", ""}
	_, err = s.Patch(ctx, 1, b.ID, UpdateInput{Title: &title, Tags: &badTags})
	if !errors.Is(err, ErrInvalidBookmark) {
		t.Fatalf("invalid patch error=%v", err)
	}
	got, err := s.Get(ctx, 1, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "hello" || len(got.Tags) != 2 {
		t.Fatalf("failed transaction partially changed row: %+v", got)
	}
	tags := []string{"new"}
	got, err = s.Patch(ctx, 1, b.ID, UpdateInput{Title: &title, Tags: &tags})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "new" {
		t.Fatalf("replacement tags=%v", got.Tags)
	}
	var tagsText string
	if err := db.QueryRow(`SELECT tags FROM bookmarks WHERE id=?`, b.ID).Scan(&tagsText); err != nil {
		t.Fatal(err)
	}
	if tagsText != "new" {
		t.Fatalf("denormalized tags=%q", tagsText)
	}
	var matches int
	if err := db.QueryRow(`SELECT count(*) FROM bookmark_search WHERE bookmark_search MATCH 'new'`).Scan(&matches); err != nil {
		t.Fatal(err)
	}
	if matches != 1 {
		t.Fatalf("FTS matches=%d", matches)
	}
	if err := s.Archive(ctx, 1, b.ID); err != nil {
		t.Fatal(err)
	}
	archived, err := s.Get(ctx, 1, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !archived.IsArchived || !archived.DateModified.Equal(s.now()) {
		t.Fatalf("archive state/timestamp: %+v", archived)
	}
	if rows, err := s.List(ctx, 1, true); err != nil || len(rows) != 1 {
		t.Fatalf("archived list=%v, err=%v", rows, err)
	}
	if err := s.Unarchive(ctx, 1, b.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, 1, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, 1, b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted get=%v", err)
	}
}

func TestCanceledContext(t *testing.T) {
	s, _ := testService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.Create(ctx, 1, CreateInput{URL: "https://example.com"})
	if err == nil {
		t.Fatal("Create with canceled context succeeded")
	}
}

func TestListWithSingleConnectionAndTaggedBookmark(t *testing.T) {
	s, _ := testServiceWithPool(t, 1)
	if _, err := s.Create(context.Background(), 1, CreateInput{
		URL: "https://example.com", Tags: []string{"tag"},
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	bookmarks, err := s.List(ctx, 1, false)
	if err != nil {
		t.Fatalf("List with MaxOpenConns=1: %v", err)
	}
	if len(bookmarks) != 1 || len(bookmarks[0].Tags) != 1 || bookmarks[0].Tags[0] != "tag" {
		t.Fatalf("List returned %+v", bookmarks)
	}
}
