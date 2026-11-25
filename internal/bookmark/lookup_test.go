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

	tags, err := s.ListTags(ctx, 1)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 2 || tags[0].Name != "alpha" || tags[1].Name != "beta" {
		t.Errorf("ListTags = %+v, want alpha, beta", tags)
	}
	for _, tag := range tags {
		if tag.UserID != 1 || tag.DateAdded.IsZero() {
			t.Errorf("tag = %+v, want owner id 1 and parsed date", tag)
		}
	}
	other, err := s.ListTags(ctx, 2)
	if err != nil {
		t.Fatalf("ListTags for other user: %v", err)
	}
	if other == nil || len(other) != 0 {
		t.Errorf("ListTags for other user = %+v, want empty non-nil slice", other)
	}
}
