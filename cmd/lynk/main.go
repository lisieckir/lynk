// Command lynk is the application entrypoint.
//
// Phase 1 foundation: typed configuration, structured logging and graceful
// signal handling are wired here. HTTP, health, API, UI and database startup
// are added by later Phase 1 tasks.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/lisieckir/lynk/internal/config"
	"github.com/lisieckir/lynk/internal/logging"
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

// run is separated from main so configuration and shutdown behavior can be
// tested without installing process signal handlers.
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

	return shutdown.Wait(ctx, shutdown.DefaultTimeout, func(context.Context) error {
		logger.Info("shutdown complete")
		return nil
	})
}
