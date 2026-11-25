package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lisieckir/lynk/internal/config"
)

func lookupFrom(values map[string]string) config.LookupFunc {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

func freeHTTPAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve HTTP address: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("release HTTP address: %v", err)
	}
	return addr
}

func tempValues(t *testing.T, addr string) map[string]string {
	t.Helper()
	dir := t.TempDir()
	return map[string]string{
		"LYNK_DATA_DIR":      dir,
		"LYNK_DATABASE_PATH": filepath.Join(dir, "lynk.db"),
		"LYNK_HTTP_ADDR":     addr,
	}
}

// TestRunValidConfig keeps the P1-001 entrypoint smoke test: the command
// package compiles and run returns cleanly. A cancelled context stands in for
// SIGTERM because main now installs real process signal handlers.
func TestRunValidConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var out bytes.Buffer
	if err := run(ctx, lookupFrom(tempValues(t, freeHTTPAddr(t))), &out); err != nil {
		t.Fatalf("run with default configuration: %v", err)
	}
	got := out.String()
	for _, want := range []string{`"msg":"configuration loaded"`, `"msg":"shutdown complete"`, `"env":"production"`} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestRunStartupLogOmitsFilesystemPaths(t *testing.T) {
	values := tempValues(t, "127.0.0.1:9091")
	values["LYNK_BASE_URL"] = "https://lynk.example.org"
	dataDir := values["LYNK_DATA_DIR"]
	database := values["LYNK_DATABASE_PATH"]
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var out bytes.Buffer
	if err := run(ctx, lookupFrom(values), &out); err != nil {
		t.Fatalf("run with custom paths: %v", err)
	}
	got := out.String()
	for _, absent := range []string{dataDir, database, "lynk.db", `"data_dir"`, `"database_path"`} {
		if strings.Contains(got, absent) {
			t.Errorf("startup log leaked %q:\n%s", absent, got)
		}
	}
	for _, want := range []string{
		`"msg":"configuration loaded"`,
		`"http_addr":"127.0.0.1:9091"`,
		`"base_url":"https://lynk.example.org"`,
		`"log_level":"INFO"`,
		`"msg":"shutdown complete"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("startup log missing %q:\n%s", want, got)
		}
	}
}

func TestRunInvalidConfigFailsBeforeLogging(t *testing.T) {
	lookup := func(name string) (string, bool) {
		if name == "LYNK_HTTP_ADDR" {
			return "localhost", true
		}
		return "", false
	}

	var out bytes.Buffer
	err := run(context.Background(), lookup, &out)
	if err == nil {
		t.Fatal("run succeeded with invalid LYNK_HTTP_ADDR")
	}
	if !strings.Contains(err.Error(), "LYNK_HTTP_ADDR") {
		t.Errorf("error does not name the invalid variable: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("logs emitted for invalid configuration: %s", out.String())
	}
}

func TestRunInvalidHealthTimeoutFailsBeforeLogging(t *testing.T) {
	lookup := func(name string) (string, bool) {
		if name == "LYNK_HEALTH_TIMEOUT" {
			return "0s", true
		}
		return "", false
	}

	var out bytes.Buffer
	err := run(context.Background(), lookup, &out)
	if err == nil {
		t.Fatal("run succeeded with invalid LYNK_HEALTH_TIMEOUT")
	}
	if !strings.Contains(err.Error(), "LYNK_HEALTH_TIMEOUT") {
		t.Errorf("error does not name the invalid variable: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("logs emitted for invalid configuration: %s", out.String())
	}
}

func TestRunServesHealthUntilCancelled(t *testing.T) {
	addr := freeHTTPAddr(t)
	values := tempValues(t, addr)
	values["LYNK_HEALTH_TIMEOUT"] = "750ms"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, lookupFrom(values), &out)
	}()

	client := &http.Client{Timeout: 2 * time.Second}
	var resp *http.Response
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for {
		resp, err = client.Get("http://" + addr + "/health")
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("health endpoint never became available: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read health response: %v", err)
	}
	if got, want := string(body), "{\"status\":\"ok\"}\n"; got != want {
		t.Errorf("health body = %q, want %q", got, want)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run did not shut down within 3s")
	}
	got := out.String()
	for _, want := range []string{`"msg":"http server started"`, `"msg":"shutdown complete"`} {
		if !strings.Contains(got, want) {
			t.Errorf("logs missing %q:\n%s", want, got)
		}
	}
	for _, absent := range []string{values["LYNK_DATABASE_PATH"], "lynk.db"} {
		if strings.Contains(got, absent) {
			t.Errorf("logs leaked %q:\n%s", absent, got)
		}
	}
}
