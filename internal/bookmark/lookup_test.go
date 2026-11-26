package bookmark

import (
	"context"
	"errors"
	"testing"
)

func TestGetByURLAndListTags(t *testing.T) {
	s, _ := testService(t)
	ctx := context.Background()

	created, err := s.Create(ctx, 1, CreateInput{
		URL: "https://example.com/x/", Title: "Example", Tags: []string{"beta", "alpha"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := s.GetByURL(ctx, 1, "HTTPS://EXAMPLE.COM/x")
	if err != nil {
		t.Fatalf("GetByURL: %v", err)
	}
	if got.ID != created.ID || got.Title != "Example" {
		t.Errorf("GetByURL = %+v, want id/title of %+v", got, created)
	}
	if _, err := s.GetByURL(ctx, 2, "https://example.com/x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-user GetByURL error = %v, want ErrNotFound", err)
	}
	if _, err := s.GetByURL(ctx, 1, "  "); !errors.Is(err, ErrInvalidBookmark) {
		t.Errorf("blank GetByURL error = %v, want ErrInvalidBookmark", err)
	}

	page, err := s.ListTags(ctx, 1, 0, 0)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	tags := page.Items
	if page.Count != 2 || page.Limit != DefaultLimit || page.Offset != 0 {
		t.Errorf("ListTags page = count %d limit %d offset %d, want 2/%d/0", page.Count, page.Limit, page.Offset, DefaultLimit)
	}
	if len(tags) != 2 || tags[0].Name != "alpha" || tags[1].Name != "beta" {
		t.Errorf("ListTags = %+v, want alpha, beta", tags)
	}
	for _, tag := range tags {
		if tag.UserID != 1 || tag.DateAdded.IsZero() {
			t.Errorf("tag = %+v, want owner id 1 and parsed date", tag)
		}
	}

	// Windowing: limit/offset cut the stable name order without changing the
	// total count.
	firstPage, err := s.ListTags(ctx, 1, 1, 0)
	if err != nil {
		t.Fatalf("ListTags limit=1: %v", err)
	}
	if firstPage.Count != 2 || len(firstPage.Items) != 1 || firstPage.Items[0].Name != "alpha" {
		t.Errorf("ListTags limit=1 = %+v, want count 2 and only alpha", firstPage)
	}
	secondPage, err := s.ListTags(ctx, 1, 1, 1)
	if err != nil {
		t.Fatalf("ListTags offset=1: %v", err)
	}
	if secondPage.Count != 2 || len(secondPage.Items) != 1 || secondPage.Items[0].Name != "beta" {
		t.Errorf("ListTags offset=1 = %+v, want count 2 and only beta", secondPage)
	}
	other, err := s.ListTags(ctx, 2, 0, 0)
	if err != nil {
		t.Fatalf("ListTags for other user: %v", err)
	}
	if other.Items == nil || len(other.Items) != 0 || other.Count != 0 {
		t.Errorf("ListTags for other user = %+v, want empty non-nil slice", other)
	}
}
