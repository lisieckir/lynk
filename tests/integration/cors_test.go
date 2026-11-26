package integration_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lisieckir/lynk/internal/api/linkding"
	"github.com/lisieckir/lynk/internal/auth"
	"github.com/lisieckir/lynk/internal/cors"
	"github.com/lisieckir/lynk/internal/database"
	"github.com/lisieckir/lynk/internal/health"
	"github.com/lisieckir/lynk/internal/server"
)

const allowedWebOrigin = "https://app.example.com"

type corsFixture struct {
	url    string
	client *http.Client
	token  string
}

// newCORSFixture drives the same real SQLite database, real API handler and
// real net/http server used in production through an httptest listener.
func newCORSFixture(t *testing.T, opts cors.Options) *corsFixture {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, database.Config{Path: filepath.Join(t.TempDir(), "cors.db")})
	if err != nil {
		t.Fatalf("open real migrated SQLite: %v", err)
	}
	srv := httptest.NewServer(server.HandlerWithAPIAndCORS(db, linkding.NewHandler(db, nil), health.DefaultTimeout, opts, nil))
	t.Cleanup(func() {
		srv.Close()
		_ = db.Close()
	})
	created, err := auth.CreateToken(ctx, db, "alice", "cors-integration")
	if err != nil {
		t.Fatalf("create integration token: %v", err)
	}
	return &corsFixture{
		url:    srv.URL,
		client: &http.Client{Timeout: 2 * time.Second},
		token:  "Token " + created.Token,
	}
}

func integrationCORSOptions(headers string) cors.Options {
	return cors.Options{
		AllowedOrigins: allowedWebOrigin,
		AllowedMethods: "GET, POST",
		AllowedHeaders: headers,
		MaxAgeSeconds:  600,
	}
}

func corsRequest(t *testing.T, f *corsFixture, method, path string, headers map[string]string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequest(method, f.url+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return resp.StatusCode, resp.Header.Clone(), string(body)
}

func assertNoCORSHeaders(t *testing.T, h http.Header) {
	t.Helper()
	for _, name := range []string{
		"Access-Control-Allow-Origin", "Access-Control-Allow-Methods",
		"Access-Control-Allow-Headers", "Access-Control-Max-Age",
		"Access-Control-Allow-Credentials",
	} {
		if got := h.Get(name); got != "" {
			t.Errorf("%s = %q, want no CORS header", name, got)
		}
	}
}

func varyContains(h http.Header, want string) bool {
	for _, value := range h.Values("Vary") {
		for _, part := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), want) {
				return true
			}
		}
	}
	return false
}

func TestCORSAllowedPreflightWithoutToken(t *testing.T) {
	f := newCORSFixture(t, integrationCORSOptions("Content-Type, Authorization"))
	status, h, body := corsRequest(t, f, http.MethodOptions, "/api/bookmarks/", map[string]string{
		"Origin":                         allowedWebOrigin,
		"Access-Control-Request-Method":  http.MethodPost,
		"Access-Control-Request-Headers": "authorization, content-type",
	})

	if status != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body = %q", status, body)
	}
	if body != "" {
		t.Errorf("preflight body = %q, want empty", body)
	}
	if got := h.Get("Access-Control-Allow-Origin"); got != allowedWebOrigin {
		t.Errorf("Allow-Origin = %q, want %q", got, allowedWebOrigin)
	}
	if got := h.Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Errorf("Allow-Methods = %q, want POST", got)
	}
	allowedHeaders := h.Get("Access-Control-Allow-Headers")
	if !strings.Contains(allowedHeaders, "Authorization") || !strings.Contains(allowedHeaders, "Content-Type") {
		t.Errorf("Allow-Headers = %q, want Authorization and Content-Type", allowedHeaders)
	}
	if got := h.Get("Access-Control-Max-Age"); got != "600" {
		t.Errorf("Max-Age = %q, want 600", got)
	}
	if !varyContains(h, "Origin") {
		t.Errorf("Vary = %v, want Origin", h.Values("Vary"))
	}
	if got := h.Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Allow-Credentials = %q, want none", got)
	}
	if got := h.Get("Set-Cookie"); got != "" {
		t.Errorf("Set-Cookie = %q, want none", got)
	}
	if strings.Contains(body, "Token") || strings.Contains(h.Get("Access-Control-Allow-Origin"), "*") {
		t.Error("preflight leaked a token or used a wildcard origin")
	}
}

func TestCORSDisallowedPreflightIsRejected(t *testing.T) {
	f := newCORSFixture(t, integrationCORSOptions("Content-Type, Authorization"))
	tests := map[string]map[string]string{
		"origin": {
			"Origin":                         "https://evil.example.com",
			"Access-Control-Request-Method":  http.MethodPost,
			"Access-Control-Request-Headers": "content-type",
		},
		"method": {
			"Origin":                        allowedWebOrigin,
			"Access-Control-Request-Method": http.MethodDelete,
		},
	}
	for name, headers := range tests {
		t.Run(name, func(t *testing.T) {
			status, h, body := corsRequest(t, f, http.MethodOptions, "/api/bookmarks/", headers)
			if status != http.StatusForbidden {
				t.Fatalf("status = %d, want 403; body = %q", status, body)
			}
			if body != "" {
				t.Errorf("error body = %q, want empty", body)
			}
			assertNoCORSHeaders(t, h)
		})
	}
}

func TestCORSAuthorizationRequiresExplicitPolicy(t *testing.T) {
	f := newCORSFixture(t, integrationCORSOptions("Content-Type"))
	status, h, body := corsRequest(t, f, http.MethodOptions, "/api/bookmarks/", map[string]string{
		"Origin":                         allowedWebOrigin,
		"Access-Control-Request-Method":  http.MethodPost,
		"Access-Control-Request-Headers": "authorization",
	})
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %q", status, body)
	}
	assertNoCORSHeaders(t, h)
	if strings.Contains(body, allowedWebOrigin) {
		t.Error("error echoed the origin")
	}
}

func TestCORSSimpleRequestsKeepAPIAuth(t *testing.T) {
	f := newCORSFixture(t, integrationCORSOptions("Content-Type, Authorization"))

	status, h, body := corsRequest(t, f, http.MethodGet, "/api/user/profile/", map[string]string{
		"Origin":        allowedWebOrigin,
		"Authorization": f.token,
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", status, body)
	}
	if !strings.Contains(body, `"theme"`) {
		t.Errorf("profile body = %q, want the real profile payload", body)
	}
	if got := h.Get("Access-Control-Allow-Origin"); got != allowedWebOrigin {
		t.Errorf("Allow-Origin = %q, want %q", got, allowedWebOrigin)
	}
	if !varyContains(h, "Origin") {
		t.Errorf("Vary = %v, want Origin", h.Values("Vary"))
	}
	if got := h.Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Allow-Credentials = %q, want none", got)
	}
	if strings.Contains(h.Get("Access-Control-Allow-Origin"), "*") {
		t.Error("wildcard origin emitted")
	}

	// A disallowed origin still reaches the API but a browser cannot read the
	// response because no Access-Control-* header is present. Non-browser
	// clients and host-permission extensions are not broken.
	status, h, body = corsRequest(t, f, http.MethodGet, "/api/user/profile/", map[string]string{
		"Origin":        "https://evil.example.com",
		"Authorization": f.token,
	})
	if status != http.StatusOK || !strings.Contains(body, `"theme"`) {
		t.Fatalf("status = %d, body = %q; want a valid authenticated response", status, body)
	}
	assertNoCORSHeaders(t, h)

	for name, headers := range map[string]map[string]string{
		"without token": {"Origin": allowedWebOrigin},
		"no origin":     {},
	} {
		t.Run(name, func(t *testing.T) {
			status, _, body := corsRequest(t, f, http.MethodGet, "/api/user/profile/", headers)
			if status != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401; body = %q", status, body)
			}
			if !strings.Contains(body, "unauthorized") {
				t.Errorf("401 body = %q, want the stable auth response", body)
			}
		})
	}
}

func TestCORSPublicHealthIsUnchanged(t *testing.T) {
	f := newCORSFixture(t, integrationCORSOptions("Content-Type, Authorization"))

	for name, headers := range map[string]map[string]string{
		"anonymous":      {},
		"allowed origin": {"Origin": allowedWebOrigin},
		"other origin":   {"Origin": "https://evil.example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			status, h, body := corsRequest(t, f, http.MethodGet, health.Path, headers)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %q", status, body)
			}
			if want := "{\"status\":\"ok\"}\n"; body != want {
				t.Errorf("body = %q, want %q", body, want)
			}
			assertNoCORSHeaders(t, h)
		})
	}

	status, h, _ := corsRequest(t, f, http.MethodOptions, health.Path, map[string]string{
		"Origin":                        allowedWebOrigin,
		"Access-Control-Request-Method": http.MethodGet,
	})
	if status != http.StatusMethodNotAllowed {
		t.Errorf("OPTIONS %s status = %d, want 405", health.Path, status)
	}
	if got := h.Get("Allow"); got != "GET, HEAD" {
		t.Errorf("Allow = %q, want %q", got, "GET, HEAD")
	}
	assertNoCORSHeaders(t, h)
}
