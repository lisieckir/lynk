package health

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lisieckir/lynk/internal/database"
)

func openTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lynk.db")
	db, err := database.Open(context.Background(), database.Config{Path: path})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, path
}

type fakeChecker struct {
	ping func(context.Context) error
}

func (f fakeChecker) PingContext(ctx context.Context) error { return f.ping(ctx) }

func TestHandlerHealthyDatabase(t *testing.T) {
	db, path := openTestDB(t)

	rec := httptest.NewRecorder()
	NewHandler(db).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, Path, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != okBody {
		t.Errorf("body = %q, want %q", got, okBody)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q", got)
	}
	if strings.Contains(rec.Body.String(), path) {
		t.Errorf("healthy body leaked database path %q", path)
	}
}

func TestHandlerUnavailableDatabase(t *testing.T) {
	db, path := openTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatalf("close test database: %v", err)
	}

	rec := httptest.NewRecorder()
	NewHandler(db).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, Path, nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Body.String(); got != unavailableBody {
		t.Errorf("body = %q, want %q", got, unavailableBody)
	}
	for _, leaked := range []string{path, "lynk.db", "closed", "sql:"} {
		if strings.Contains(rec.Body.String(), leaked) {
			t.Errorf("unavailable body leaked %q: %s", leaked, rec.Body.String())
		}
	}
}

func TestHandlerNilCheckerIsUnavailable(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, Path, nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Body.String(); got != unavailableBody {
		t.Errorf("body = %q, want %q", got, unavailableBody)
	}
}

func TestHandlerTypedNilDatabaseIsUnavailable(t *testing.T) {
	var db *sql.DB
	rec := httptest.NewRecorder()
	NewHandler(db).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, Path, nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Body.String(); got != unavailableBody {
		t.Errorf("body = %q, want %q", got, unavailableBody)
	}
}

type nilPointerChecker struct{}

func (*nilPointerChecker) PingContext(context.Context) error {
	panic("typed nil checker must not be called")
}

func TestHandlerTypedNilCheckerIsUnavailable(t *testing.T) {
	var checker *nilPointerChecker
	rec := httptest.NewRecorder()
	NewHandler(checker).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, Path, nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Body.String(); got != unavailableBody {
		t.Errorf("body = %q, want %q", got, unavailableBody)
	}
}

func TestHandlerDoesNotExposeCheckerError(t *testing.T) {
	const secret = "/run/secrets/lynk token=abcdef0123456789"
	checker := fakeChecker{ping: func(context.Context) error {
		return errors.New("connect " + secret)
	}}

	rec := httptest.NewRecorder()
	NewHandler(checker).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, Path, nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Body.String(); got != unavailableBody {
		t.Errorf("body = %q, want %q", got, unavailableBody)
	}
	if strings.Contains(rec.Body.String(), secret) || strings.Contains(rec.Body.String(), "connect") {
		t.Errorf("body exposed internal error: %s", rec.Body.String())
	}
}

func TestHandlerReadinessTimeout(t *testing.T) {
	checker := fakeChecker{ping: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}}

	start := time.Now()
	rec := httptest.NewRecorder()
	NewHandlerWithTimeout(checker, 20*time.Millisecond).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, Path, nil))
	elapsed := time.Since(start)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if elapsed > time.Second {
		t.Errorf("readiness check took %v despite 20ms timeout", elapsed)
	}
	if got := rec.Body.String(); got != unavailableBody {
		t.Errorf("body = %q, want %q", got, unavailableBody)
	}
}

func TestHandlerMethodHandling(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		wantStatus int
		wantBody   string
		wantAllow  string
	}{
		{"get", http.MethodGet, http.StatusOK, okBody, ""},
		{"head", http.MethodHead, http.StatusOK, "", ""},
		{"post", http.MethodPost, http.StatusMethodNotAllowed, methodNotAllowedBody, "GET, HEAD"},
		{"delete", http.MethodDelete, http.StatusMethodNotAllowed, methodNotAllowedBody, "GET, HEAD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, _ := openTestDB(t)
			rec := httptest.NewRecorder()
			NewHandler(db).ServeHTTP(rec, httptest.NewRequest(tt.method, Path, nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
			if got := rec.Header().Get("Allow"); got != tt.wantAllow {
				t.Errorf("Allow = %q, want %q", got, tt.wantAllow)
			}
		})
	}
}
