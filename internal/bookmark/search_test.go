package bookmark

import (
	"context"
	"fmt"
	"testing"
)

func TestFTSQuery(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"   ", ""},
		{"hello", `"hello"*`},
		{"  Hello  World ", `"Hello" "World"*`},
		{"a.b/c?x=y", `"a" "b" "c" "x" "y"*`},
		{`say "hi" now`, `"say" "hi" "now"*`},
		{"café", `"café"*`},
		{`"UNION SELECT", 1; DROP`, `"UNION" "SELECT" "1" "DROP"*`},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := ftsQuery(tc.in); got != tc.want {
				t.Errorf("ftsQuery(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestListSearchCoversFTSFields(t *testing.T) {
	s, _ := testService(t)
	ctx := context.Background()
	b1, err := s.Create(ctx, 1, CreateInput{
		URL: "https://alpha.example.com/", Title: "Alpha title",
		Description: "unique description", Notes: "some notes", Tags: []string{"zeta"},
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.Create(ctx, 2, CreateInput{URL: "https://alpha.example.com/", Title: "Alpha title"})
	if err != nil {
		t.Fatal(err)
	}

	// Each case: a query and the single user 1 bookmark it must match.
	cases := []struct {
		name, query string
		wantID      int64
	}{
		{"title", "Alpha", b1.ID},
		{"description", "unique description", b1.ID},
		{"notes", "some notes", b1.ID},
		{"url", "alpha.example.com", b1.ID},
		{"tags", "zeta", b1.ID},
		{"partial word (prefix)", "alph", b1.ID},
		{"infix is not a prefix match", "lph", 0},
		{"no match", "nomatchterm", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, err := s.List(ctx, 1, false, PageOptions{Query: tc.query})
			if err != nil {
				t.Fatalf("List query %q: %v", tc.query, err)
			}
			if page.Count != len(page.Items) {
				t.Fatalf("List query %q: count=%d != len(items)=%d", tc.query, page.Count, len(page.Items))
			}
			if tc.wantID == 0 {
				if page.Count != 0 || len(page.Items) != 0 {
					t.Errorf("List query %q returned %d rows, want none", tc.query, page.Count)
				}
				return
			}
			if page.Count != 1 || len(page.Items) != 1 {
				t.Fatalf("List query %q count=%d items=%v, want 1 match", tc.query, page.Count, page.Items)
			}
			if got := page.Items[0].ID; got != tc.wantID {
				t.Errorf("List query %q matched id=%d, want %d", tc.query, got, tc.wantID)
			}
			if page.Items[0].UserID != 1 {
				t.Errorf("List query %q returned foreign bookmark", tc.query)
			}
		})
	}

	// Owner scoping: the same query as the other user returns their row only.
	page, err := s.List(ctx, 2, false, PageOptions{Query: "alpha.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Count != 1 || page.Items[0].ID != other.ID {
		t.Errorf("owner 2 search = %+v, want only bookmark %d", page, other.ID)
	}
}

func TestListSearchInjectionSafety(t *testing.T) {
	s, _ := testService(t)
	ctx := context.Background()
	if _, err := s.Create(ctx, 1, CreateInput{URL: "https://example.com", Title: "title", Tags: []string{"tag"}}); err != nil {
		t.Fatal(err)
	}
	queries := []string{
		`"`, `'`, `" OR 1=1`, `a" AND b`, `"alpha" OR`, `--`, `-- -x`,
		`DROP TABLE bookmarks`, `"UNION SELECT id FROM bookmarks --`,
		`AND OR NOT NEAR(`, `title:x`, `""`, `1; DELETE FROM bookmarks`,
		`MATCH`, `column:x`, `"`,
	}
	for _, q := range queries {
		t.Run(q, func(t *testing.T) {
			page, err := s.List(ctx, 1, false, PageOptions{Query: q})
			if err != nil {
				t.Fatalf("List query %q: %v", q, err)
			}
			if page.Count != len(page.Items) {
				t.Errorf("List query %q: count=%d != len(items)=%d", q, page.Count, len(page.Items))
			}
			for _, item := range page.Items {
				if item.UserID != 1 {
					t.Errorf("List query %q returned foreign bookmark %+v", q, item)
				}
			}
		})
	}
}

func TestListPaginationSemantics(t *testing.T) {
	s, _ := testService(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if _, err := s.Create(ctx, 1, CreateInput{URL: fmt.Sprintf("https://example%d.com", i)}); err != nil {
			t.Fatal(err)
		}
	}

	// Default page shape when no limit is given: the full list.
	page, err := s.List(ctx, 1, false, PageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Count != 5 || len(page.Items) != 5 {
		t.Errorf("default page count=%d len=%d, want 5/5", page.Count, len(page.Items))
	}
	if page.Limit != DefaultLimit || page.Offset != 0 {
		t.Errorf("default page limit/offset = %d/%d, want %d/0", page.Limit, page.Offset, DefaultLimit)
	}

	for _, tc := range []struct {
		name    string
		opts    PageOptions
		wantLen int
		wantIDs []int64
	}{
		{"limit 2 offset 0", PageOptions{Limit: 2, Offset: 0}, 2, []int64{5, 4}},
		{"limit 2 offset 1", PageOptions{Limit: 2, Offset: 1}, 2, []int64{4, 3}},
		{"limit 2 offset 4", PageOptions{Limit: 2, Offset: 4}, 1, []int64{1}},
		{"limit 2 offset 6", PageOptions{Limit: 2, Offset: 6}, 0, nil},
		{"negative values", PageOptions{Limit: -3, Offset: -1}, 5, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page, err := s.List(ctx, 1, false, tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			if page.Count != 5 || len(page.Items) != tc.wantLen {
				t.Fatalf("count=%d len=%d, want 5/%d", page.Count, len(page.Items), tc.wantLen)
			}
			for i, id := range tc.wantIDs {
				if page.Items[i].ID != id {
					t.Errorf("item %d id = %d, want %d", i, page.Items[i].ID, id)
				}
			}
			if page.Items == nil {
				t.Error("Items must be non-nil")
			}
		})
	}

	// User 2 sees nothing of user 1's list.
	page, err = s.List(ctx, 2, false, PageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Count != 0 || len(page.Items) != 0 || page.Items == nil {
		t.Errorf("user 2 page = %+v, want empty non-nil", page)
	}
}

func TestListSearchArchivedCombination(t *testing.T) {
	s, _ := testService(t)
	ctx := context.Background()
	b, err := s.Create(ctx, 1, CreateInput{URL: "https://example.com/archived-term", Title: "Term"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Archive(ctx, 1, b.ID); err != nil {
		t.Fatal(err)
	}
	// Archived rows stay out of the active list, but are found in the
	// archived list when the same search query is used.
	for _, tc := range []struct {
		archived bool
		query    string
		want     int
	}{
		{false, "archived-term", 0},
		{true, "archived-term", 1},
		{true, "term", 1},
		{true, "", 1},
	} {
		page, err := s.List(ctx, 1, tc.archived, PageOptions{Query: tc.query})
		if err != nil {
			t.Fatal(err)
		}
		if page.Count != tc.want || len(page.Items) != tc.want {
			t.Errorf("archived=%v query=%q count=%d len=%d, want %d", tc.archived, tc.query, page.Count, len(page.Items), tc.want)
		}
	}
}
