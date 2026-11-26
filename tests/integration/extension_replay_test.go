// Package integration replays the HTTP contract of the official
// linkding-extension at pinned revision
// d0221d9c5271245217d06e76c49b86a70b0fedb9 (see docs/api-compatibility.md)
// against a real migrated SQLite database driven through httptest.
//
// The replayed flows are the extension's own request sequence: popup
// initialization (profile, tags?limit=5000, check with ignore_cache and the
// bookmark detail), save and duplicate merge with
// disable_scraping/disable_html_snapshot, omnibox search with q&limit=5, the
// limit=1 connection test and delete. No network, metadata fetcher, SingleFile
// asset or browser is involved: every request is replayed in-process.
package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/lisieckir/lynk/internal/api/linkding"
	"github.com/lisieckir/lynk/internal/auth"
	"github.com/lisieckir/lynk/internal/database"
)

// bookmarkShapeKeys is the stable BookmarkSerializer field set the extension
// parses; a missing key breaks the popup, an extra key is a contract leak.
var bookmarkShapeKeys = []string{
	"id", "url", "title", "description", "notes", "web_archive_snapshot_url",
	"favicon_url", "preview_image_url", "is_archived", "unread", "shared",
	"tag_names", "date_added", "date_modified", "website_title", "website_description",
}

var profileShapeKeys = []string{
	"theme", "bookmark_date_display", "bookmark_link_target",
	"web_archive_integration", "tag_search", "enable_sharing",
	"enable_public_sharing", "enable_favicons", "display_url",
	"permanent_notes", "search_preferences", "version",
}

type fixture struct {
	handler http.Handler
	token   string
}

// newFixture opens a fresh, fully migrated SQLite database in a temporary
// directory and returns a handler for the authenticated extension session.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, database.Config{Path: filepath.Join(t.TempDir(), "replay.db")})
	if err != nil {
		t.Fatalf("open real migrated SQLite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	created, err := auth.CreateToken(ctx, db, "alice", "extension-replay")
	if err != nil {
		t.Fatalf("create extension token: %v", err)
	}
	return &fixture{
		handler: linkding.NewHandler(db, nil),
		token:   "Token " + created.Token,
	}
}

// replay sends one request exactly as the pinned extension does, including the
// Authorization header, and returns the recorded response.
func replay(t *testing.T, f *fixture, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return replayWithAuth(t, f.handler, method, path, f.token, body)
}

func replayWithAuth(t *testing.T, h http.Handler, method, path, authz string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// save replays POST /api/bookmarks/... and asserts the only status the
// extension accepts from a save: 201.
func save(t *testing.T, f *fixture, path string, payload map[string]any) map[string]any {
	t.Helper()
	rec := replay(t, f, http.MethodPost, path, payload)
	wantStatus(t, rec, http.StatusCreated)
	return decode(t, rec)
}

func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body.String())
	}
	if want == http.StatusNoContent {
		return
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode JSON %q: %v", rec.Body.String(), err)
	}
	return out
}

func wantKeys(t *testing.T, obj map[string]any, keys ...string) {
	t.Helper()
	got := make([]string, 0, len(obj))
	for key := range obj {
		got = append(got, key)
	}
	want := append([]string(nil), keys...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("JSON keys = %v, want %v", got, want)
	}
}

func pageResults(t *testing.T, page map[string]any) []any {
	t.Helper()
	results, ok := page["results"].([]any)
	if !ok {
		t.Fatalf("results = %T, want array", page["results"])
	}
	return results
}

func wantBookmarkShape(t *testing.T, obj map[string]any) {
	t.Helper()
	wantKeys(t, obj, bookmarkShapeKeys...)
}

// TestExtensionPopupInitializationReplay replays the popup startup sequence:
// profile, tags?limit=5000, check with ignore_cache, then the detail GET the
// extension performs when the checked bookmark carries no date_added.
func TestExtensionPopupInitializationReplay(t *testing.T) {
	f := newFixture(t)

	// Every extension request carries its token; without one the API answers
	// 401 before any flow runs.
	rec := replayWithAuth(t, f.handler, http.MethodGet, "/api/user/profile/", "", nil)
	wantStatus(t, rec, http.StatusUnauthorized)

	saved := save(t, f, "/api/bookmarks/?disable_scraping", map[string]any{
		"url":         "https://replay.example/article",
		"title":       "Replay article",
		"description": "Pinned contract",
		"tag_names":   []string{"replay", "alpha"},
	})
	wantBookmarkShape(t, saved)

	// 1. GET /api/user/profile/ — background refresh of the cached profile.
	rec = replay(t, f, http.MethodGet, "/api/user/profile/", nil)
	wantStatus(t, rec, http.StatusOK)
	profile := decode(t, rec)
	wantKeys(t, profile, profileShapeKeys...)
	if profile["version"] != "dev" {
		t.Errorf("profile version = %v, want dev", profile["version"])
	}
	if profile["tag_search"] != "strict" {
		t.Errorf("profile tag_search = %v, want string strict", profile["tag_search"])
	}

	// 2. GET /api/tags/?limit=5000 — the whole tag set in one page.
	rec = replay(t, f, http.MethodGet, "/api/tags/?limit=5000", nil)
	wantStatus(t, rec, http.StatusOK)
	tags := decode(t, rec)
	wantKeys(t, tags, "count", "next", "previous", "results")
	if tags["count"] != float64(2) || tags["next"] != nil || tags["previous"] != nil {
		t.Fatalf("tags page = %v, want count 2 with null links", tags)
	}
	names := make([]string, 0, 2)
	for _, raw := range pageResults(t, tags) {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("tag = %T, want object", raw)
		}
		wantKeys(t, item, "id", "name", "date_added")
		names = append(names, item["name"].(string))
	}
	if !reflect.DeepEqual(names, []string{"alpha", "replay"}) {
		t.Errorf("tag names = %v, want alpha, replay", names)
	}

	// 3. GET /api/bookmarks/check/?url=...&ignore_cache=true — the flag is
	// accepted and the shape stays complete and stable.
	checkPath := "/api/bookmarks/check/?url=" +
		url.QueryEscape("HTTPS://REPLAY.example/article/") + "&ignore_cache=true"
	rec = replay(t, f, http.MethodGet, checkPath, nil)
	wantStatus(t, rec, http.StatusOK)
	result := decode(t, rec)
	wantKeys(t, result, "bookmark", "metadata", "auto_tags")

	checked, ok := result["bookmark"].(map[string]any)
	if !ok {
		t.Fatalf("bookmark = %v, want the stored row", result["bookmark"])
	}
	wantBookmarkShape(t, checked)
	if checked["id"] != saved["id"] || checked["url"] != saved["url"] {
		t.Errorf("checked bookmark = %v, want %v", checked, saved)
	}
	added, ok := checked["date_added"].(string)
	if !ok {
		t.Fatalf("date_added = %T, want string", checked["date_added"])
	}
	if _, err := time.Parse(time.RFC3339Nano, added); err != nil {
		t.Errorf("date_added = %q does not parse: %v", added, err)
	}
	metadata, ok := result["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata = %T, want object", result["metadata"])
	}
	wantKeys(t, metadata, "url", "title", "description", "preview_image")
	if metadata["url"] != "HTTPS://REPLAY.example/article/" {
		t.Errorf("metadata url = %v, want the requested URL", metadata["url"])
	}
	if metadata["title"] != nil || metadata["description"] != nil || metadata["preview_image"] != nil {
		t.Errorf("metadata = %v, want nulls: there is no metadata fetcher", metadata)
	}
	if !reflect.DeepEqual(result["auto_tags"], []any{}) {
		t.Errorf("auto_tags = %v, want []", result["auto_tags"])
	}

	// The same check without ignore_cache answers identically apart from the
	// echoed metadata.url, so the flag cannot change the shape the popup parses.
	rec = replay(t, f, http.MethodGet,
		"/api/bookmarks/check/?url="+url.QueryEscape("https://replay.example/article/"), nil)
	wantStatus(t, rec, http.StatusOK)
	plain := decode(t, rec)
	wantKeys(t, plain, "bookmark", "metadata", "auto_tags")
	if !reflect.DeepEqual(plain["bookmark"], result["bookmark"]) ||
		!reflect.DeepEqual(plain["auto_tags"], result["auto_tags"]) {
		t.Errorf("check without ignore_cache = %v, want the same bookmark and auto_tags as %v", plain, result)
	}
	plainMetadata, ok := plain["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata = %T, want object", plain["metadata"])
	}
	if plainMetadata["url"] != "https://replay.example/article/" ||
		plainMetadata["title"] != nil || plainMetadata["description"] != nil || plainMetadata["preview_image"] != nil {
		t.Errorf("metadata without ignore_cache = %v, want the requested URL and nulls", plainMetadata)
	}

	// 4. GET /api/bookmarks/{id}/ — the detail fallback, identical to the
	// checked bookmark.
	rec = replay(t, f, http.MethodGet, fmt.Sprintf("/api/bookmarks/%d/", int64(saved["id"].(float64))), nil)
	wantStatus(t, rec, http.StatusOK)
	if got := decode(t, rec); !reflect.DeepEqual(got, checked) {
		t.Errorf("detail = %v, want the checked bookmark %v", got, checked)
	}
}

// TestExtensionSaveAndMergeReplay replays POST /api/bookmarks/ with the
// extension's scraping flags. The same POST is also the extension's edit
// operation: a duplicate URL merges into the stored row and still answers 201.
func TestExtensionSaveAndMergeReplay(t *testing.T) {
	f := newFixture(t)

	saved := save(t, f, "/api/bookmarks/?disable_scraping&disable_html_snapshot", map[string]any{
		"url":         "https://save.example/page",
		"title":       "Saved title",
		"description": "Saved description",
		"notes":       "Saved notes",
		"tag_names":   []string{"save", "replay"},
		"unread":      true,
		"shared":      false,
	})
	wantBookmarkShape(t, saved)
	if saved["url"] != "https://save.example/page" ||
		saved["title"] != "Saved title" ||
		saved["description"] != "Saved description" ||
		saved["notes"] != "Saved notes" ||
		saved["unread"] != true || saved["shared"] != false {
		t.Errorf("saved bookmark = %v, want the submitted fields", saved)
	}
	if !reflect.DeepEqual(saved["tag_names"], []any{"replay", "save"}) {
		t.Errorf("tag_names = %v, want the submitted tags", saved["tag_names"])
	}
	if saved["is_archived"] != false {
		t.Errorf("is_archived = %v, want false", saved["is_archived"])
	}
	for _, key := range []string{"web_archive_snapshot_url", "favicon_url", "preview_image_url"} {
		if saved[key] != nil {
			t.Errorf("%s = %v, want null", key, saved[key])
		}
	}
	if saved["website_title"] != "Saved title" {
		t.Errorf("website_title = %v, want Saved title", saved["website_title"])
	}
	if saved["website_description"] != "Saved description" {
		t.Errorf("website_description = %v, want Saved description", saved["website_description"])
	}
	id := saved["id"]

	// The extension's edit posts the same URL again with the new values.
	edited := save(t, f, "/api/bookmarks/?disable_scraping", map[string]any{
		"url":         "https://save.example/page",
		"title":       "Edited title",
		"description": "Edited description",
		"notes":       "",
		"tag_names":   []string{"save"},
		"unread":      false,
		"shared":      false,
	})
	wantBookmarkShape(t, edited)
	if edited["id"] != id {
		t.Errorf("merge id = %v, want %v", edited["id"], id)
	}
	if edited["title"] != "Edited title" || edited["description"] != "Edited description" ||
		edited["notes"] != "" || edited["unread"] != false {
		t.Errorf("merged bookmark = %v, want the submitted values", edited)
	}
	if !reflect.DeepEqual(edited["tag_names"], []any{"save"}) {
		t.Errorf("merged tag_names = %v, want [save]", edited["tag_names"])
	}
	// The merge keeps the stored archive state and date_added.
	if edited["is_archived"] != false || edited["date_added"] != saved["date_added"] {
		t.Errorf("merged = %v, want stored is_archived/date_added from %v", edited, saved)
	}

	// Both requests addressed one row, so the list holds exactly one bookmark.
	rec := replay(t, f, http.MethodGet, "/api/bookmarks/", nil)
	wantStatus(t, rec, http.StatusOK)
	page := decode(t, rec)
	wantKeys(t, page, "count", "next", "previous", "results")
	if page["count"] != float64(1) {
		t.Errorf("list count = %v, want 1", page["count"])
	}
}

// TestExtensionSearchAndConnectionReplay replays the omnibox request
// (?q=...&limit=5) and the connection test (?limit=1), both of which must
// answer 200 with a results array.
func TestExtensionSearchAndConnectionReplay(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 7; i++ {
		save(t, f, "/api/bookmarks/?disable_scraping", map[string]any{
			"url":   fmt.Sprintf("https://search.example/%d", i),
			"title": fmt.Sprintf("Omnibox suggestion %d", i),
		})
	}
	save(t, f, "/api/bookmarks/?disable_scraping", map[string]any{
		"url":   "https://search.example/unrelated",
		"title": "Unrelated page",
	})

	// Omnibox: q filters, limit=5 caps the window, count stays the total.
	rec := replay(t, f, http.MethodGet,
		"/api/bookmarks/?q="+url.QueryEscape("omnibox")+"&limit=5", nil)
	wantStatus(t, rec, http.StatusOK)
	page := decode(t, rec)
	wantKeys(t, page, "count", "next", "previous", "results")
	if page["count"] != float64(7) {
		t.Errorf("search count = %v, want 7 matches", page["count"])
	}
	results := pageResults(t, page)
	if len(results) != 5 {
		t.Fatalf("search results = %d, want the limit of 5", len(results))
	}
	for _, raw := range results {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("result = %T, want object", raw)
		}
		wantBookmarkShape(t, item)
	}
	if page["previous"] != nil {
		t.Errorf("search previous = %v, want null on the first page", page["previous"])
	}
	next, ok := page["next"].(string)
	if !ok {
		t.Fatalf("search next = %v, want a URL", page["next"])
	}
	nextQuery, err := url.Parse(next)
	if err != nil {
		t.Fatalf("parse next %q: %v", next, err)
	}
	if q := nextQuery.Query(); q.Get("limit") != "5" || q.Get("offset") != "5" || q.Get("q") != "omnibox" {
		t.Errorf("search next params = %v, want q/limit/offset carried over", q)
	}

	// Connection test: limit=1 proves reachability with one row and a results
	// array, whatever the row count is.
	rec = replay(t, f, http.MethodGet, "/api/bookmarks/?limit=1", nil)
	wantStatus(t, rec, http.StatusOK)
	page = decode(t, rec)
	wantKeys(t, page, "count", "next", "previous", "results")
	if page["count"] != float64(8) {
		t.Errorf("connection test count = %v, want 8", page["count"])
	}
	results = pageResults(t, page)
	if len(results) != 1 {
		t.Errorf("connection test results = %d, want exactly 1", len(results))
	}
	if _, ok := results[0].(map[string]any); !ok {
		t.Errorf("connection test result = %T, want object", results[0])
	}
}

// TestExtensionDeleteReplay replays DELETE /api/bookmarks/{id}/: the
// extension accepts only 204 with an empty body, and a repeat delete must be a
// 404 rather than a second success.
func TestExtensionDeleteReplay(t *testing.T) {
	f := newFixture(t)
	saved := save(t, f, "/api/bookmarks/?disable_scraping", map[string]any{
		"url":   "https://delete.example/page",
		"title": "Delete me",
	})
	path := fmt.Sprintf("/api/bookmarks/%d/", int64(saved["id"].(float64)))

	rec := replay(t, f, http.MethodDelete, path, nil)
	wantStatus(t, rec, http.StatusNoContent)
	if rec.Body.Len() != 0 {
		t.Errorf("delete body = %q, want empty", rec.Body.String())
	}

	rec = replay(t, f, http.MethodGet, path, nil)
	wantStatus(t, rec, http.StatusNotFound)

	rec = replay(t, f, http.MethodDelete, path, nil)
	wantStatus(t, rec, http.StatusNotFound)

	rec = replay(t, f, http.MethodGet, "/api/bookmarks/", nil)
	wantStatus(t, rec, http.StatusOK)
	page := decode(t, rec)
	wantKeys(t, page, "count", "next", "previous", "results")
	if page["count"] != float64(0) {
		t.Errorf("list count after delete = %v, want 0", page["count"])
	}
}
