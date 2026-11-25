package server

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lisieckir/lynk/internal/database"
	"github.com/lisieckir/lynk/internal/health"
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

func newListener(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on ephemeral port: %v", err)
	}
	return ln
}

func waitForServe(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down within 3s")
		return nil
	}
}

func getPath(t *testing.T, addr, path string) (int, string) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s response: %v", path, err)
	}
	return resp.StatusCode, string(body)
}

func getHealth(t *testing.T, addr string) (int, string) {
	return getPath(t, addr, health.Path)
}

type blockingChecker struct{}

func (blockingChecker) PingContext(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestDefaultConfigReadinessTimeout(t *testing.T) {
	if got := DefaultConfig(":9090").ReadinessTimeout; got != health.DefaultTimeout {
		t.Fatalf("ReadinessTimeout = %v, want %v", got, health.DefaultTimeout)
	}
}

func TestHandlerWithCheckerUsesConfiguredReadinessTimeout(t *testing.T) {
	h := handlerWithChecker(blockingChecker{}, 25*time.Millisecond)

	start := time.Now()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, health.Path, nil))
	elapsed := time.Since(start)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if elapsed > time.Second {
		t.Errorf("readiness check took %v despite 25ms configured timeout", elapsed)
	}
}

func TestCleanupWithAttemptsCloseAfterShutdownError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var calls []string
	err := cleanupWith(ctx, time.Second,
		func(context.Context) error {
			calls = append(calls, "shutdown")
			return errors.New("shutdown failed")
		},
		func() error {
			calls = append(calls, "close")
			return errors.New("close failed")
		},
	)
	if err == nil {
		t.Fatal("cleanupWith succeeded with shutdown and close errors")
	}
	for _, want := range []string{"shut down http server: shutdown failed", "close failed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if len(calls) != 2 || calls[0] != "shutdown" || calls[1] != "close" {
		t.Errorf("cleanup order = %v, want [shutdown close]", calls)
	}
}

func TestCleanupWithReportsCloseErrorAfterSuccessfulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cleanupWith(ctx, time.Second,
		func(context.Context) error { return nil },
		func() error { return errors.New("close failed") },
	)
	if err == nil || !strings.Contains(err.Error(), "close failed") {
		t.Fatalf("cleanupWith did not report close error: %v", err)
	}
	if !strings.Contains(err.Error(), "shutdown cleanup") {
		t.Errorf("error lacks cleanup context: %v", err)
	}
}

func TestServeHealthyDatabaseAndGracefulShutdown(t *testing.T) {
	db, dbPath := openTestDB(t)
	ln := newListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, DefaultConfig(ln.Addr().String()), ln, db, logger)
	}()

	status, body := getHealth(t, ln.Addr().String())
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if want := "{\"status\":\"ok\"}\n"; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}

	cancel()
	if err := waitForServe(t, done); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if err := db.PingContext(context.Background()); err == nil {
		t.Error("database remained open after graceful shutdown")
	}

	output := logs.String()
	for _, want := range []string{"http server started", "http server shutting down", "shutdown complete"} {
		if !strings.Contains(output, want) {
			t.Errorf("logs missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{dbPath, "lynk.db", "database_path"} {
		if strings.Contains(output, leaked) {
			t.Errorf("logs leaked %q:\n%s", leaked, output)
		}
	}
}

func TestServeUnavailableDatabase(t *testing.T) {
	db, dbPath := openTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatalf("close test database: %v", err)
	}
	ln := newListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, DefaultConfig(ln.Addr().String()), ln, db, nil)
	}()

	status, body := getHealth(t, ln.Addr().String())
	if status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", status, http.StatusServiceUnavailable)
	}
	const wantBody = "{\"status\":\"unavailable\"}\n"
	if body != wantBody {
		t.Errorf("body = %q, want %q", body, wantBody)
	}
	if strings.Contains(body, dbPath) || strings.Contains(body, "lynk.db") || strings.Contains(body, "closed") {
		t.Errorf("response leaked internal details: %q", body)
	}
	statusAgain, bodyAgain := getHealth(t, ln.Addr().String())
	if statusAgain != status || bodyAgain != body {
		t.Errorf("second response differs: (%d, %q), want (%d, %q)", statusAgain, bodyAgain, status, body)
	}

	cancel()
	if err := waitForServe(t, done); err != nil {
		t.Fatalf("Serve: %v", err)
	}
}

func TestServeCanonicalHealthPath(t *testing.T) {
	db, _ := openTestDB(t)
	ln := newListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, DefaultConfig(ln.Addr().String()), ln, db, nil)
	}()

	status, body := getHealth(t, ln.Addr().String())
	if status != http.StatusOK {
		t.Fatalf("GET %s status = %d, want %d", health.Path, status, http.StatusOK)
	}
	if want := "{\"status\":\"ok\"}\n"; body != want {
		t.Errorf("GET %s body = %q, want %q", health.Path, body, want)
	}

	for i := 0; i < 2; i++ {
		slashStatus, slashBody := getPath(t, ln.Addr().String(), health.Path+"/")
		if slashStatus != http.StatusNotFound {
			t.Errorf("GET %s/ status = %d, want %d", health.Path, slashStatus, http.StatusNotFound)
		}
		if strings.Contains(slashBody, "lynk.db") || strings.Contains(slashBody, "token") {
			t.Errorf("GET %s/ leaked internal details: %q", health.Path, slashBody)
		}
	}

	cancel()
	if err := waitForServe(t, done); err != nil {
		t.Fatalf("Serve: %v", err)
	}
}

func TestServeClosesDatabaseWhenServeFails(t *testing.T) {
	db, _ := openTestDB(t)
	ln := newListener(t)
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener before Serve: %v", err)
	}

	err := Serve(context.Background(), DefaultConfig(ln.Addr().String()), ln, db, nil)
	if err == nil {
		t.Fatal("Serve succeeded with a closed listener")
	}
	if !strings.Contains(err.Error(), "serve http") {
		t.Errorf("error does not identify the serve failure: %v", err)
	}
	if err := db.PingContext(context.Background()); err == nil {
		t.Error("database remained open after serve failure")
	}
}

func TestRunListenFailureClosesDatabase(t *testing.T) {
	db, _ := openTestDB(t)
	occupied := newListener(t)
	defer occupied.Close()

	err := Run(context.Background(), DefaultConfig(occupied.Addr().String()), db, nil)
	if err == nil {
		t.Fatal("Run succeeded with an occupied listen address")
	}
	if !strings.Contains(err.Error(), "listen on http address") {
		t.Errorf("error does not identify the listen failure: %v", err)
	}
	if err := db.PingContext(context.Background()); err == nil {
		t.Error("database remained open after listen failure")
	}
}

func TestServeRejectsNilArgumentsAndClosesDatabase(t *testing.T) {
	ln := newListener(t)
	defer ln.Close()

	db, _ := openTestDB(t)
	if err := Serve(nil, Config{}, ln, db, nil); err == nil {
		t.Error("Serve accepted a nil context")
	}
	if err := db.PingContext(context.Background()); err == nil {
		t.Error("database remained open after nil-context error")
	}

	db2, _ := openTestDB(t)
	if err := Serve(context.Background(), Config{}, nil, db2, nil); err == nil {
		t.Error("Serve accepted a nil listener")
	}
	if err := db2.PingContext(context.Background()); err == nil {
		t.Error("database remained open after nil-listener error")
	}
}
