package cors

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func options(origins, methods, headers string) Options {
	return Options{
		AllowedOrigins: origins,
		AllowedMethods: methods,
		AllowedHeaders: headers,
		MaxAgeSeconds:  DefaultMaxAgeSeconds,
	}
}

func newPolicy(t *testing.T, opts Options) *Policy {
	t.Helper()
	p, err := New(opts)
	if err != nil {
		t.Fatalf("New(%+v): %v", opts, err)
	}
	return p
}

func TestDefaultOptionsIsFailClosed(t *testing.T) {
	p := newPolicy(t, DefaultOptions())
	if p.active {
		t.Fatal("default policy is active; no origin should be allowed unless configured")
	}
	if p.maxAge != DefaultMaxAgeSeconds {
		t.Errorf("max age = %d, want %d", p.maxAge, DefaultMaxAgeSeconds)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	h := p.Handler(next)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/bookmarks/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d (policy must be a no-op)", rec.Code, http.StatusTeapot)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("default policy emitted Allow-Origin %q", got)
	}
}

func TestNewNormalizesExplicitPolicy(t *testing.T) {
	p := newPolicy(t, options(" HTTPS://App.Example.COM/ , https://api.example.com:8443", "post,get,POST", "authorization, content-type, Authorization"))
	if !p.active {
		t.Fatal("configured policy is inactive")
	}
	if _, ok := p.origins["https://app.example.com"]; !ok {
		t.Errorf("origin not normalized to https://app.example.com: %v", p.origins)
	}
	if _, ok := p.origins["https://api.example.com:8443"]; !ok {
		t.Errorf("explicit port origin missing: %v", p.origins)
	}
	if len(p.origins) != 2 {
		t.Errorf("origins = %v, want 2 entries", p.origins)
	}
	if p.methodList != "GET, POST" {
		t.Errorf("method list = %q, want %q", p.methodList, "GET, POST")
	}
	if p.headerList != "Authorization, Content-Type" {
		t.Errorf("header list = %q, want %q", p.headerList, "Authorization, Content-Type")
	}
	if _, ok := p.headers["Authorization"]; !ok {
		t.Error("Authorization was not explicitly allowed")
	}
}

func TestNewRejectsUnsafeConfiguration(t *testing.T) {
	base := options("https://app.example.com", "GET, POST", "Content-Type")
	tests := []struct {
		name string
		opts Options
	}{
		{"wildcard origin", func() Options { o := base; o.AllowedOrigins = "*"; return o }()},
		{"wildcard subdomain", func() Options { o := base; o.AllowedOrigins = "https://*.example.com"; return o }()},
		{"origin with path", func() Options { o := base; o.AllowedOrigins = "https://example.com/lynk"; return o }()},
		{"origin with credentials", func() Options { o := base; o.AllowedOrigins = "https://user@example.com"; return o }()},
		{"origin with query", func() Options { o := base; o.AllowedOrigins = "https://example.com/?x=1"; return o }()},
		{"non-http origin", func() Options { o := base; o.AllowedOrigins = "ftp://example.com"; return o }()},
		{"unparseable origin", func() Options { o := base; o.AllowedOrigins = "https://[::1"; return o }()},
		{"wildcard method", func() Options { o := base; o.AllowedMethods = "*"; return o }()},
		{"unsupported method", func() Options { o := base; o.AllowedMethods = "TRACE"; return o }()},
		{"wildcard header", func() Options { o := base; o.AllowedHeaders = "*"; return o }()},
		{"invalid header name", func() Options { o := base; o.AllowedHeaders = "X-Bad Header"; return o }()},
		{"negative max age", func() Options { o := base; o.MaxAgeSeconds = -1; return o }()},
		{"max age above cap", func() Options { o := base; o.MaxAgeSeconds = MaxMaxAgeSeconds + 1; return o }()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(tt.opts)
			if err == nil {
				t.Fatalf("New(%+v) succeeded with policy %+v", tt.opts, p)
			}
		})
	}
}

func servePreflight(t *testing.T, p *Policy, rec *httptest.ResponseRecorder, origin, method, headers string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodOptions, "/api/bookmarks/", nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", method)
	if headers != "" {
		req.Header.Set("Access-Control-Request-Headers", headers)
	}
	p.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("preflight reached the authenticated API handler")
	})).ServeHTTP(rec, req)
}

func TestAllowedPreflightNeedsNoTokenAndNoData(t *testing.T) {
	p := newPolicy(t, options("https://app.example.com", "GET, POST", "Content-Type, Authorization"))
	rec := httptest.NewRecorder()
	servePreflight(t, p, rec, "https://app.example.com", "POST", "authorization, content-type")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %q", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("preflight body = %q, want empty", rec.Body.String())
	}
	h := rec.Header()
	if got := h.Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("Allow-Origin = %q", got)
	}
	if got := h.Get("Access-Control-Allow-Methods"); got != "GET, POST" {
		t.Errorf("Allow-Methods = %q", got)
	}
	if got := h.Get("Access-Control-Allow-Headers"); got != "Authorization, Content-Type" {
		t.Errorf("Allow-Headers = %q", got)
	}
	if got := h.Get("Access-Control-Max-Age"); got != "600" {
		t.Errorf("Max-Age = %q, want 600", got)
	}
	if got := h.Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("unexpected Allow-Credentials = %q", got)
	}
	if got := h.Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary = %q, want Origin", got)
	}
	for _, name := range []string{"Access-Control-Request-Method", "Access-Control-Request-Headers"} {
		if !containsVary(h.Values("Vary"), name) {
			t.Errorf("Vary %v does not contain %s", h.Values("Vary"), name)
		}
	}
	if strings.Contains(rec.Body.String(), "Token") || h.Get("Set-Cookie") != "" {
		t.Error("preflight leaked credentials")
	}
}

func TestPreflightRejectedWithoutLeakingConfiguration(t *testing.T) {
	p := newPolicy(t, options("https://app.example.com", "GET, POST", "Content-Type"))
	tests := []struct {
		name    string
		origin  string
		method  string
		headers string
	}{
		{"disallowed origin", "https://evil.example.com", "POST", "content-type"},
		{"disallowed method", "https://app.example.com", "DELETE", ""},
		{"authorization not configured", "https://app.example.com", "POST", "authorization"},
		{"unknown header", "https://app.example.com", "POST", "x-secret"},
		{"null origin", "null", "GET", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			servePreflight(t, p, rec, tt.origin, tt.method, tt.headers)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("error body = %q, want empty", rec.Body.String())
			}
			for _, name := range []string{
				"Access-Control-Allow-Origin", "Access-Control-Allow-Methods",
				"Access-Control-Allow-Headers", "Access-Control-Max-Age",
			} {
				if got := rec.Header().Get(name); got != "" {
					t.Errorf("%s = %q on rejected preflight", name, got)
				}
			}
			if strings.Contains(rec.Body.String(), tt.origin) {
				t.Error("error echoed the origin")
			}
		})
	}
}

func TestSimpleRequestsOnlyGetCORSForAllowedOrigins(t *testing.T) {
	p := newPolicy(t, options("https://app.example.com", "GET", "Content-Type"))
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("payload"))
	})
	tests := []struct {
		name       string
		origin     string
		wantOrigin string
	}{
		{"allowed", "https://app.example.com", "https://app.example.com"},
		{"disallowed", "https://evil.example.com", ""},
		{"no origin", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/bookmarks/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()
			p.Handler(next).ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || rec.Body.String() != "payload" {
				t.Fatalf("response = (%d, %q), want (200, payload)", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.wantOrigin {
				t.Errorf("Allow-Origin = %q, want %q", got, tt.wantOrigin)
			}
			if !containsVary(rec.Header().Values("Vary"), "Origin") {
				t.Errorf("Vary = %v, want Origin", rec.Header().Values("Vary"))
			}
			if strings.Contains(rec.Header().Get("Access-Control-Allow-Origin"), "*") {
				t.Error("wildcard origin emitted")
			}
		})
	}
}

func TestPlainOptionsWithoutRequestMethodKeepsOriginalBehavior(t *testing.T) {
	p := newPolicy(t, options("https://app.example.com", "GET, POST", "Content-Type"))
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", "GET, POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	req := httptest.NewRequest(http.MethodOptions, "/api/bookmarks/", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()
	p.Handler(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("Allow-Origin = %q on a non-preflight OPTIONS", got)
	}
}

func TestNilPolicyIsNoOp(t *testing.T) {
	var p *Policy
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	rec := httptest.NewRecorder()
	p.Handler(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/bookmarks/", nil))
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func containsVary(values []string, want string) bool {
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), want) {
				return true
			}
		}
	}
	return false
}
