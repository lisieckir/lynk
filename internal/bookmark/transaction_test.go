package bookmark

import (
	"context"
	"errors"
	"testing"
)

// TestCreateInTx keeps the transaction-scoped create primitive honest: it must
// obey the caller's commit/rollback decision and use the same duplicate-merge
// rules as Create.
func TestCreateInTx(t *testing.T) {
	svc, db := testService(t)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := svc.CreateInTx(ctx, tx, 1, CreateInput{URL: "https://tx.example/a", Title: "A"}); err != nil {
		t.Fatalf("CreateInTx: %v", err)
	}
	if _, err := svc.GetByURLInTx(ctx, tx, 1, "https://TX.example/a/"); err != nil {
		t.Fatalf("GetByURLInTx cannot see uncommitted row: %v", err)
	}
	if _, err := svc.GetByURL(ctx, 1, "https://tx.example/a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("connection outside transaction sees uncommitted row: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if _, err := svc.GetByURL(ctx, 1, "https://tx.example/a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("row survived rollback: %v", err)
	}

	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin committed: %v", err)
	}
	if _, err := svc.CreateInTx(ctx, tx, 1, CreateInput{URL: "https://tx.example/b", Title: "B"}); err != nil {
		t.Fatalf("CreateInTx committed: %v", err)
	}
	if _, err := svc.CreateInTx(ctx, tx, 1, CreateInput{URL: "https://tx.example/b", Title: "Merged"}); err != nil {
		t.Fatalf("CreateInTx duplicate: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	item, err := svc.GetByURL(ctx, 1, "https://tx.example/b")
	if err != nil {
		t.Fatalf("GetByURL: %v", err)
	}
	if item.Title != "Merged" {
		t.Errorf("title = %q, want %q", item.Title, "Merged")
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM bookmarks WHERE user_id = 1`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestTransactionHelpersRejectNilAndInvalidInput(t *testing.T) {
	svc, _ := testService(t)
	ctx := context.Background()
	if _, err := svc.CreateInTx(ctx, nil, 1, CreateInput{URL: "https://e.example"}); err == nil {
		t.Error("CreateInTx accepted a nil transaction")
	}
	if _, err := svc.GetByURLInTx(ctx, nil, 1, "https://e.example"); err == nil {
		t.Error("GetByURLInTx accepted a nil transaction")
	}
	tx, err := svc.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := svc.CreateInTx(ctx, tx, 1, CreateInput{URL: "javascript:alert(1)"}); !errors.Is(err, ErrInvalidBookmark) {
		t.Errorf("invalid URL: got %v, want ErrInvalidBookmark", err)
	}
}
