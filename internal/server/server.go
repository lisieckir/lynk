// Package server wires Lynk's minimal net/http server and its lifecycle.
//
// Phase 1 registers only the health route. API and UI handlers are added by
// later phases without changing the shutdown contract established here.
package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/lisieckir/lynk/internal/health"
	"github.com/lisieckir/lynk/internal/shutdown"
)

const (
	DefaultReadHeaderTimeout = 5 * time.Second
	DefaultReadTimeout       = 10 * time.Second
	DefaultWriteTimeout      = 10 * time.Second
	DefaultIdleTimeout       = 60 * time.Second
)

// Config holds HTTP server settings. Zero or negative timeouts select the
// documented defaults; ShutdownTimeout uses shutdown.DefaultTimeout.
type Config struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	ReadinessTimeout  time.Duration
}

// DefaultConfig returns safe timeouts for the given listen address.
func DefaultConfig(addr string) Config {
	return Config{
		Addr:              addr,
		ReadHeaderTimeout: DefaultReadHeaderTimeout,
		ReadTimeout:       DefaultReadTimeout,
		WriteTimeout:      DefaultWriteTimeout,
		IdleTimeout:       DefaultIdleTimeout,
		ShutdownTimeout:   shutdown.DefaultTimeout,
		ReadinessTimeout:  health.DefaultTimeout,
	}
}

// Handler returns the root handler with the default readiness timeout.
func Handler(db *sql.DB) http.Handler {
	return HandlerWithReadinessTimeout(db, health.DefaultTimeout)
}

// HandlerWithReadinessTimeout returns the Phase 1 root handler. Only the exact
// canonical path GET /health is registered; /health/ is deliberately not
// registered and therefore returns a deterministic 404 from ServeMux.
func HandlerWithReadinessTimeout(db *sql.DB, timeout time.Duration) http.Handler {
	return handlerWithChecker(db, timeout)
}

// handlerWithChecker is the test seam that proves a configured timeout reaches
// the readiness handler without widening the production API.
func handlerWithChecker(checker health.Checker, timeout time.Duration) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET "+health.Path, health.NewHandlerWithTimeout(checker, timeout))
	return mux
}

// New builds an HTTP server with bounded request and readiness timeouts.
func New(cfg Config, db *sql.DB) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           HandlerWithReadinessTimeout(db, durationOr(cfg.ReadinessTimeout, health.DefaultTimeout)),
		ReadHeaderTimeout: durationOr(cfg.ReadHeaderTimeout, DefaultReadHeaderTimeout),
		ReadTimeout:       durationOr(cfg.ReadTimeout, DefaultReadTimeout),
		WriteTimeout:      durationOr(cfg.WriteTimeout, DefaultWriteTimeout),
		IdleTimeout:       durationOr(cfg.IdleTimeout, DefaultIdleTimeout),
	}
}

// Run binds cfg.Addr and serves until ctx is cancelled or serving fails. It
// owns db once called: every error path attempts to close it, and successful
// shutdown drains HTTP requests before closing the database.
func Run(ctx context.Context, cfg Config, db *sql.DB, logger *slog.Logger) error {
	if ctx == nil {
		return errors.Join(errors.New("server: nil context"), closeDatabase(db))
	}
	if cfg.Addr == "" {
		return errors.Join(errors.New("server: empty listen address"), closeDatabase(db))
	}
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return errors.Join(fmt.Errorf("listen on http address: %w", err), closeDatabase(db))
	}
	return Serve(ctx, cfg, ln, db, logger)
}

// Serve runs an already-bound listener with the same lifecycle as Run. Tests
// use it with an ephemeral listener so startup and shutdown are deterministic.
func Serve(ctx context.Context, cfg Config, ln net.Listener, db *sql.DB, logger *slog.Logger) error {
	if ctx == nil {
		return errors.Join(errors.New("server: nil context"), closeDatabase(db))
	}
	if ln == nil {
		return errors.Join(errors.New("server: nil listener"), closeDatabase(db))
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	// The fallback close covers cancellation that arrives before http.Server
	// gets a chance to register the listener with Shutdown.
	defer func() { _ = ln.Close() }()

	srv := New(cfg, db)
	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	serveErrCh := make(chan error, 1)
	go func() {
		err := srv.Serve(ln)
		serveErrCh <- err
		cancel()
	}()

	logger.Info("http server started", "addr", ln.Addr().String())
	cleanupErr := cleanupWith(serveCtx, cfg.ShutdownTimeout,
		func(cleanupCtx context.Context) error {
			logger.Info("http server shutting down")
			return srv.Shutdown(cleanupCtx)
		},
		func() error { return closeDatabase(db) },
	)

	serveErr := <-serveErrCh
	switch {
	case serveErr == nil:
	case errors.Is(serveErr, http.ErrServerClosed):
		serveErr = nil
	default:
		serveErr = fmt.Errorf("serve http: %w", serveErr)
	}

	result := errors.Join(serveErr, cleanupErr)
	if result == nil {
		logger.Info("shutdown complete")
	}
	return result
}

// cleanupWith drains the HTTP server before closing the database. It attempts
// the close even when shutdown fails and joins both errors for the caller.
func cleanupWith(ctx context.Context, timeout time.Duration, shutdownFn func(context.Context) error, closeDB func() error) error {
	if ctx == nil {
		return errors.New("server: nil context")
	}
	return shutdown.Wait(ctx, timeout, func(cleanupCtx context.Context) error {
		shutdownErr := shutdownFn(cleanupCtx)
		if shutdownErr != nil {
			shutdownErr = fmt.Errorf("shut down http server: %w", shutdownErr)
		}
		closeErr := closeDB()
		return errors.Join(shutdownErr, closeErr)
	})
}

func closeDatabase(db *sql.DB) error {
	if db == nil {
		return nil
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	return nil
}

func durationOr(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}
