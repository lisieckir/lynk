// Command lynk is the application entrypoint.
//
// Phase 1 foundation: typed configuration, structured logging, SQLite
// migrations, the unauthenticated /health endpoint and graceful signal
// handling are wired here. API, UI and container packaging are later tasks.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/lisieckir/lynk/internal/config"
	"github.com/lisieckir/lynk/internal/database"
	"github.com/lisieckir/lynk/internal/logging"
	"github.com/lisieckir/lynk/internal/server"
	"github.com/lisieckir/lynk/internal/shutdown"
)

func main() {
	ctx, stop := shutdown.Context(context.Background())
	defer stop()
	if err := run(ctx, os.LookupEnv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("lynk: %w", err))
		os.Exit(1)
	}
}

// run is separated from main so configuration, startup and shutdown behavior
// can be tested without installing process signal handlers.
func run(ctx context.Context, lookup config.LookupFunc, out io.Writer) error {
	cfg, err := config.LoadFrom(lookup)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	logger := logging.New(cfg.LogLevel, cfg.Env, out)
	// Keep the startup record useful without emitting filesystem paths.
	logger.Info("configuration loaded",
		"env", cfg.Env.String(),
		"http_addr", cfg.HTTPAddr,
		"base_url", cfg.BaseURL,
		"log_level", cfg.LogLevel.String(),
	)

	// An already-cancelled context means shutdown was requested before startup
	// could finish. Avoid touching the database or binding a port; preserve the
	// original entrypoint contract of a clean, logged shutdown.
	if ctx.Err() != nil {
		return shutdown.Wait(ctx, shutdown.DefaultTimeout, func(context.Context) error {
			logger.Info("shutdown complete")
			return nil
		})
	}

	db, err := database.Open(ctx, database.Config{Path: cfg.DatabasePath})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	logger.Info("database ready")

	serverCfg := server.DefaultConfig(cfg.HTTPAddr)
	serverCfg.ReadinessTimeout = cfg.HealthTimeout
	return server.Run(ctx, serverCfg, db, logger)
}
