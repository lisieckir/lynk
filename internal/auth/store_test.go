package auth

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/lisieckir/lynk/internal/database"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(context.Background(), database.Config{
		Path: filepath.Join(t.TempDir(), "test.db"),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestCreateToken(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	info, err := CreateToken(ctx, db, "alice", "main")
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if info.UserID == 0 {
		t.Error("expected non-zero user id")
	}
	if len(info.Token) != TokenHexLen {
		t.Errorf("token len = %d, want %d", len(info.Token), TokenHexLen)
	}
	// Verify stored digest matches
	digest := HashToken(info.Token)
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM api_tokens WHERE token_digest = ?", digest).Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 token row, got %d", count)
	}
}

func TestLookupByTokenRevoked(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	info, err := CreateToken(ctx, db, "bob", "t1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Lookup should work
	u, err := LookupByToken(ctx, db, info.Token)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if u.Username != "bob" {
		t.Errorf("username = %q, want bob", u.Username)
	}
	// Revoke the token
	_, err = db.ExecContext(ctx, "UPDATE api_tokens SET revoked_at = '2026-01-01T00:00:00Z' WHERE id = ?", info.ID)
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	// Lookup should fail now
	_, err = LookupByToken(ctx, db, info.Token)
	if err == nil {
		t.Error("expected error for revoked token")
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestLookupByTokenConcurrent(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	info, err := CreateToken(ctx, db, "carol", "t2")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := LookupByToken(ctx, db, info.Token)
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Errorf("concurrent lookup error: %v", e)
	}
}

func TestRevokeByPrefix(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	info, err := CreateToken(ctx, db, "dave", "t3")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Revoke by prefix
	if err := RevokeByPrefix(ctx, db, info.Prefix); err != nil {
		t.Fatalf("RevokeByPrefix: %v", err)
	}
	// Lookup should now fail
	_, err = LookupByToken(ctx, db, info.Token)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized after revoke, got %v", err)
	}
	// Revoke again should be not found
	err = RevokeByPrefix(ctx, db, info.Prefix)
	if !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("expected ErrTokenNotFound, got %v", err)
	}
}
