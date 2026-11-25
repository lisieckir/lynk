package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func emptyLookup(string) (string, bool) { return "", false }

// TestRunValidConfig keeps the P1-001 entrypoint smoke test: the command
// package compiles and run returns cleanly. A cancelled context stands in for
// SIGTERM because main now installs real process signal handlers.
func TestRunValidConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var out bytes.Buffer
	if err := run(ctx, emptyLookup, &out); err != nil {
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
	const (
		dataDir  = "/var/lib/lynk-private"
		database = "/var/lib/lynk-private/lynk.db"
	)
	lookup := func(name string) (string, bool) {
		switch name {
		case "LYNK_DATA_DIR":
			return dataDir, true
		case "LYNK_DATABASE_PATH":
			return database, true
		case "LYNK_HTTP_ADDR":
			return "127.0.0.1:9091", true
		case "LYNK_BASE_URL":
			return "https://lynk.example.org", true
		}
		return "", false
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var out bytes.Buffer
	if err := run(ctx, lookup, &out); err != nil {
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
