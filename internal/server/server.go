// Package server wires Lynk's minimal net/http server and its lifecycle.
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

	"github.com/lisieckir/lynk/internal/api/linkding"
	"github.com/lisieckir/lynk/internal/health"
	"github.com/lisieckir/lynk/internal/metadata"
	"github.com/lisieckir/lynk/internal/shutdown"
	"github.com/lisieckir/lynk/internal/worker"
)

const (
	DefaultReadHeaderTimeout = 5 * time.Second
	DefaultReadTimeout       = 10 * time.Second
	DefaultWriteTimeout      = 10 * time.Second
	DefaultIdleTimeout       = 60 * time.Second
)

type Config struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	ReadinessTimeout  time.Duration
}

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

func Handler(db *sql.DB) http.Handler {
	return HandlerWithReadinessTimeout(db, health.DefaultTimeout)
}

func HandlerWithReadinessTimeout(db *sql.DB, timeout time.Duration) http.Handler {
	return HandlerWithAPI(db, linkding.NewHandler(db, slog.Default()), timeout)
}

func HandlerWithAPI(db *sql.DB, api http.Handler, timeout time.Duration) http.Handler {
	mux := handlerWithChecker(db, timeout)
	if api != nil {
		mux.Handle("/api/", api)
		mux.Handle("/api", api)
	}
	return mux
}

func handlerWithChecker(checker health.Checker, timeout time.Duration) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET "+health.Path, health.NewHandlerWithTimeout(checker, timeout))
	return mux
}

func New(cfg Config, db *sql.DB) *http.Server {
	return NewWithLogger(cfg, db, slog.Default())
}

func NewWithLogger(cfg Config, db *sql.DB, logger *slog.Logger) *http.Server {
	if logger == nil {
		logger = slog.Default()
	}
	// Avoid starting a background worker in this helper to prevent lifecycle leaks.
	// Use Serve/Run for a fully managed lifecycle (worker started and stopped).
	api := linkding.NewHandler(db, logger)
	handler := HandlerWithAPI(db, api, durationOr(cfg.ReadinessTimeout, health.DefaultTimeout))
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: durationOr(cfg.ReadHeaderTimeout, DefaultReadHeaderTimeout),
		ReadTimeout:       durationOr(cfg.ReadTimeout, DefaultReadTimeout),
		WriteTimeout:      durationOr(cfg.WriteTimeout, DefaultWriteTimeout),
		IdleTimeout:       durationOr(cfg.IdleTimeout, DefaultIdleTimeout),
	}
	return srv
}

type metaEnqAdapter struct {
	w *worker.MetadataWorker
}

func (a *metaEnqAdapter) Enqueue(bookmarkID int64, url string) {
	a.w.Enqueue(worker.MetadataJob{BookmarkID: bookmarkID, URL: url})
}

// Run binds cfg.Addr and serves until ctx is cancelled or serving fails.
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

// Serve runs an already-bound listener with the same lifecycle as Run.
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

	defer func() { _ = ln.Close() }()

	// Setup metadata worker
	fetcher := metadata.NewFetcher(metadata.Config{})
	metaWorker := worker.NewMetadataWorker(db, fetcher, logger, 100)
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()
	metaWorker.Start(workerCtx, 2)

	api := linkding.NewHandlerWithMetadata(db, logger, &metaEnqAdapter{w: metaWorker})
	handler := HandlerWithAPI(db, api, durationOr(cfg.ReadinessTimeout, health.DefaultTimeout))
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: durationOr(cfg.ReadHeaderTimeout, DefaultReadHeaderTimeout),
		ReadTimeout:       durationOr(cfg.ReadTimeout, DefaultReadTimeout),
		WriteTimeout:      durationOr(cfg.WriteTimeout, DefaultWriteTimeout),
		IdleTimeout:       durationOr(cfg.IdleTimeout, DefaultIdleTimeout),
	}

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
			// Stop worker first
			metaWorker.Stop()
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
