package linkding_test

import (
	"bytes"
	"context"
	"database/sql"
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
	"github.com/lisieckir/lynk/internal/health"
	"github.com/lisieckir/lynk/internal/server"
)

type fixture struct {
	db         *sql.DB
	handler    http.Handler
	aliceToken string
	bobToken   string
	bobUserID  int64
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, database.Config{Path: filepath.Join(t.TempDir(), "compat.db")})
	if err != nil {
		t.Fatalf("open real migrated SQLite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	alice, err := auth.CreateToken(ctx, db, "alice", "compat")
	if err != nil {
		t.Fatalf("create alice token: %v", err)
	}
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := db.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, created_at, updated_at)
		VALUES ('bob', '!', ?, ?)`, stamp, stamp)
	if err != nil {
		t.Fatalf("insert bob: %v", err)
	}
	bobID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("bob id: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO user_profiles (user_id, theme, bookmark_date_display, bookmark_link_target,
			web_archive_integration, tag_search, enable_sharing, enable_public_sharing,
			enable_favicons, display_url, permanent_notes, search_preferences)
		VALUES (?, '', '', '', '', 0, 0, 0, 1, 0, '', '{}')`, bobID); err != nil {
		t.Fatalf("insert bob profile: %v", err)
	}
	bob, err := auth.CreateToken(ctx, db, "bob", "compat")
	if err != nil {
		t.Fatalf("create bob token: %v", err)
	}
	return &fixture{
		db:         db,
		handler:    linkding.NewHandler(db, nil),
		aliceToken: alice.Token,
		bobToken:   bob.Token,
		bobUserID:  bobID,
	}
}

func request(t *testing.T, h http.Handler, method, path, authz string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		var payload []byte
		switch value := body.(type) {
		case string:
			payload = []byte(value)
		default:
			var err error
			payload, err = json.Marshal(value)
			if err != nil {
				t.Fatalf("marshal request body: %v", err)
			}
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

func token(authz string) string { return "Token " + authz }

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") && want != http.StatusNoContent {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

func decodeObject(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode JSON %q: %v", rec.Body.String(), err)
	}
	return out
}

func assertNoKeys(t *testing.T, obj map[string]any, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if _, present := obj[key]; present {
			t.Errorf("response leaked internal key %q: %v", key, obj)
		}
	}
}

func assertExactKeys(t *testing.T, obj map[string]any, keys ...string) {
	t.Helper()
	got := make([]string, 0, len(obj))
	for key := range obj {
		got = append(got, key)
	}
	wantKeys := append([]string(nil), keys...)
	sort.Strings(got)
	sort.Strings(wantKeys)
	if !reflect.DeepEqual(got, wantKeys) {
		t.Errorf("JSON keys = %v, want %v", got, wantKeys)
	}
}

func createBookmark(t *testing.T, h http.Handler, authz string, body map[string]any) map[string]any {
	t.Helper()
	rec := request(t, h, http.MethodPost, "/api/bookmarks/", authz, body)
	assertStatus(t, rec, http.StatusCreated)
	return decodeObject(t, rec)
}

func TestAuthenticationRequiredAndAcceptedSchemes(t *testing.T) {
	f := newFixture(t)
	paths := []string{
		"/api/user/profile/",
		"/api/tags/",
		"/api/bookmarks/",
		"/api/bookmarks/check/?url=https://example.com",
		"/api/bookmarks/1/",
	}
	for _, path := range paths {
		t.Run("no auth "+path, func(t *testing.T) {
			rec := request(t, f.handler, http.MethodGet, path, "", nil)
			assertStatus(t, rec, http.StatusUnauthorized)
			if got := decodeObject(t, rec); !reflect.DeepEqual(got, map[string]any{"error": "unauthorized"}) {
				t.Errorf("body = %v, want unauthorized envelope", got)
			}
		})
	}
	for _, authz := range []string{
		"Token " + strings.Repeat("0", 40),
		"Basic " + f.aliceToken,
		"Token ",
	} {
		rec := request(t, f.handler, http.MethodGet, "/api/user/profile/", authz, nil)
		assertStatus(t, rec, http.StatusUnauthorized)
	}
	for _, authz := range []string{
		"Token " + f.aliceToken,
		"token " + f.aliceToken,
		"Bearer " + f.aliceToken,
		"bearer " + f.aliceToken,
		"Token " + f.bobToken,
	} {
		rec := request(t, f.handler, http.MethodGet, "/api/user/profile/", authz, nil)
		assertStatus(t, rec, http.StatusOK)
	}
}

func TestCanonicalRoutesAndHealthWiring(t *testing.T) {
	f := newFixture(t)
	// server.Handler proves the API is mounted alongside the unauthenticated
	// health route, and that missing slashes do not trigger a mux redirect.
	h := server.Handler(f.db)
	for _, path := range []string{
		"/api",
		"/api/bookmarks",
		"/api/bookmarks/check",
		"/api/bookmarks/1",
		"/api/tags",
		"/api/user/profile",
		"/api/bookmarks/abc/",
		"/api/bookmarks/1/extra/",
		"/api/unknown/",
	} {
		t.Run("404 "+path, func(t *testing.T) {
			rec := request(t, h, http.MethodGet, path, token(f.aliceToken), nil)
			assertStatus(t, rec, http.StatusNotFound)
			if location := rec.Header().Get("Location"); location != "" {
				t.Errorf("Location = %q, want no redirect", location)
			}
		})
	}

	cases := []struct {
		method, path, allow string
	}{
		{http.MethodPost, "/api/tags/", "GET"},
		{http.MethodDelete, "/api/bookmarks/", "GET, POST"},
		{http.MethodPost, "/api/user/profile/", "GET"},
		{http.MethodPost, "/api/bookmarks/1/", "GET, PUT, PATCH, DELETE"},
		{http.MethodGet, "/api/bookmarks/1/archive/", "POST"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := request(t, h, tc.method, tc.path, token(f.aliceToken), nil)
			assertStatus(t, rec, http.StatusMethodNotAllowed)
			if got := rec.Header().Get("Allow"); got != tc.allow {
				t.Errorf("Allow = %q, want %q", got, tc.allow)
			}
		})
	}

	healthRec := request(t, h, http.MethodGet, health.Path, "", nil)
	assertStatus(t, healthRec, http.StatusOK)
	if got := healthRec.Body.String(); got != "{\"status\":\"ok\"}\n" {
		t.Errorf("health body = %q", got)
	}
	healthSlash := request(t, h, http.MethodGet, health.Path+"/", "", nil)
	if healthSlash.Code != http.StatusNotFound {
		t.Errorf("GET %s/ status = %d, want 404", health.Path, healthSlash.Code)
	}

	// Confirm the same no-redirect behavior over a real HTTP server.
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/bookmarks", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", token(f.aliceToken))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /api/bookmarks: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound || resp.Header.Get("Location") != "" {
		t.Errorf("status = %d, Location = %q, want 404 without redirect", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestProfileContractAndOwnerValues(t *testing.T) {
	f := newFixture(t)
	rec := request(t, f.handler, http.MethodGet, "/api/user/profile/", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	profile := decodeObject(t, rec)
	assertExactKeys(t, profile,
		"theme", "bookmark_date_display", "bookmark_link_target", "web_archive_integration",
		"tag_search", "enable_sharing", "enable_public_sharing", "enable_favicons",
		"display_url", "permanent_notes", "search_preferences", "version")
	assertNoKeys(t, profile, "user_id", "username", "password_hash", "token", "id")
	if profile["tag_search"] != "strict" {
		t.Errorf("tag_search = %v, want string strict", profile["tag_search"])
	}
	if profile["permanent_notes"] != false {
		t.Errorf("permanent_notes = %v, want bool false", profile["permanent_notes"])
	}
	if _, ok := profile["search_preferences"].(map[string]any); !ok {
		t.Errorf("search_preferences = %T, want object", profile["search_preferences"])
	}
	if profile["version"] != "dev" || profile["enable_favicons"] != true {
		t.Errorf("version/enable_favicons = %v/%v", profile["version"], profile["enable_favicons"])
	}

	// Bob's stored profile uses Lynk's integer/text encoding; the DTO must
	// still expose Linkding's string/bool types and only Bob's values.
	ctx := context.Background()
	_, err := f.db.ExecContext(ctx, `
		UPDATE user_profiles SET theme='dark', tag_search=1, permanent_notes='keep',
			search_preferences='{"sort":"added_desc"}', enable_sharing=1
		WHERE user_id=?`, f.bobUserID)
	if err != nil {
		t.Fatalf("update bob profile: %v", err)
	}
	rec = request(t, f.handler, http.MethodGet, "/api/user/profile/", token(f.bobToken), nil)
	assertStatus(t, rec, http.StatusOK)
	profile = decodeObject(t, rec)
	if profile["theme"] != "dark" || profile["tag_search"] != "lax" || profile["permanent_notes"] != true || profile["enable_sharing"] != true {
		t.Errorf("bob profile not mapped to Linkding types: %v", profile)
	}
	prefs, ok := profile["search_preferences"].(map[string]any)
	if !ok || prefs["sort"] != "added_desc" {
		t.Errorf("search_preferences = %v, want sort=added_desc", profile["search_preferences"])
	}
}

func TestBookmarkCreateGetListDeleteContract(t *testing.T) {
	f := newFixture(t)
	created := createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url":           " HTTPS://Example.COM/path///?b=2&a=1 ",
		"title":         "First",
		"description":   "Desc",
		"notes":         "Notes",
		"tag_names":     []string{"beta", "alpha"},
		"unread":        true,
		"shared":        true,
		"id":            999,
		"website_title": "ignored",
		"date_added":    "2000-01-01T00:00:00Z",
	})
	assertExactKeys(t, created,
		"id", "url", "title", "description", "notes", "web_archive_snapshot_url",
		"favicon_url", "preview_image_url", "is_archived", "unread", "shared",
		"tag_names", "date_added", "date_modified", "website_title", "website_description")
	assertNoKeys(t, created, "user_id", "url_normalized", "tags", "password_hash")
	if created["id"] == float64(999) || created["id"].(float64) <= 0 {
		t.Errorf("id = %v, want generated id", created["id"])
	}
	if created["url"] != "HTTPS://Example.COM/path///?b=2&a=1" {
		t.Errorf("url = %v, want trimmed submitted URL", created["url"])
	}
	if created["title"] != "First" || created["description"] != "Desc" || created["notes"] != "Notes" {
		t.Errorf("create did not persist fields: %v", created)
	}
	if !reflect.DeepEqual(created["tag_names"], []any{"alpha", "beta"}) {
		t.Errorf("tag_names = %v, want sorted names", created["tag_names"])
	}
	for _, key := range []string{"web_archive_snapshot_url", "favicon_url", "preview_image_url", "website_title", "website_description"} {
		if created[key] != nil {
			t.Errorf("%s = %v, want null", key, created[key])
		}
	}
	if created["is_archived"] != false || created["unread"] != true || created["shared"] != true {
		t.Errorf("flags = %v", created)
	}
	for _, key := range []string{"date_added", "date_modified"} {
		value, ok := created[key].(string)
		if !ok {
			t.Fatalf("%s = %T, want string", key, created[key])
		}
		if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
			t.Errorf("%s = %q does not parse: %v", key, value, err)
		}
	}
	id := created["id"].(float64)

	// Linkding's create is also the extension's edit: a normalized duplicate
	// merges into the same owned row and still returns 201.
	merged := createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url":         "https://example.com/path/?a=1&b=2",
		"title":       "Updated",
		"description": "New desc",
		"tag_names":   []string{"gamma"},
	})
	if merged["id"] != id || merged["title"] != "Updated" || merged["description"] != "New desc" {
		t.Errorf("duplicate create did not merge: %v", merged)
	}
	if !reflect.DeepEqual(merged["tag_names"], []any{"gamma"}) {
		t.Errorf("tag_names after merge = %v", merged["tag_names"])
	}

	rec := request(t, f.handler, http.MethodGet, fmt.Sprintf("/api/bookmarks/%d/", int64(id)), token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	if got := decodeObject(t, rec); !reflect.DeepEqual(got, merged) {
		t.Errorf("GET detail = %v, want %v", got, merged)
	}

	rec = request(t, f.handler, http.MethodGet, "/api/bookmarks/", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	page := decodeObject(t, rec)
	assertExactKeys(t, page, "count", "next", "previous", "results")
	if page["count"] != float64(1) || page["next"] != nil || page["previous"] != nil {
		t.Errorf("pagination envelope = %v", page)
	}
	results, ok := page["results"].([]any)
	if !ok || len(results) != 1 || !reflect.DeepEqual(results[0], merged) {
		t.Errorf("results = %v, want one merged bookmark", page["results"])
	}

	rec = request(t, f.handler, http.MethodDelete, fmt.Sprintf("/api/bookmarks/%d/", int64(id)), token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusNoContent)
	if rec.Body.Len() != 0 {
		t.Errorf("DELETE body = %q, want empty", rec.Body.String())
	}
	rec = request(t, f.handler, http.MethodGet, fmt.Sprintf("/api/bookmarks/%d/", int64(id)), token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusNotFound)
	rec = request(t, f.handler, http.MethodDelete, fmt.Sprintf("/api/bookmarks/%d/", int64(id)), token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusNotFound)
}

func TestTagsAndCrossUserScoping(t *testing.T) {
	f := newFixture(t)
	aliceFirst := createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url": "https://alice.example/one", "title": "Alice one", "tag_names": []string{"one"},
	})
	createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url": "https://alice.example/two", "title": "Alice two", "tag_names": []string{"two"},
	})
	bobBookmark := createBookmark(t, f.handler, token(f.bobToken), map[string]any{
		"url": "https://bob.example/secret", "title": "Bob secret", "tag_names": []string{"secret"},
	})

	rec := request(t, f.handler, http.MethodGet, "/api/tags/", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	page := decodeObject(t, rec)
	assertExactKeys(t, page, "count", "next", "previous", "results")
	if page["count"] != float64(2) || page["next"] != nil || page["previous"] != nil {
		t.Errorf("tags envelope = %v", page)
	}
	tags := page["results"].([]any)
	firstTag := tags[0].(map[string]any)
	assertExactKeys(t, firstTag, "id", "name", "date_added")
	if firstTag["name"] != "one" || tags[1].(map[string]any)["name"] != "two" {
		t.Errorf("tag order/names = %v", tags)
	}
	if _, err := time.Parse(time.RFC3339Nano, firstTag["date_added"].(string)); err != nil {
		t.Errorf("tag date_added = %q: %v", firstTag["date_added"], err)
	}

	rec = request(t, f.handler, http.MethodGet, "/api/tags/", token(f.bobToken), nil)
	assertStatus(t, rec, http.StatusOK)
	bobTags := decodeObject(t, rec)["results"].([]any)
	if len(bobTags) != 1 || bobTags[0].(map[string]any)["name"] != "secret" {
		t.Errorf("bob tags = %v, want only secret", bobTags)
	}

	aliceID := int64(aliceFirst["id"].(float64))
	bobID := int64(bobBookmark["id"].(float64))
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, fmt.Sprintf("/api/bookmarks/%d/", bobID)},
		{http.MethodDelete, fmt.Sprintf("/api/bookmarks/%d/", bobID)},
	} {
		rec := request(t, f.handler, tc.method, tc.path, token(f.aliceToken), nil)
		assertStatus(t, rec, http.StatusNotFound)
	}
	rec = request(t, f.handler, http.MethodGet, fmt.Sprintf("/api/bookmarks/%d/", aliceID), token(f.bobToken), nil)
	assertStatus(t, rec, http.StatusNotFound)

	rec = request(t, f.handler, http.MethodGet, "/api/bookmarks/", token(f.bobToken), nil)
	assertStatus(t, rec, http.StatusOK)
	page = decodeObject(t, rec)
	if page["count"] != float64(1) {
		t.Errorf("bob list count = %v, want 1", page["count"])
	}
}

func TestCheckContract(t *testing.T) {
	f := newFixture(t)
	created := createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url": "https://check.example/a?x=1", "title": "Checked",
	})

	rec := request(t, f.handler, http.MethodGet,
		"/api/bookmarks/check/?url="+urlQueryEscape("HTTPS://CHECK.example/a/?x=1"), token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	result := decodeObject(t, rec)
	assertExactKeys(t, result, "bookmark", "metadata", "auto_tags")
	if result["bookmark"] == nil {
		t.Fatalf("bookmark = null, want existing row: %v", result)
	}
	if result["bookmark"].(map[string]any)["id"] != created["id"] {
		t.Errorf("check id = %v, want %v", result["bookmark"], created)
	}
	metadata := result["metadata"].(map[string]any)
	assertExactKeys(t, metadata, "url", "title", "description", "preview_image")
	if metadata["url"] != "HTTPS://CHECK.example/a/?x=1" || metadata["title"] != nil ||
		metadata["description"] != nil || metadata["preview_image"] != nil {
		t.Errorf("metadata = %v, want null values with requested URL", metadata)
	}
	if !reflect.DeepEqual(result["auto_tags"], []any{}) {
		t.Errorf("auto_tags = %v, want []", result["auto_tags"])
	}

	rec = request(t, f.handler, http.MethodGet,
		"/api/bookmarks/check/?url="+urlQueryEscape("https://check.example/unknown"), token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	if result = decodeObject(t, rec); result["bookmark"] != nil {
		t.Errorf("unknown URL returned a bookmark: %v", result)
	}

	// Owner scoping: Bob's check cannot see Alice's bookmark.
	rec = request(t, f.handler, http.MethodGet,
		"/api/bookmarks/check/?url="+urlQueryEscape("https://check.example/a?x=1"), token(f.bobToken), nil)
	assertStatus(t, rec, http.StatusOK)
	if result = decodeObject(t, rec); result["bookmark"] != nil {
		t.Errorf("cross-user check leaked a bookmark: %v", result)
	}

	for _, tc := range []struct {
		name, query, field string
	}{
		{"missing", "", "url"},
		{"blank", "?url=%20%20", "url"},
		{"unsafe scheme", "?url=" + urlQueryEscape("javascript:alert(1)"), "url"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := request(t, f.handler, http.MethodGet, "/api/bookmarks/check/"+tc.query, token(f.aliceToken), nil)
			assertStatus(t, rec, http.StatusBadRequest)
			body := decodeObject(t, rec)
			if _, ok := body[tc.field]; !ok {
				t.Errorf("error = %v, want field-keyed %q", body, tc.field)
			}
		})
	}
}

func TestValidationErrorContract(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		name, body, field string
	}{
		{"invalid json", `{`, ""},
		{"trailing json", `{"url":"https://example.com"}{"x":1}`, ""},
		{"missing url", `{"title":"x"}`, "url"},
		{"blank url", `{"url":"   "}`, "url"},
		{"bad scheme", `{"url":"javascript:alert(1)"}`, "url"},
		{"tag type", `{"url":"https://example.com","tag_names":[1]}`, "tag_names"},
		{"blank tag", `{"url":"https://example.com","tag_names":[""]}`, "tag_names"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := request(t, f.handler, http.MethodPost, "/api/bookmarks/", token(f.aliceToken), tc.body)
			assertStatus(t, rec, http.StatusBadRequest)
			body := decodeObject(t, rec)
			if tc.field == "" {
				if body["detail"] == nil {
					t.Errorf("error = %v, want detail envelope", body)
				}
				return
			}
			if _, ok := body[tc.field]; !ok {
				t.Errorf("error = %v, want field-keyed %q", body, tc.field)
			}
		})
	}
}

func urlQueryEscape(value string) string {
	return url.QueryEscape(value)
}

func resultIDs(t *testing.T, page map[string]any) []int64 {
	t.Helper()
	results, ok := page["results"].([]any)
	if !ok {
		t.Fatalf("results = %T, want array", page["results"])
	}
	ids := make([]int64, 0, len(results))
	for _, raw := range results {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("result = %T, want object", raw)
		}
		id, ok := item["id"].(float64)
		if !ok {
			t.Fatalf("result id = %T, want number", item["id"])
		}
		ids = append(ids, int64(id))
	}
	return ids
}

func queryParams(t *testing.T, raw string) url.Values {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse link %q: %v", raw, err)
	}
	return u.Query()
}

func TestSearchOverFTSFields(t *testing.T) {
	f := newFixture(t)
	b1 := createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url": "https://alpha.example/one", "title": "Alpha title",
		"description": "shared description", "notes": "some notes",
		"tag_names": []string{"zeta"},
	})
	b2 := createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url": "https://beta.example/two", "title": "Beta title",
	})
	wantAlpha := []int64{int64(b1["id"].(float64))}
	wantBeta := []int64{int64(b2["id"].(float64))}
	cases := []struct {
		name, query string
		want        []int64
	}{
		{"title", "Alpha", wantAlpha},
		{"second title", "Beta", wantBeta},
		{"description", "shared description", wantAlpha},
		{"notes", "some notes", wantAlpha},
		{"url", "alpha.example", wantAlpha},
		{"tags", "zeta", wantAlpha},
		{"partial word", "alph", wantAlpha},
		{"no match", "zzz-nomatch-term", []int64{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := request(t, f.handler, http.MethodGet, "/api/bookmarks/?q="+urlQueryEscape(tc.query), token(f.aliceToken), nil)
			assertStatus(t, rec, http.StatusOK)
			page := decodeObject(t, rec)
			assertExactKeys(t, page, "count", "next", "previous", "results")
			if got := resultIDs(t, page); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("search q=%q ids = %v, want %v", tc.query, got, tc.want)
			}
			if page["next"] != nil || page["previous"] != nil {
				t.Errorf("search q=%q next/previous = %v/%v, want null", tc.query, page["next"], page["previous"])
			}
		})
	}
}

func TestSearchInjectionSafety(t *testing.T) {
	f := newFixture(t)
	created := createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url": "https://example.com", "title": "Example",
	})
	queries := []string{
		`"`, `'`, `a" AND b`, `"Alpha" OR`, `--`, `-- -x`,
		`DROP TABLE bookmarks`, `"UNION SELECT id FROM bookmarks --`,
		`AND OR NOT NEAR(`, `title:x`, `""`, `MATCH`,
	}
	for _, q := range queries {
		t.Run(q, func(t *testing.T) {
			rec := request(t, f.handler, http.MethodGet, "/api/bookmarks/?q="+urlQueryEscape(q), token(f.aliceToken), nil)
			assertStatus(t, rec, http.StatusOK)
			page := decodeObject(t, rec)
			assertExactKeys(t, page, "count", "next", "previous", "results")
			results, ok := page["results"].([]any)
			if !ok {
				t.Fatalf("results = %T, want array", page["results"])
			}
			if float64(len(results)) != page["count"].(float64) {
				t.Errorf("q=%q count=%v != len(results)=%d", q, page["count"], len(results))
			}
			for _, raw := range results {
				if raw.(map[string]any)["id"] != created["id"] {
					t.Errorf("q=%q returned a bookmark the query should not match", q)
				}
			}
		})
	}
}

func TestListPagination(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 5; i++ {
		createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
			"url": fmt.Sprintf("https://paginate.example/%d", i), "title": fmt.Sprintf("Paginate %d", i),
		})
	}

	// Default page: the whole list fits and next/previous are null.
	rec := request(t, f.handler, http.MethodGet, "/api/bookmarks/", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	page := decodeObject(t, rec)
	assertExactKeys(t, page, "count", "next", "previous", "results")
	if page["count"] != float64(5) || page["next"] != nil || page["previous"] != nil {
		t.Errorf("default page = %v", page)
	}
	if got := resultIDs(t, page); len(got) != 5 {
		t.Errorf("default page ids = %v, want 5", got)
	}

	// Page 1 of limit=2: count is the total, previous is null, next carries
	// the limit and the next offset.
	rec = request(t, f.handler, http.MethodGet, "/api/bookmarks/?limit=2", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	page = decodeObject(t, rec)
	if page["count"] != float64(5) || len(resultIDs(t, page)) != 2 {
		t.Fatalf("page1 = %v", page)
	}
	if page["previous"] != nil {
		t.Errorf("page1 previous = %v, want null", page["previous"])
	}
	next := page["next"]
	if next == nil {
		t.Fatal("page1 next = nil, want URL")
	}
	if q := queryParams(t, next.(string)); q.Get("limit") != "2" || q.Get("offset") != "2" {
		t.Errorf("page1 next params = %v, want limit=2 offset=2", q)
	}

	// Page 2: follow next; previous has no offset when it is the first page.
	rec = request(t, f.handler, http.MethodGet, next.(string), token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	page = decodeObject(t, rec)
	if page["count"] != float64(5) || len(resultIDs(t, page)) != 2 {
		t.Fatalf("page2 = %v", page)
	}
	if prev := page["previous"]; prev == nil || queryParams(t, prev.(string)).Get("offset") != "" {
		t.Errorf("page2 previous = %v, want URL without offset", page["previous"])
	}
	if next := page["next"]; next == nil || queryParams(t, next.(string)).Get("offset") != "4" {
		t.Errorf("page2 next = %v, want URL with offset=4", page["next"])
	}

	// Last page: next is null, previous carries offset=2.
	rec = request(t, f.handler, http.MethodGet, "/api/bookmarks/?limit=2&offset=4", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	page = decodeObject(t, rec)
	if page["count"] != float64(5) || page["next"] != nil || len(resultIDs(t, page)) != 1 {
		t.Errorf("last page = %v", page)
	}
	if prev := page["previous"]; prev == nil || queryParams(t, prev.(string)).Get("offset") != "2" {
		t.Errorf("last page previous = %v, want URL with offset=2", page["previous"])
	}

	// Invalid limit/offset fall back to the Linkding defaults (limit=abc and
	// limit=0 use the default page size, offset=abc and offset<0 read as 0).
	for _, path := range []string{
		"/api/bookmarks/?limit=abc", "/api/bookmarks/?limit=0",
		"/api/bookmarks/?offset=abc", "/api/bookmarks/?offset=-4",
	} {
		rec = request(t, f.handler, http.MethodGet, path, token(f.aliceToken), nil)
		assertStatus(t, rec, http.StatusOK)
		page = decodeObject(t, rec)
		if page["count"] != float64(5) || page["next"] != nil || page["previous"] != nil {
			t.Errorf("%s page = %v, want full page", path, page)
		}
	}
}

func TestUpdatePutPatchAndCollisions(t *testing.T) {
	f := newFixture(t)
	b := createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url": "https://update.example/a", "title": "Original", "description": "Original desc",
	})
	duplicate := createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url": "https://dup.example/x", "title": "Dup",
	})
	bobBookmark := createBookmark(t, f.handler, token(f.bobToken), map[string]any{
		"url": "https://collision.example/b", "title": "Bob",
	})
	id := fmt.Sprintf("/api/bookmarks/%d/", int64(b["id"].(float64)))

	// PUT without url: DRF marks url required for updates.
	rec := request(t, f.handler, http.MethodPut, id, token(f.aliceToken), map[string]any{"title": "x"})
	assertStatus(t, rec, http.StatusBadRequest)
	if _, ok := decodeObject(t, rec)["url"]; !ok {
		t.Errorf("PUT without url error = %v, want url field", decodeObject(t, rec))
	}

	// PUT with an invalid URL: 400, url field.
	rec = request(t, f.handler, http.MethodPut, id, token(f.aliceToken), map[string]any{"url": "javascript:alert(1)"})
	assertStatus(t, rec, http.StatusBadRequest)
	if _, ok := decodeObject(t, rec)["url"]; !ok {
		t.Errorf("PUT invalid url error = %v, want url field", decodeObject(t, rec))
	}

	// PUT success: 200 with the full bookmark object, updated values persisted.
	rec = request(t, f.handler, http.MethodPut, id, token(f.aliceToken), map[string]any{
		"url": "https://update.example/b", "title": "Updated", "description": "New desc", "unread": true,
	})
	assertStatus(t, rec, http.StatusOK)
	updated := decodeObject(t, rec)
	assertExactKeys(t, updated,
		"id", "url", "title", "description", "notes", "web_archive_snapshot_url",
		"favicon_url", "preview_image_url", "is_archived", "unread", "shared",
		"tag_names", "date_added", "date_modified", "website_title", "website_description")
	if updated["id"] != b["id"] || updated["url"] != "https://update.example/b" ||
		updated["title"] != "Updated" || updated["description"] != "New desc" || updated["unread"] != true {
		t.Errorf("PUT response = %v", updated)
	}
	rec = request(t, f.handler, http.MethodGet, id, token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	if got := decodeObject(t, rec); !reflect.DeepEqual(got, updated) {
		t.Errorf("GET after PUT = %v, want %v", got, updated)
	}

	// PUT onto a URL another bookmark of the same user owns: 400 url collision.
	rec = request(t, f.handler, http.MethodPut, id, token(f.aliceToken), map[string]any{"url": "HTTPS://DUP.example/x"})
	assertStatus(t, rec, http.StatusBadRequest)
	if _, ok := decodeObject(t, rec)["url"]; !ok {
		t.Errorf("collision error = %v, want url field", decodeObject(t, rec))
	}
	// The collision leaves the other user's duplicate row untouched.
	rec = request(t, f.handler, http.MethodGet,
		fmt.Sprintf("/api/bookmarks/%d/", int64(duplicate["id"].(float64))), token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	if got := decodeObject(t, rec)["url"]; got != "https://dup.example/x" {
		t.Errorf("duplicate url = %v, want https://dup.example/x", got)
	}

	// PUT to another user's bookmark: 404.
	rec = request(t, f.handler, http.MethodPut, fmt.Sprintf("/api/bookmarks/%d/", int64(bobBookmark["id"].(float64))), token(f.aliceToken), map[string]any{"url": "https://elsewhere.example"})
	assertStatus(t, rec, http.StatusNotFound)

	// PATCH is partial: only the provided fields change.
	rec = request(t, f.handler, http.MethodPatch, id, token(f.aliceToken), map[string]any{"title": "Patched"})
	assertStatus(t, rec, http.StatusOK)
	patched := decodeObject(t, rec)
	assertExactKeys(t, patched,
		"id", "url", "title", "description", "notes", "web_archive_snapshot_url",
		"favicon_url", "preview_image_url", "is_archived", "unread", "shared",
		"tag_names", "date_added", "date_modified", "website_title", "website_description")
	if patched["title"] != "Patched" || patched["description"] != "New desc" || patched["url"] != "https://update.example/b" {
		t.Errorf("PATCH response = %v, want patched title with other fields intact", patched)
	}

	// PATCH on a missing bookmark: 404.
	rec = request(t, f.handler, http.MethodPatch, "/api/bookmarks/999999/", token(f.aliceToken), map[string]any{"title": "x"})
	assertStatus(t, rec, http.StatusNotFound)
}

func TestUpdateBlankTagNamesReturnsFieldError(t *testing.T) {
	f := newFixture(t)
	b := createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url": "https://blanktags.example/a", "title": "Original", "tag_names": []string{"keep"},
	})
	id := fmt.Sprintf("/api/bookmarks/%d/", int64(b["id"].(float64)))

	for _, method := range []string{http.MethodPut, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			rec := request(t, f.handler, method, id, token(f.aliceToken), map[string]any{
				"url":       "https://blanktags.example/a",
				"tag_names": []string{""},
			})
			assertStatus(t, rec, http.StatusBadRequest)
			body := decodeObject(t, rec)
			if _, ok := body["tag_names"]; !ok {
				t.Errorf("%s blank tag_names error = %v, want tag_names field", method, body)
			}
			if _, ok := body["url"]; ok {
				t.Errorf("%s blank tag_names error leaked url field: %v", method, body)
			}

			// Whitespace-only tags are blank too.
			rec = request(t, f.handler, method, id, token(f.aliceToken), map[string]any{
				"url":       "https://blanktags.example/a",
				"tag_names": []string{"   "},
			})
			assertStatus(t, rec, http.StatusBadRequest)
			body = decodeObject(t, rec)
			if _, ok := body["tag_names"]; !ok {
				t.Errorf("%s whitespace tag_names error = %v, want tag_names field", method, body)
			}
		})
	}

	// The failed validation must not have changed the stored tags.
	rec := request(t, f.handler, http.MethodGet, id, token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	if got := decodeObject(t, rec)["tag_names"]; !reflect.DeepEqual(got, []any{"keep"}) {
		t.Errorf("tags after failed update = %v, want [keep]", got)
	}
}

func TestArchiveUnarchiveAndArchivedList(t *testing.T) {
	f := newFixture(t)
	b := createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
		"url": "https://archive.example/a", "title": "Arch me",
	})
	bobBookmark := createBookmark(t, f.handler, token(f.bobToken), map[string]any{
		"url": "https://archive.example/bob", "title": "Bob arch",
	})
	id := fmt.Sprintf("/api/bookmarks/%d/", int64(b["id"].(float64)))

	// Archive: empty 204 body.
	rec := request(t, f.handler, http.MethodPost, id+"archive/", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusNoContent)
	if rec.Body.Len() != 0 {
		t.Errorf("archive body = %q, want empty", rec.Body.String())
	}

	// Detail shows the archived flag.
	rec = request(t, f.handler, http.MethodGet, id, token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	if got := decodeObject(t, rec)["is_archived"]; got != true {
		t.Errorf("GET is_archived = %v, want true", got)
	}

	// Active list excludes it; the archived list includes it with the envelope.
	rec = request(t, f.handler, http.MethodGet, "/api/bookmarks/", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	if page := decodeObject(t, rec); page["count"] != float64(0) {
		t.Errorf("active page count = %v, want 0", page["count"])
	}
	rec = request(t, f.handler, http.MethodGet, "/api/bookmarks/archived/", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	page := decodeObject(t, rec)
	assertExactKeys(t, page, "count", "next", "previous", "results")
	if page["count"] != float64(1) || page["next"] != nil || page["previous"] != nil {
		t.Errorf("archived page = %v", page)
	}
	if ids := resultIDs(t, page); !reflect.DeepEqual(ids, []int64{int64(b["id"].(float64))}) {
		t.Errorf("archived ids = %v, want the archived bookmark", ids)
	}

	// The archived list honours q.
	rec = request(t, f.handler, http.MethodGet, "/api/bookmarks/archived/?q=Arch", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	if page := decodeObject(t, rec); page["count"] != float64(1) {
		t.Errorf("archived search count = %v, want 1", page["count"])
	}

	// Archive on another user's bookmark: 404.
	rec = request(t, f.handler, http.MethodPost, fmt.Sprintf("/api/bookmarks/%d/archive/", int64(bobBookmark["id"].(float64))), token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusNotFound)

	// Unarchive: empty 204; the archived list empties again.
	rec = request(t, f.handler, http.MethodPost, id+"unarchive/", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusNoContent)
	if rec.Body.Len() != 0 {
		t.Errorf("unarchive body = %q, want empty", rec.Body.String())
	}
	rec = request(t, f.handler, http.MethodGet, "/api/bookmarks/archived/", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	if page := decodeObject(t, rec); page["count"] != float64(0) {
		t.Errorf("archived page after unarchive count = %v, want 0", page["count"])
	}

	// The other user can archive their own bookmark.
	rec = request(t, f.handler, http.MethodPost, fmt.Sprintf("/api/bookmarks/%d/archive/", int64(bobBookmark["id"].(float64))), token(f.bobToken), nil)
	assertStatus(t, rec, http.StatusNoContent)
	rec = request(t, f.handler, http.MethodGet, "/api/bookmarks/archived/", token(f.bobToken), nil)
	assertStatus(t, rec, http.StatusOK)
	if page := decodeObject(t, rec); page["count"] != float64(1) {
		t.Errorf("bob archived count = %v, want 1", page["count"])
	}
}
