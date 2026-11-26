package linkding_test

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

// TestTagsLimitOffsetPagination proves GET /api/tags/ obeys the same
// LimitOffsetPagination contract as the bookmark lists. The extension loads
// the whole tag set with ?limit=5000 and expects the {count,next,previous,
// results} envelope, so both the bounded pages and the wide page matter.
func TestTagsLimitOffsetPagination(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 5; i++ {
		createBookmark(t, f.handler, token(f.aliceToken), map[string]any{
			"url":       fmt.Sprintf("https://tags.example/%d", i),
			"tag_names": []string{fmt.Sprintf("tag%d", i)},
		})
	}
	wantOrder := []string{"tag0", "tag1", "tag2", "tag3", "tag4"}

	// Default page: the whole set fits inside the default page size.
	rec := request(t, f.handler, http.MethodGet, "/api/tags/", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	page := decodeObject(t, rec)
	assertExactKeys(t, page, "count", "next", "previous", "results")
	if page["count"] != float64(5) || page["next"] != nil || page["previous"] != nil {
		t.Errorf("default tags page = %v, want count 5 with null links", page)
	}
	if got := tagNames(t, page); !reflect.DeepEqual(got, wantOrder) {
		t.Errorf("default tags = %v, want %v", got, wantOrder)
	}

	// limit=2: two rows per page while count stays the total.
	rec = request(t, f.handler, http.MethodGet, "/api/tags/?limit=2", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	page = decodeObject(t, rec)
	if page["count"] != float64(5) || page["previous"] != nil {
		t.Fatalf("tags page 1 = %v, want count 5 and null previous", page)
	}
	if got := tagNames(t, page); !reflect.DeepEqual(got, wantOrder[:2]) {
		t.Errorf("tags page 1 = %v, want %v", got, wantOrder[:2])
	}
	next, ok := page["next"].(string)
	if !ok {
		t.Fatalf("tags page 1 next = %v, want URL", page["next"])
	}
	if q := queryParams(t, next); q.Get("limit") != "2" || q.Get("offset") != "2" {
		t.Errorf("tags page 1 next params = %v, want limit=2 offset=2", q)
	}

	// Follow the link: the second window and a previous link back to offset 0.
	rec = request(t, f.handler, http.MethodGet, next, token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	page = decodeObject(t, rec)
	if page["count"] != float64(5) {
		t.Errorf("tags page 2 count = %v, want 5", page["count"])
	}
	if got := tagNames(t, page); !reflect.DeepEqual(got, wantOrder[2:4]) {
		t.Errorf("tags page 2 = %v, want %v", got, wantOrder[2:4])
	}
	prev, ok := page["previous"].(string)
	if !ok {
		t.Fatalf("tags page 2 previous = %v, want URL", page["previous"])
	}
	if offset := queryParams(t, prev).Get("offset"); offset != "" && offset != "0" {
		t.Errorf("tags page 2 previous offset = %q, want the first page", offset)
	}
	if page["next"] == nil {
		t.Fatal("tags page 2 next = nil, want the last page link")
	}

	// Last window: one row, no next link.
	rec = request(t, f.handler, http.MethodGet, "/api/tags/?limit=2&offset=4", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	page = decodeObject(t, rec)
	if page["count"] != float64(5) || page["next"] != nil {
		t.Errorf("tags last page = %v, want count 5 and null next", page)
	}
	if got := tagNames(t, page); !reflect.DeepEqual(got, wantOrder[4:]) {
		t.Errorf("tags last page = %v, want %v", got, wantOrder[4:])
	}

	// The extension's own request returns the full set in a single page.
	rec = request(t, f.handler, http.MethodGet, "/api/tags/?limit=5000", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	page = decodeObject(t, rec)
	assertExactKeys(t, page, "count", "next", "previous", "results")
	if page["count"] != float64(5) || page["next"] != nil || page["previous"] != nil {
		t.Errorf("tags limit=5000 page = %v, want the whole set", page)
	}
	if got := tagNames(t, page); !reflect.DeepEqual(got, wantOrder) {
		t.Errorf("tags limit=5000 = %v, want %v", got, wantOrder)
	}

	// Invalid limit/offset read as the defaults; an offset past the end is an
	// empty page that still reports the total.
	for _, path := range []string{"/api/tags/?limit=abc", "/api/tags/?limit=0", "/api/tags/?offset=abc", "/api/tags/?offset=-4"} {
		rec = request(t, f.handler, http.MethodGet, path, token(f.aliceToken), nil)
		assertStatus(t, rec, http.StatusOK)
		page = decodeObject(t, rec)
		if page["count"] != float64(5) || page["next"] != nil || page["previous"] != nil {
			t.Errorf("%s tags page = %v, want the full default page", path, page)
		}
	}
	rec = request(t, f.handler, http.MethodGet, "/api/tags/?offset=999", token(f.aliceToken), nil)
	assertStatus(t, rec, http.StatusOK)
	page = decodeObject(t, rec)
	if page["count"] != float64(5) || page["next"] != nil || page["previous"] == nil {
		t.Errorf("tags offset=999 page = %v, want count 5, null next and a previous link", page)
	}
	if got := tagNames(t, page); len(got) != 0 {
		t.Errorf("tags offset=999 results = %v, want none", got)
	}

	// Owner scoping holds for every window: Bob owns no tags.
	rec = request(t, f.handler, http.MethodGet, "/api/tags/?limit=5000", token(f.bobToken), nil)
	assertStatus(t, rec, http.StatusOK)
	page = decodeObject(t, rec)
	if page["count"] != float64(0) || len(tagNames(t, page)) != 0 {
		t.Errorf("bob tags page = %v, want an empty owned page", page)
	}
}

func tagNames(t *testing.T, page map[string]any) []string {
	t.Helper()
	results, ok := page["results"].([]any)
	if !ok {
		t.Fatalf("results = %T, want array", page["results"])
	}
	names := make([]string, 0, len(results))
	for _, raw := range results {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("tag = %T, want object", raw)
		}
		name, ok := item["name"].(string)
		if !ok {
			t.Fatalf("tag name = %T, want string", item["name"])
		}
		names = append(names, name)
	}
	return names
}
