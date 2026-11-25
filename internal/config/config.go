// Package config loads and validates Lynk's environment configuration.
//
// Configuration is resolved once at startup. Every value has a documented
// default from SPEC.md and invalid values fail with an error that names the
// environment variable. Paths are validated as filesystem paths only:
// creating directories, files and database schema belongs to later tasks.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Environment is the deployment mode selected by LYNK_ENV.
type Environment string

const (
	// EnvDevelopment uses human-readable logs.
	EnvDevelopment Environment = "development"
	// EnvProduction uses structured JSON logs.
	EnvProduction Environment = "production"
)

// String implements fmt.Stringer.
func (e Environment) String() string { return string(e) }

// IsProduction reports whether production-only rules should apply.
func (e Environment) IsProduction() bool { return e == EnvProduction }

// IsDevelopment reports whether development-only behavior should apply.
func (e Environment) IsDevelopment() bool { return e == EnvDevelopment }

// LookupFunc matches os.LookupEnv so tests can supply configuration without
// mutating the process environment.
type LookupFunc func(string) (string, bool)

// Config is the fully resolved, validated configuration. Construct it through
// Default or Load, then call Validate if values were changed programmatically.
type Config struct {
	Env          Environment
	HTTPAddr     string
	DataDir      string
	DatabasePath string
	LogLevel     slog.Level
	BaseURL      string
}

// Default returns the values documented in SPEC.md section 8.
func Default() Config {
	return Config{
		Env:          EnvProduction,
		HTTPAddr:     ":9090",
		DataDir:      "/data",
		DatabasePath: "/data/lynk.db",
		LogLevel:     slog.LevelInfo,
		BaseURL:      "https://lynk.example.com",
	}
}

// Load reads the process environment.
func Load() (Config, error) {
	return LoadFrom(os.LookupEnv)
}

// LoadFrom resolves every supported variable, reports all invalid values at
// once, and returns a zero Config on any error. An unset or blank variable
// selects its documented default.
func LoadFrom(lookup LookupFunc) (Config, error) {
	if lookup == nil {
		return Config{}, errors.New("configuration lookup function is nil")
	}

	cfg := Default()
	var errs []error
	if raw, ok := lookupValue(lookup, "LYNK_ENV"); ok {
		env, err := parseEnvironment(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("LYNK_ENV: %w", err))
		} else {
			cfg.Env = env
		}
	}
	if raw, ok := lookupValue(lookup, "LYNK_HTTP_ADDR"); ok {
		addr, err := parseHTTPAddr(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("LYNK_HTTP_ADDR: %w", err))
		} else {
			cfg.HTTPAddr = addr
		}
	}
	if raw, ok := lookupValue(lookup, "LYNK_DATA_DIR"); ok {
		dir, err := parseDirectory(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("LYNK_DATA_DIR: %w", err))
		} else {
			cfg.DataDir = dir
		}
	}
	if raw, ok := lookupValue(lookup, "LYNK_DATABASE_PATH"); ok {
		path, err := parseDatabasePath(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("LYNK_DATABASE_PATH: %w", err))
		} else {
			cfg.DatabasePath = path
		}
	}
	if raw, ok := lookupValue(lookup, "LYNK_LOG_LEVEL"); ok {
		level, err := parseLogLevel(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("LYNK_LOG_LEVEL: %w", err))
		} else {
			cfg.LogLevel = level
		}
	}
	if raw, ok := lookupValue(lookup, "LYNK_BASE_URL"); ok {
		baseURL, err := parseBaseURL(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("LYNK_BASE_URL: %w", err))
		} else {
			cfg.BaseURL = baseURL
		}
	}

	// Validate is intentionally called even after successful parsing: it is
	// the single source of truth for invariants, including programmatic use.
	if err := cfg.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}

// Validate checks all fields and joins every violation into one readable
// error. It performs no filesystem access and creates nothing.
func (c Config) Validate() error {
	var errs []error
	if _, err := parseEnvironment(string(c.Env)); err != nil {
		errs = append(errs, fmt.Errorf("LYNK_ENV: %w", err))
	}
	if _, err := parseHTTPAddr(c.HTTPAddr); err != nil {
		errs = append(errs, fmt.Errorf("LYNK_HTTP_ADDR: %w", err))
	}
	if _, err := parseDirectory(c.DataDir); err != nil {
		errs = append(errs, fmt.Errorf("LYNK_DATA_DIR: %w", err))
	}
	if _, err := parseDatabasePath(c.DatabasePath); err != nil {
		errs = append(errs, fmt.Errorf("LYNK_DATABASE_PATH: %w", err))
	}
	if err := validateLevel(c.LogLevel); err != nil {
		errs = append(errs, fmt.Errorf("LYNK_LOG_LEVEL: %w", err))
	}
	if _, err := parseBaseURL(c.BaseURL); err != nil {
		errs = append(errs, fmt.Errorf("LYNK_BASE_URL: %w", err))
	}
	return errors.Join(errs...)
}

func lookupValue(lookup LookupFunc, name string) (string, bool) {
	raw, ok := lookup(name)
	if !ok {
		return "", false
	}
	if strings.TrimSpace(raw) == "" {
		return "", false
	}
	return raw, true
}

func parseEnvironment(raw string) (Environment, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(EnvDevelopment):
		return EnvDevelopment, nil
	case string(EnvProduction):
		return EnvProduction, nil
	default:
		return "", fmt.Errorf("unknown environment %q (want %q or %q)", raw, EnvDevelopment, EnvProduction)
	}
}

// parseHTTPAddr accepts host:port forms, including the wildcard ":9090" and
// bracketed IPv6. Hostname resolution is deliberately deferred to net/http.
func parseHTTPAddr(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("must not be empty")
	}
	if strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, " \t\r\n/\\?#@") {
		return "", fmt.Errorf("invalid listen address %q", raw)
	}
	host, port, err := net.SplitHostPort(raw)
	if err != nil {
		return "", fmt.Errorf("invalid listen address %q (want host:port): %w", raw, err)
	}
	if host != "" && !validHost(host) {
		return "", fmt.Errorf("invalid listen host %q", host)
	}
	number, err := strconv.Atoi(port)
	if err != nil {
		return "", fmt.Errorf("invalid listen port %q", port)
	}
	if number < 1 || number > 65535 {
		return "", fmt.Errorf("listen port %d is outside 1-65535", number)
	}
	return raw, nil
}

func validHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return net.ParseIP(host[1:len(host)-1]) != nil
	}
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") || strings.Contains(host, "..") {
		return false
	}
	for _, r := range host {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '.', r == '_':
		default:
			return false
		}
	}
	return true
}

func parseDirectory(raw string) (string, error) {
	return parseAbsPath(raw, false)
}

func parseDatabasePath(raw string) (string, error) {
	if strings.HasSuffix(raw, string(filepath.Separator)) {
		return "", fmt.Errorf("path %q must name a file, not a directory", raw)
	}
	return parseAbsPath(raw, true)
}

// parseAbsPath mirrors the restrictions in internal/database: absolute plain
// paths only, no SQLite DSN/URI syntax and no control characters. This prevents
// a configuration value from being interpreted as a driver connection string.
func parseAbsPath(raw string, rejectMemory bool) (string, error) {
	if raw == "" {
		return "", errors.New("path is empty")
	}
	if strings.TrimSpace(raw) != raw {
		return "", fmt.Errorf("path %q has leading or trailing whitespace", raw)
	}
	for _, r := range raw {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("path %q contains a control character", raw)
		}
	}
	if strings.HasPrefix(strings.ToLower(raw), "file:") || strings.ContainsAny(raw, "?#") || (rejectMemory && strings.EqualFold(raw, ":memory:")) {
		return "", fmt.Errorf("path %q uses DSN/URI syntax; provide a filesystem path", raw)
	}
	if !filepath.IsAbs(raw) {
		return "", fmt.Errorf("path %q must be absolute", raw)
	}
	cleaned := filepath.Clean(raw)
	if cleaned == string(filepath.Separator) {
		return "", errors.New("path is a filesystem root, not a usable location")
	}
	return cleaned, nil
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown level %q (want debug, info, warn or error)", raw)
	}
}

func validateLevel(level slog.Level) error {
	switch level {
	case slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError:
		return nil
	default:
		return fmt.Errorf("unsupported slog level %d; use debug, info, warn or error", level)
	}
}

// parseBaseURL accepts only an absolute HTTP(S) URL without credentials,
// query or fragment. A trailing slash is removed for consistent joining.
func parseBaseURL(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("URL is empty")
	}
	if strings.ContainsAny(raw, " \t\r\n") {
		return "", fmt.Errorf("URL %q contains whitespace", raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("URL %q must use http or https", raw)
	}
	if u.Opaque != "" || u.Host == "" || u.Hostname() == "" {
		return "", fmt.Errorf("URL %q must have an absolute host", raw)
	}
	if u.User != nil {
		return "", errors.New("URL must not contain credentials")
	}
	if u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", errors.New("URL must not contain a query or fragment")
	}
	if strings.HasSuffix(u.Path, "/") {
		u.Path = strings.TrimRight(u.Path, "/")
		u.RawPath = ""
	}
	return u.String(), nil
}
