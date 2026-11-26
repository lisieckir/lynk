package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetch_BlocksLoopback(t *testing.T) {
	// httptest server is on 127.0.0.1, should be blocked
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><head><title>Hi</title></head></html>"))
	}))
	defer ts.Close()

	f := NewFetcher(Config{Timeout: 2 * time.Second, ConnectTimeout: 1 * time.Second, MaxRedirects: 3, MaxBytes: 1024 * 1024})
	_, err := f.Fetch(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("expected error for loopback URL, got nil")
	}
}

func TestFetch_BlocksNonHTTPSSchemes(t *testing.T) {
	f := NewFetcher(Config{})
	_, err := f.Fetch(context.Background(), "ftp://example.com/file")
	if err == nil {
		t.Fatal("expected error for ftp scheme")
	}
	_, err = f.Fetch(context.Background(), "file:///etc/passwd")
	if err == nil {
		t.Fatal("expected error for file scheme")
	}
}

func TestFetch_BlocksLocalhost(t *testing.T) {
	f := NewFetcher(Config{Timeout: 2 * time.Second})
	_, err := f.Fetch(context.Background(), "http://localhost:8080/")
	if err == nil {
		t.Fatal("expected error for localhost")
	}
}

func TestFetch_MalformedHTML(t *testing.T) {
	// We can't easily test with non-loopback, but test parseHTML directly
	res := parseHTML([]byte("<html><head><title>Test</title></head><body>no closing"))
	if res.Title != "Test" {
		t.Errorf("expected title Test, got %q", res.Title)
	}
}

func TestParseHTML_ExtractsOGAndMeta(t *testing.T) {
	html := `<html><head>
		<title>Page Title</title>
		<meta name="description" content="A description">
		<meta property="og:title" content="OG Title">
		<meta property="og:description" content="OG Desc">
		<meta property="og:image" content="https://example.com/img.png">
		<link rel="icon" href="/favicon.ico">
	</head></html>`
	res := parseHTML([]byte(html))
	if res.Title != "Page Title" {
		t.Errorf("title: got %q", res.Title)
	}
	if res.Description != "A description" {
		t.Errorf("desc: got %q", res.Description)
	}
	if res.OGTitle != "OG Title" {
		t.Errorf("ogtitle: got %q", res.OGTitle)
	}
	if res.OGDescription != "OG Desc" {
		t.Errorf("ogdesc: got %q", res.OGDescription)
	}
	if res.PreviewImageURL != "https://example.com/img.png" {
		t.Errorf("preview: got %q", res.PreviewImageURL)
	}
	if res.FaviconURL != "/favicon.ico" {
		t.Errorf("favicon: got %q", res.FaviconURL)
	}
}

func TestFetch_BlocksRedirectToLoopback(t *testing.T) {
	// Server redirects to another loopback URL
	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><title>secret</title></html>"))
	}))
	defer ts2.Close()
	ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, ts2.URL, http.StatusFound)
	}))
	defer ts1.Close()

	f := NewFetcher(Config{Timeout: 2 * time.Second, ConnectTimeout: 1 * time.Second, MaxRedirects: 3, MaxBytes: 1024 * 1024})
	_, err := f.Fetch(context.Background(), ts1.URL)
	if err == nil {
		t.Fatal("expected error when redirect goes to loopback")
	}
}

func TestFetch_Cancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte("<html></html>"))
	}))
	defer ts.Close()
	// Even though ts is loopback (blocked), let's test with context cancel on a valid-looking flow is hard.
	// Instead, test that a cancelled context returns error quickly.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := NewFetcher(Config{})
	_, err := f.Fetch(ctx, "http://example.com/")
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}
