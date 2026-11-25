// Command lynk is the application entrypoint.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/lisieckir/lynk/internal/auth"
	"github.com/lisieckir/lynk/internal/config"
	"github.com/lisieckir/lynk/internal/database"
	"github.com/lisieckir/lynk/internal/logging"
	"github.com/lisieckir/lynk/internal/server"
	"github.com/lisieckir/lynk/internal/shutdown"
)

func main() {
	ctx, stop := shutdown.Context(context.Background())
	defer stop()
	if err := run(ctx, os.Args[1:], os.LookupEnv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("lynk: %w", err))
		os.Exit(1)
	}
}

// run is separated from main so configuration, startup and shutdown behavior
// can be tested without installing process signal handlers.
func run(ctx context.Context, args []string, lookup config.LookupFunc, out io.Writer) error {
	if len(args) > 0 && args[0] == "token" {
		return runTokenCmd(ctx, args[1:], lookup, out)
	}
	return runServer(ctx, lookup, out)
}

func runServer(ctx context.Context, lookup config.LookupFunc, out io.Writer) error {
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

func runTokenCmd(ctx context.Context, args []string, lookup config.LookupFunc, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: lynk token create [--username NAME] [--name TOKENNAME] | lynk token revoke --prefix PREFIX")
	}
	switch args[0] {
	case "create":
		return runTokenCreate(ctx, args[1:], lookup, out)
	case "revoke":
		return runTokenRevoke(ctx, args[1:], lookup, out)
	default:
		return fmt.Errorf("usage: lynk token create [--username NAME] [--name TOKENNAME] | lynk token revoke --prefix PREFIX")
	}
}

func runTokenCreate(ctx context.Context, args []string, lookup config.LookupFunc, out io.Writer) error {
	fs := flag.NewFlagSet("token create", flag.ContinueOnError)
	fs.SetOutput(out)
	username := fs.String("username", "admin", "Username for the first user (used only when no users exist)")
	tokenName := fs.String("name", "default", "Token name")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.LoadFrom(lookup)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	db, err := database.Open(ctx, database.Config{Path: cfg.DatabasePath})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	info, err := auth.CreateToken(ctx, db, *username, *tokenName)
	if err != nil {
		return fmt.Errorf("create token: %w", err)
	}
	// Print raw token once (never log it)
	fmt.Fprintf(out, "Token created for user_id=%d. Save this token now, it will not be shown again.\n", info.UserID)
	fmt.Fprintf(out, "TOKEN=%s\n", info.Token)
	return nil
}

func runTokenRevoke(ctx context.Context, args []string, lookup config.LookupFunc, out io.Writer) error {
	fs := flag.NewFlagSet("token revoke", flag.ContinueOnError)
	fs.SetOutput(out)
	prefix := fs.String("prefix", "", "Token prefix to revoke (non-revoked)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *prefix == "" {
		return fmt.Errorf("usage: lynk token revoke --prefix PREFIX")
	}

	cfg, err := config.LoadFrom(lookup)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	db, err := database.Open(ctx, database.Config{Path: cfg.DatabasePath})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := auth.RevokeByPrefix(ctx, db, *prefix); err != nil {
		if errors.Is(err, auth.ErrTokenNotFound) {
			return fmt.Errorf("revoke token: token not found")
		}
		if errors.Is(err, auth.ErrTokenPrefixAmbiguous) {
			return fmt.Errorf("revoke token: prefix is ambiguous")
		}
		return fmt.Errorf("revoke token: %w", err)
	}
	fmt.Fprintln(out, "Token revoked.")
	return nil
}
