package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseAuthorizationErrorsIsUnauthorized(t *testing.T) {
	_, err := ParseAuthorization("")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected errors.Is(err, ErrUnauthorized) true, got false (err=%v)", err)
	}
	_, err = ParseAuthorization("Basic abc")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected errors.Is(err, ErrUnauthorized) true, got false (err=%v)", err)
	}
}

func TestLookupByTokenErrorsIsUnauthorized(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	// invalid format
	_, err := LookupByToken(ctx, db, "short")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestRequireAuthMiddleware(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	info, err := CreateToken(ctx, db, "dave", "m1")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	// no header
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	called := false
	handler := RequireAuth(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))
	handler.ServeHTTP(rec, req)
	if called {
		t.Error("handler should not be called on auth failure")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Errorf("body error = %q, want unauthorized", body["error"])
	}

	// good token
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("Authorization", "Token "+info.Token)
	rec2 := httptest.NewRecorder()
	var gotUserID int64
	handler2 := RequireAuth(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := UserIDFromContext(r.Context())
		if !ok {
			t.Error("expected user id in context")
		}
		gotUserID = uid
		w.WriteHeader(200)
	}))
	handler2.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec2.Code)
	}
	if gotUserID != info.UserID {
		t.Errorf("user id = %d, want %d", gotUserID, info.UserID)
	}
}
