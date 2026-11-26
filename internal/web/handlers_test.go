package web_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lisieckir/lynk/internal/auth"
	"github.com/lisieckir/lynk/internal/database"
	"github.com/lisieckir/lynk/internal/web"
)

func setupTestHandler(t *testing.T) (http.Handler, *sql.DB, string) {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(ctx, database.Config{Path: dbPath})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ti, err := auth.CreateToken(ctx, db, "testuser", "testtoken")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	h, err := web.NewHandler(db, nil, web.Config{SecureCookie: false})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	mux := http.NewServeMux()
	h.Register(mux)
	return mux, db, ti.Token
}

func TestUnauthenticatedRedirect(t *testing.T) {
	h, _, _ := setupTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != "/login" {
		t.Fatalf("expected redirect to /login, got %s", loc)
	}
}

func TestLoginGet(t *testing.T) {
	h, _, _ := setupTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "API Token") {
		t.Fatalf("expected login form content")
	}
}
