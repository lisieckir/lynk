package database

import (
	"context"
	"database/sql"
	"testing"
)

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func setupTagTest(t *testing.T) *sql.DB {
	t.Helper()
	db := openTestDB(t, testConfig(t))
	const stamp = "2026-01-02T03:04:05Z"
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, created_at, updated_at)
		VALUES (1, 'alice', 'hash', ?, ?)`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int{1, 2} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO bookmarks (
				id, user_id, url, url_normalized, title, date_added, date_modified
			) VALUES (?, 1, ?, ?, 'Tagged bookmark', ?, ?)`,
			id, "https://example.com/"+string(rune('a'+id)),
			"https://example.com/"+string(rune('a'+id)), stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func addTag(t *testing.T, ex sqlExecutor, id int, name string) {
	t.Helper()
	if _, err := ex.ExecContext(context.Background(), `
		INSERT INTO tags (id, user_id, name, date_added) VALUES (?, 1, ?, '2026-01-02T03:04:05Z')`,
		id, name); err != nil {
		t.Fatal(err)
	}
}

func linkTag(t *testing.T, ex sqlExecutor, bookmarkID, tagID int) {
	t.Helper()
	if _, err := ex.ExecContext(context.Background(), `
		INSERT INTO bookmark_tags (bookmark_id, tag_id) VALUES (?, ?)`, bookmarkID, tagID); err != nil {
		t.Fatal(err)
	}
}

func unlinkTag(t *testing.T, ex sqlExecutor, bookmarkID, tagID int) {
	t.Helper()
	if _, err := ex.ExecContext(context.Background(), `
		DELETE FROM bookmark_tags WHERE bookmark_id = ? AND tag_id = ?`, bookmarkID, tagID); err != nil {
		t.Fatal(err)
	}
}

func bookmarkTagsText(t *testing.T, db *sql.DB, bookmarkID int) string {
	t.Helper()
	var got string
	if err := db.QueryRowContext(context.Background(),
		`SELECT tags FROM bookmarks WHERE id = ?`, bookmarkID,
	).Scan(&got); err != nil {
		t.Fatal(err)
	}
	return got
}

func ftsMatchCount(t *testing.T, db *sql.DB, token string) int {
	t.Helper()
	return countRows(t, db, `SELECT count(*) FROM bookmark_search WHERE bookmark_search MATCH ?`, token)
}

func ftsRowCount(t *testing.T, db *sql.DB, rowid int) int {
	t.Helper()
	return countRows(t, db, `SELECT count(*) FROM bookmark_search WHERE rowid = ?`, rowid)
}

func assertTagsAndFTS(t *testing.T, db *sql.DB, bookmarkID int, wantTags string, wantMatches map[string]int) {
	t.Helper()
	if got := bookmarkTagsText(t, db, bookmarkID); got != wantTags {
		t.Errorf("bookmark %d tags = %q, want %q", bookmarkID, got, wantTags)
	}
	for token, want := range wantMatches {
		if got := ftsMatchCount(t, db, token); got != want {
			t.Errorf("FTS matches for %q = %d, want %d", token, got, want)
		}
	}
	if got := ftsRowCount(t, db, bookmarkID); got != 1 {
		t.Errorf("FTS rows for bookmark %d = %d, want 1 (no duplicates)", bookmarkID, got)
	}
}

func TestBookmarkTagTriggersKeepDenormalizedTagsAndFTSInSync(t *testing.T) {
	ctx := context.Background()
	db := setupTagTest(t)

	// Insert tags in deliberately non-alphabetical order.
	addTag(t, db, 1, "zeta")
	addTag(t, db, 2, "alpha")
	addTag(t, db, 3, "mid")

	linkTag(t, db, 1, 1)
	assertTagsAndFTS(t, db, 1, "zeta", map[string]int{"zeta": 1, "alpha": 0})
	linkTag(t, db, 1, 2)
	assertTagsAndFTS(t, db, 1, "alpha zeta", map[string]int{"alpha": 1, "zeta": 1})
	linkTag(t, db, 1, 3)
	assertTagsAndFTS(t, db, 1, "alpha mid zeta", map[string]int{"mid": 1, "alpha": 1, "zeta": 1})

	// Single tag deletion.
	unlinkTag(t, db, 1, 3)
	assertTagsAndFTS(t, db, 1, "alpha zeta", map[string]int{"mid": 0, "zeta": 1})

	// Replacement semantics in one transaction: old tokens disappear, new
	// tokens appear, and no duplicate FTS rows are left behind.
	addTag(t, db, 4, "beta")
	addTag(t, db, 5, "gamma")
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	unlinkTag(t, tx, 1, 2)
	unlinkTag(t, tx, 1, 1)
	linkTag(t, tx, 1, 4)
	linkTag(t, tx, 1, 5)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	assertTagsAndFTS(t, db, 1, "beta gamma", map[string]int{
		"alpha": 0, "zeta": 0, "beta": 1, "gamma": 1,
	})

	// Updating a relation moves a tag without duplicating index rows.
	addTag(t, db, 6, "delta")
	if _, err := db.ExecContext(ctx, `
		UPDATE bookmark_tags SET tag_id = 6 WHERE bookmark_id = 1 AND tag_id = 4`); err != nil {
		t.Fatal(err)
	}
	assertTagsAndFTS(t, db, 1, "delta gamma", map[string]int{
		"beta": 0, "delta": 1, "gamma": 1,
	})

	// Renaming a tag refreshes the denormalized text and FTS index.
	if _, err := db.ExecContext(ctx, `UPDATE tags SET name = 'renamed' WHERE id = 6`); err != nil {
		t.Fatal(err)
	}
	assertTagsAndFTS(t, db, 1, "gamma renamed", map[string]int{
		"delta": 0, "renamed": 1, "gamma": 1,
	})

	// Deleting a tag cascades to bookmark_tags; the child trigger recomputes
	// the parent row and FTS match disappears.
	if _, err := db.ExecContext(ctx, `DELETE FROM tags WHERE id = 5`); err != nil {
		t.Fatal(err)
	}
	assertTagsAndFTS(t, db, 1, "renamed", map[string]int{"gamma": 0, "renamed": 1})

	// A second bookmark sharing a tag is isolated from the first one.
	linkTag(t, db, 2, 6)
	if got := bookmarkTagsText(t, db, 2); got != "renamed" {
		t.Errorf("bookmark 2 tags = %q, want %q", got, "renamed")
	}
	unlinkTag(t, db, 1, 6)
	assertTagsAndFTS(t, db, 1, "", map[string]int{"renamed": 1})
	assertTagsAndFTS(t, db, 2, "renamed", map[string]int{"renamed": 1})

	// Deleting a bookmark cascades its relations and removes its FTS row.
	if _, err := db.ExecContext(ctx, `DELETE FROM bookmarks WHERE id = 2`); err != nil {
		t.Fatal(err)
	}
	if got := ftsMatchCount(t, db, "renamed"); got != 0 {
		t.Errorf("FTS matches for removed bookmark = %d, want 0", got)
	}
	if got := ftsRowCount(t, db, 2); got != 0 {
		t.Errorf("FTS rows for removed bookmark = %d, want 0", got)
	}
}
