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
	"github.com/lisieckir/lynk/internal/cors"
	"github.com/lisieckir/lynk/internal/health"
	"github.com/lisieckir/lynk/internal/metadata"
	"github.com/lisieckir/lynk/internal/shutdown"
	"github.com/lisieckir/lynk/internal/web"
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
	SecureCookies     bool
	// CORS applies only to /api routes. /health stays public and unchanged.
	CORS cors.Options
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
		SecureCookies:     false,
		CORS:              cors.DefaultOptions(),
	}
}

func Handler(db *sql.DB) http.Handler {
	return HandlerWithReadinessTimeout(db, health.DefaultTimeout)
}

func HandlerWithReadinessTimeout(db *sql.DB, timeout time.Duration) http.Handler {
	return HandlerWithAPI(db, linkding.NewHandler(db, slog.Default()), timeout)
}

func HandlerWithAPI(db *sql.DB, api http.Handler, timeout time.Duration) http.Handler {
	return HandlerWithAPIAndCORS(db, api, timeout, cors.Options{}, nil)
}

func handlerWithChecker(checker health.Checker, timeout time.Duration) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET "+health.Path, health.NewHandlerWithTimeout(checker, timeout))
	return mux
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-XSS-Protection", "0")
		next.ServeHTTP(w, r)
	})
}

func buildHandler(db *sql.DB, api http.Handler, timeout time.Duration, opts cors.Options, logger *slog.Logger, secureCookies bool) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	mux := handlerWithChecker(db, timeout)
	// Mount web UI (outside /api, outside /health)
	webHandler, err := web.NewHandler(db, logger, web.Config{SecureCookie: secureCookies})
	if err != nil {
		logger.Error("failed to create web handler", "error", err)
	} else {
		webHandler.Register(mux)
	}
	if api != nil {
		policy, err := cors.New(opts)
		if err != nil {
			logger.Warn("invalid CORS configuration; browser CORS disabled", "error", err)
		} else {
			api = policy.Handler(api)
		}
		mux.Handle("/api/", api)
		mux.Handle("/api", api)
	}
	return withSecurityHeaders(mux)
}

// HandlerWithAPIAndCORS mounts the API behind a CORS policy while leaving the
// public health route outside that wrapper. Invalid options fail closed: no
// browser origin receives CORS headers and the API keeps its auth contract.
func HandlerWithAPIAndCORS(db *sql.DB, api http.Handler, timeout time.Duration, opts cors.Options, logger *slog.Logger) http.Handler {
	return buildHandler(db, api, timeout, opts, logger, false)
}

func New(cfg Config, db *sql.DB) *http.Server {
	return NewWithLogger(cfg, db, slog.Default())
}

func NewWithLogger(cfg Config, db *sql.DB, logger *slog.Logger) *http.Server {
	if logger == nil {
		logger = slog.Default()
	}
	api := linkding.NewHandler(db, logger)
	handler := buildHandler(db, api, durationOr(cfg.ReadinessTimeout, health.DefaultTimeout), cfg.CORS, logger, cfg.SecureCookies)
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
	handler := buildHandler(db, api, durationOr(cfg.ReadinessTimeout, health.DefaultTimeout), cfg.CORS, logger, cfg.SecureCookies)
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
