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

func setupConfiguredHandler(t *testing.T, cfg web.Config) (http.Handler, *sql.DB, string) {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(ctx, database.Config{Path: dbPath})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ti, err := auth.CreateToken(ctx, db, "configured-user", "configured-token")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	h, err := web.NewHandler(db, nil, cfg)
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

func TestLogoutRequiresCSRF(t *testing.T) {
	h, _, token := setupTestHandler(t)
	// Get login page to obtain login CSRF cookie
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login get: %d", rec.Code)
	}
	// extract login csrf cookie
	var loginCSRF string
	for _, c := range rec.Result().Cookies() {
		if c.Name == "lynk_login_csrf" {
			loginCSRF = c.Value
		}
	}
	if loginCSRF == "" {
		t.Fatalf("no login csrf cookie")
	}
	// Post login
	form := "csrf_token=" + loginCSRF + "&token=" + token
	req = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "lynk_login_csrf", Value: loginCSRF})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login post: %d body=%s", rec.Code, rec.Body.String())
	}
	// get session cookie
	var sessionCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "lynk_session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatalf("no session cookie")
	}
	// Try logout without CSRF
	req = httptest.NewRequest(http.MethodPost, "/logout", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("logout without csrf: expected 303, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "err=invalid+csrf") {
		t.Fatalf("expected invalid csrf redirect, got %s", rec.Header().Get("Location"))
	}
}

func TestSecurityHeaders(t *testing.T) {
	h, _, _ := setupTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("missing X-Frame-Options")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("missing Cache-Control no-store, got %q", rec.Header().Get("Cache-Control"))
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Errorf("missing CSP")
	}
}

func TestLoginBodyTooLarge(t *testing.T) {
	h, _, _ := setupTestHandler(t)
	// Create a body larger than 1MiB
	big := strings.Repeat("a", 1<<20+100)
	form := "csrf_token=x&token=y&data=" + big
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	// Should redirect to login with error (parse error) due to MaxBytesReader
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
}

func TestProductionSessionCookieFlags(t *testing.T) {
	h, _, token := setupConfiguredHandler(t, web.Config{SecureCookie: true})

	getReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)
	var loginCSRF string
	for _, cookie := range getRec.Result().Cookies() {
		if cookie.Name == web.LoginCSRFCookieName {
			loginCSRF = cookie.Value
		}
	}
	if loginCSRF == "" {
		t.Fatal("login csrf cookie missing")
	}

	req := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader("csrf_token="+loginCSRF+"&token="+token))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: web.LoginCSRFCookieName, Value: loginCSRF})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login post: got %d, body=%s", rec.Code, rec.Body.String())
	}

	var session *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == web.SessionCookieName {
			session = cookie
		}
	}
	if session == nil {
		t.Fatal("session cookie missing")
	}
	if !session.Secure || !session.HttpOnly {
		t.Fatalf("session cookie security flags = secure:%v httponly:%v", session.Secure, session.HttpOnly)
	}
	if session.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session SameSite = %v, want Lax", session.SameSite)
	}
	if session.Path != "/" || session.MaxAge <= 0 {
		t.Fatalf("session cookie path/max-age = %q/%d", session.Path, session.MaxAge)
	}
}
