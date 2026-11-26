package linkding

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPageLinksHugeValuesNoBogusNext(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/bookmarks/?limit=1&offset=0", nil)
	// offset + limit overflows an int on 64-bit when both are huge; the
	// helper must not report a next page in that case.
	huge := 1 << 62
	next, _ := pageLinks(r, 5, huge, huge)
	if next != nil {
		t.Fatalf("pageLinks returned bogus next URL: %s", *next)
	}

	// A normal in-range page still produces a next link.
	next, _ = pageLinks(r, 5, 2, 0)
	if next == nil {
		t.Fatal("pageLinks returned nil next for in-range page")
	}
}
