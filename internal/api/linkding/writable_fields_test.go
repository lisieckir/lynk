package linkding_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestWritableArchiveAndDates covers the P2-005 field parity: is_archived,
// date_added and date_modified are accepted, validated and persisted by
// create, PUT and PATCH the way Linkding's BookmarkSerializer treats them.
func TestWritableArchiveAndDates(t *testing.T) {
	f := newFixture(t)

	// Create carries all three fields and the response must echo them.
	created := createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url":           "https://writable.example/a",
		"title":         "Writable",
		"is_archived":   true,
		"date_added":    "2001-02-03T04:05:06Z",
		"date_modified": "2001-02-04T05:06:07.123456Z",
	})
	if created["is_archived"] != true {
		t.Errorf("create is_archived = %v, want true", created["is_archived"])
	}
	if created["date_added"] != "2001-02-03T04:05:06Z" {
		t.Errorf("create date_added = %v, want submitted value", created["date_added"])
	}
	if created["date_modified"] != "2001-02-04T05:06:07.123456Z" {
		t.Errorf("create date_modified = %v, want submitted value", created["date_modified"])
	}
	id := int64(created["id"].(float64))
	path := fmt.Sprintf("/api/bookmarks/%d/", id)

	// The values are persisted, not just echoed: a detail read returns them,
	// and an archived create lands in the archived list only.
	rec := request(t, f.handler, http.MethodGet, path, token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	if got := decodeObject(t, rec); got["is_archived"] != true || got["date_added"] != "2001-02-03T04:05:06Z" {
		t.Errorf("GET detail = %v, want persisted archive state and date_added", got)
	}
	rec = request(t, f.handler, http.MethodGet, "/api/bookmarks/", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	if page := decodeObject(t, rec); page["count"] != float64(0) {
		t.Errorf("active list count = %v, want 0 for a bookmark created archived", page["count"])
	}
	rec = request(t, f.handler, http.MethodGet, "/api/bookmarks/archived/", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	if page := decodeObject(t, rec); page["count"] != float64(1) {
		t.Errorf("archived list count = %v, want 1", page["count"])
	}

	// PUT rewrites is_archived and date_added; date_modified defaults to the
	// current time when the client does not send it.
	rec = request(t, f.handler, http.MethodPut, path, token(f.aliceToken), map[string]any{
		"url":         "https://writable.example/a",
		"is_archived": false,
		"date_added":  "2002-03-04T05:06:07Z",
	})
	assertStatus(t, rec, http.StatusOK)
	updated := decodeObject(t, rec)
	if updated["is_archived"] != false || updated["date_added"] != "2002-03-04T05:06:07Z" {
		t.Errorf("PUT response = %v, want unarchived with the new date_added", updated)
	}
	putModified := mustParseStamp(t, updated["date_modified"])
	if putModified.Before(time.Now().Add(-24 * time.Hour)) {
		t.Errorf("PUT date_modified = %v, want the server clock", updated["date_modified"])
	}

	// PATCH is partial: date_added survives an unrelated patch, and an
	// explicitly supplied date_modified is persisted as sent (see
	// docs/api-compatibility.md for the difference from Linkding's update
	// service, which restamps date_modified with the server clock).
	rec = request(t, f.handler, http.MethodPatch, path, token(f.aliceToken), map[string]any{
		"title":         "Patched title",
		"date_modified": "2003-04-05T06:07:08Z",
	})
	assertStatus(t, rec, http.StatusOK)
	patched := decodeObject(t, rec)
	if patched["title"] != "Patched title" || patched["date_added"] != "2002-03-04T05:06:07Z" ||
		patched["is_archived"] != false {
		t.Errorf("PATCH response = %v, want only title/date_modified changed", patched)
	}
	if patched["date_modified"] != "2003-04-05T06:07:08Z" {
		t.Errorf("PATCH date_modified = %v, want the submitted value", patched["date_modified"])
	}
	rec = request(t, f.handler, http.MethodGet, path, token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	if got := decodeObject(t, rec); got["date_modified"] != "2003-04-05T06:07:08Z" {
		t.Errorf("GET after PATCH date_modified = %v, want persisted value", got["date_modified"])
	}
}

// TestWritableDatesRejectMalformedValues proves the date fields are validated
// before the database is touched and that a rejected write leaves the stored
// row unchanged.
func TestWritableDatesRejectMalformedValues(t *testing.T) {
	f := newFixture(t)
	created := createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url": "https://dates.example/a", "title": "Dates",
		"date_added": "2001-02-03T04:05:06Z",
	})
	path := fmt.Sprintf("/api/bookmarks/%d/", int64(created["id"].(float64)))

	cases := []struct {
		name, method, target, body string
		field                      string
	}{
		{"create bad date_added", http.MethodPost, "/api/bookmarks/",
			`{"url":"https://dates.example/b","date_added":"yesterday"}`, "date_added"},
		{"create bad date_modified", http.MethodPost, "/api/bookmarks/",
			`{"url":"https://dates.example/b","date_modified":"2001-02-03"}`, "date_modified"},
		{"create blank date_added", http.MethodPost, "/api/bookmarks/",
			`{"url":"https://dates.example/b","date_added":""}`, "date_added"},
		{"create date_added type", http.MethodPost, "/api/bookmarks/",
			`{"url":"https://dates.example/b","date_added":1700000000}`, "date_added"},
		{"create is_archived type", http.MethodPost, "/api/bookmarks/",
			`{"url":"https://dates.example/b","is_archived":"yes"}`, "is_archived"},
		{"put bad date_added", http.MethodPut, path,
			`{"url":"https://dates.example/a","date_added":"not-a-date"}`, "date_added"},
		{"patch bad date_modified", http.MethodPatch, path,
			`{"date_modified":"03/04/2003"}`, "date_modified"},
		{"patch bad is_archived", http.MethodPatch, path,
			`{"is_archived":1}`, "is_archived"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := request(t, f.handler, tc.method, tc.target, token(f.aliceToken), tc.body)
			assertStatus(t, rec, http.StatusBadRequest)
			body := decodeObject(t, rec)
			messages, ok := body[tc.field].([]any)
			if !ok || len(messages) == 0 {
				t.Fatalf("error = %v, want field-keyed %q", body, tc.field)
			}
			if _, leaked := body["url"]; tc.field != "url" && leaked {
				t.Errorf("error = %v, want no unrelated url field", body)
			}
		})
	}

	// Every rejected write left the stored row untouched.
	rec := request(t, f.handler, http.MethodGet, path, token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	got := decodeObject(t, rec)
	if got["date_added"] != "2001-02-03T04:05:06Z" || got["is_archived"] != false || got["title"] != "Dates" {
		t.Errorf("row after rejected writes = %v, want the original values", got)
	}

	// A JSON null date is treated as "not supplied": the server clock fills in.
	created = createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url": "https://dates.example/null", "date_added": nil, "date_modified": nil,
	})
	mustParseStamp(t, created["date_added"])
	mustParseStamp(t, created["date_modified"])
}

// TestDuplicateCreatePreservesArchiveAndDates mirrors Linkding's merge: a
// repeat POST overwrites title/description/notes/flags but keeps the stored
// archive state and date_added while bumping date_modified to now.
func TestDuplicateCreatePreservesArchiveAndDates(t *testing.T) {
	f := newFixture(t)
	created := createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url": "https://merge.example/a", "title": "Original",
		"is_archived": true, "date_added": "2001-02-03T04:05:06Z",
	})
	merged := createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url":           "https://merge.example/a",
		"title":         "Edited by extension",
		"is_archived":   false,
		"date_added":    "2010-01-01T00:00:00Z",
		"date_modified": "2010-01-02T00:00:00Z",
	})
	if merged["id"] != created["id"] {
		t.Fatalf("merge id = %v, want %v", merged["id"], created["id"])
	}
	if merged["title"] != "Edited by extension" {
		t.Errorf("merged title = %v", merged["title"])
	}
	if merged["is_archived"] != true || merged["date_added"] != "2001-02-03T04:05:06Z" {
		t.Errorf("merge = %v, want the stored is_archived/date_added preserved", merged)
	}
	if merged["date_modified"] == "2010-01-02T00:00:00Z" {
		t.Errorf("merge date_modified = %v, want the server clock", merged["date_modified"])
	}
	if stamp := mustParseStamp(t, merged["date_modified"]); stamp.Before(time.Now().Add(-time.Hour)) {
		t.Errorf("merge date_modified = %v, want a fresh timestamp", merged["date_modified"])
	}
}

func mustParseStamp(t *testing.T, value any) time.Time {
	t.Helper()
	raw, ok := value.(string)
	if !ok {
		t.Fatalf("date = %T, want string", value)
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		t.Fatalf("date %q does not parse: %v", raw, err)
	}
	return parsed
}
