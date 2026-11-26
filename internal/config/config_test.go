package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/lisieckir/lynk/internal/cors"
)

func lookupFrom(values map[string]string) LookupFunc {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

func TestDefaultMatchesSpec(t *testing.T) {
	want := Config{
		Env:                  EnvProduction,
		HTTPAddr:             ":9090",
		DataDir:              "/data",
		DatabasePath:         "/data/lynk.db",
		LogLevel:             slog.LevelInfo,
		BaseURL:              "https://lynk.example.com",
		HealthTimeout:        2 * time.Second,
		CORS:                 cors.DefaultOptions(),
		LoginRateLimitMax:    10,
		LoginRateLimitWindow: time.Minute,
		WebMaxSessions:       10000,
		TrustProxyHeaders:    false,
	}
	if got := Default(); got != want {
		t.Fatalf("Default() = %+v, want %+v", got, want)
	}
	if err := Default().Validate(); err != nil {
		t.Fatalf("default configuration failed validation: %v", err)
	}
}

func TestLoadFromDefaults(t *testing.T) {
	tests := map[string]map[string]string{
		"unset": nil,
		"blank": {
			"LYNK_ENV": "", "LYNK_HTTP_ADDR": " ", "LYNK_DATA_DIR": "",
			"LYNK_DATABASE_PATH": "", "LYNK_LOG_LEVEL": "\t", "LYNK_BASE_URL": "",
			"LYNK_HEALTH_TIMEOUT": "", "LYNK_CORS_ALLOWED_ORIGINS": "",
			"LYNK_CORS_ALLOWED_METHODS": "", "LYNK_CORS_ALLOWED_HEADERS": "",
			"LYNK_CORS_MAX_AGE": "",
		},
	}
	for name, values := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := LoadFrom(lookupFrom(values))
			if err != nil {
				t.Fatalf("LoadFrom: %v", err)
			}
			if want := Default(); got != want {
				t.Fatalf("LoadFrom() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestLoadFromValidValues(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		check  func(*testing.T, Config)
	}{
		{"env development", map[string]string{"LYNK_ENV": "development"}, func(t *testing.T, c Config) {
			if c.Env != EnvDevelopment || !c.Env.IsDevelopment() || c.Env.IsProduction() {
				t.Errorf("Env = %q", c.Env)
			}
		}},
		{"env normalized", map[string]string{"LYNK_ENV": " PRODUCTION "}, func(t *testing.T, c Config) {
			if c.Env != EnvProduction || !c.Env.IsProduction() {
				t.Errorf("Env = %q", c.Env)
			}
		}},
		{"wildcard address", map[string]string{"LYNK_HTTP_ADDR": ":9090"}, func(t *testing.T, c Config) {
			if c.HTTPAddr != ":9090" {
				t.Errorf("HTTPAddr = %q", c.HTTPAddr)
			}
		}},
		{"ipv4 address", map[string]string{"LYNK_HTTP_ADDR": "127.0.0.1:8080"}, func(t *testing.T, c Config) {
			if c.HTTPAddr != "127.0.0.1:8080" {
				t.Errorf("HTTPAddr = %q", c.HTTPAddr)
			}
		}},
		{"ipv6 address", map[string]string{"LYNK_HTTP_ADDR": "[::1]:9090"}, func(t *testing.T, c Config) {
			if c.HTTPAddr != "[::1]:9090" {
				t.Errorf("HTTPAddr = %q", c.HTTPAddr)
			}
		}},
		{"hostname address", map[string]string{"LYNK_HTTP_ADDR": "lynk.local:443"}, func(t *testing.T, c Config) {
			if c.HTTPAddr != "lynk.local:443" {
				t.Errorf("HTTPAddr = %q", c.HTTPAddr)
			}
		}},
		{"cleaned data dir", map[string]string{"LYNK_DATA_DIR": "/var/lib/lynk/./"}, func(t *testing.T, c Config) {
			if c.DataDir != "/var/lib/lynk" {
				t.Errorf("DataDir = %q", c.DataDir)
			}
		}},
		{"cleaned database path", map[string]string{"LYNK_DATABASE_PATH": "/var/lib/lynk/../lynk.db"}, func(t *testing.T, c Config) {
			if c.DatabasePath != "/var/lib/lynk.db" {
				t.Errorf("DatabasePath = %q", c.DatabasePath)
			}
		}},
		{"debug level", map[string]string{"LYNK_LOG_LEVEL": "DEBUG"}, func(t *testing.T, c Config) {
			if c.LogLevel != slog.LevelDebug {
				t.Errorf("LogLevel = %v", c.LogLevel)
			}
		}},
		{"warn level", map[string]string{"LYNK_LOG_LEVEL": "Warn"}, func(t *testing.T, c Config) {
			if c.LogLevel != slog.LevelWarn {
				t.Errorf("LogLevel = %v", c.LogLevel)
			}
		}},
		{"warning alias", map[string]string{"LYNK_LOG_LEVEL": "warning"}, func(t *testing.T, c Config) {
			if c.LogLevel != slog.LevelWarn {
				t.Errorf("LogLevel = %v", c.LogLevel)
			}
		}},
		{"error level", map[string]string{"LYNK_LOG_LEVEL": "error"}, func(t *testing.T, c Config) {
			if c.LogLevel != slog.LevelError {
				t.Errorf("LogLevel = %v", c.LogLevel)
			}
		}},
		{"base url without slash", map[string]string{"LYNK_BASE_URL": "http://localhost:8080/"}, func(t *testing.T, c Config) {
			if c.BaseURL != "http://localhost:8080" {
				t.Errorf("BaseURL = %q", c.BaseURL)
			}
		}},
		{"base url subpath", map[string]string{"LYNK_BASE_URL": "https://host.example/lynk/"}, func(t *testing.T, c Config) {
			if c.BaseURL != "https://host.example/lynk" {
				t.Errorf("BaseURL = %q", c.BaseURL)
			}
		}},
		{"health timeout", map[string]string{"LYNK_HEALTH_TIMEOUT": "1500ms"}, func(t *testing.T, c Config) {
			if c.HealthTimeout != 1500*time.Millisecond {
				t.Errorf("HealthTimeout = %v", c.HealthTimeout)
			}
		}},
		{"cors allowed origins", map[string]string{"LYNK_CORS_ALLOWED_ORIGINS": "https://app.example.com, https://admin.example.com:8443"}, func(t *testing.T, c Config) {
			if c.CORS.AllowedOrigins != "https://app.example.com, https://admin.example.com:8443" {
				t.Errorf("CORS.AllowedOrigins = %q", c.CORS.AllowedOrigins)
			}
			if c.CORS.AllowedMethods != "GET, POST" || c.CORS.AllowedHeaders != "Content-Type" {
				t.Errorf("CORS safe defaults changed: %+v", c.CORS)
			}
			if c.CORS.MaxAgeSeconds != cors.DefaultMaxAgeSeconds {
				t.Errorf("CORS.MaxAgeSeconds = %d", c.CORS.MaxAgeSeconds)
			}
		}},
		{"cors explicit methods and headers", map[string]string{
			"LYNK_CORS_ALLOWED_METHODS": "GET, POST, DELETE",
			"LYNK_CORS_ALLOWED_HEADERS": "Content-Type, Authorization",
			"LYNK_CORS_MAX_AGE":         "300",
		}, func(t *testing.T, c Config) {
			if c.CORS.AllowedMethods != "GET, POST, DELETE" || c.CORS.AllowedHeaders != "Content-Type, Authorization" {
				t.Errorf("CORS policy = %+v", c.CORS)
			}
			if c.CORS.MaxAgeSeconds != 300 {
				t.Errorf("CORS.MaxAgeSeconds = %d", c.CORS.MaxAgeSeconds)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadFrom(lookupFrom(tt.values))
			if err != nil {
				t.Fatalf("LoadFrom: %v", err)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			tt.check(t, got)
		})
	}
}

func TestLoadFromInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		values  map[string]string
		wantErr string
	}{
		{"unknown env", map[string]string{"LYNK_ENV": "staging"}, "LYNK_ENV"},
		{"env alias", map[string]string{"LYNK_ENV": "prod"}, "LYNK_ENV"},
		{"address without port", map[string]string{"LYNK_HTTP_ADDR": "localhost"}, "LYNK_HTTP_ADDR"},
		{"address with nonnumeric port", map[string]string{"LYNK_HTTP_ADDR": "localhost:http"}, "LYNK_HTTP_ADDR"},
		{"address with port zero", map[string]string{"LYNK_HTTP_ADDR": ":0"}, "LYNK_HTTP_ADDR"},
		{"address with huge port", map[string]string{"LYNK_HTTP_ADDR": ":65536"}, "LYNK_HTTP_ADDR"},
		{"address with extra colon", map[string]string{"LYNK_HTTP_ADDR": "localhost:80:90"}, "LYNK_HTTP_ADDR"},
		{"address with scheme", map[string]string{"LYNK_HTTP_ADDR": "http://localhost:9090"}, "LYNK_HTTP_ADDR"},
		{"address with whitespace", map[string]string{"LYNK_HTTP_ADDR": " localhost:9090"}, "LYNK_HTTP_ADDR"},
		{"relative data dir", map[string]string{"LYNK_DATA_DIR": "data/lynk"}, "LYNK_DATA_DIR"},
		{"data dir as dsn", map[string]string{"LYNK_DATA_DIR": "file:/data"}, "LYNK_DATA_DIR"},
		{"data dir with query", map[string]string{"LYNK_DATA_DIR": "/data?mode=rw"}, "LYNK_DATA_DIR"},
		{"data dir with control char", map[string]string{"LYNK_DATA_DIR": "/data/new\nline"}, "LYNK_DATA_DIR"},
		{"data dir at root", map[string]string{"LYNK_DATA_DIR": "/"}, "LYNK_DATA_DIR"},
		{"relative database path", map[string]string{"LYNK_DATABASE_PATH": "lynk.db"}, "LYNK_DATABASE_PATH"},
		{"memory database", map[string]string{"LYNK_DATABASE_PATH": ":memory:"}, "LYNK_DATABASE_PATH"},
		{"database dsn", map[string]string{"LYNK_DATABASE_PATH": "file:/data/lynk.db?mode=rw"}, "LYNK_DATABASE_PATH"},
		{"database path directory", map[string]string{"LYNK_DATABASE_PATH": "/data/"}, "LYNK_DATABASE_PATH"},
		{"unknown level", map[string]string{"LYNK_LOG_LEVEL": "trace"}, "LYNK_LOG_LEVEL"},
		{"numeric level", map[string]string{"LYNK_LOG_LEVEL": "1"}, "LYNK_LOG_LEVEL"},
		{"non-http base url", map[string]string{"LYNK_BASE_URL": "ftp://host.example"}, "LYNK_BASE_URL"},
		{"relative base url", map[string]string{"LYNK_BASE_URL": "/lynk"}, "LYNK_BASE_URL"},
		{"base url without host", map[string]string{"LYNK_BASE_URL": "https://"}, "LYNK_BASE_URL"},
		{"base url with credentials", map[string]string{"LYNK_BASE_URL": "https://user:pass@host.example"}, "LYNK_BASE_URL"},
		{"base url with query", map[string]string{"LYNK_BASE_URL": "https://host.example?token=abc"}, "LYNK_BASE_URL"},
		{"base url with fragment", map[string]string{"LYNK_BASE_URL": "https://host.example/#part"}, "LYNK_BASE_URL"},
		{"base url with space", map[string]string{"LYNK_BASE_URL": "https://host ex/ample"}, "LYNK_BASE_URL"},
		{"health timeout malformed", map[string]string{"LYNK_HEALTH_TIMEOUT": "soon"}, "LYNK_HEALTH_TIMEOUT"},
		{"health timeout zero", map[string]string{"LYNK_HEALTH_TIMEOUT": "0s"}, "LYNK_HEALTH_TIMEOUT"},
		{"health timeout negative", map[string]string{"LYNK_HEALTH_TIMEOUT": "-1s"}, "LYNK_HEALTH_TIMEOUT"},
		{"cors wildcard origin", map[string]string{"LYNK_CORS_ALLOWED_ORIGINS": "*"}, "LYNK_CORS_ALLOWED_ORIGINS"},
		{"cors wildcard subdomain", map[string]string{"LYNK_CORS_ALLOWED_ORIGINS": "https://*.example.com"}, "LYNK_CORS_ALLOWED_ORIGINS"},
		{"cors origin with path", map[string]string{"LYNK_CORS_ALLOWED_ORIGINS": "https://example.com/lynk"}, "LYNK_CORS_ALLOWED_ORIGINS"},
		{"cors wildcard method", map[string]string{"LYNK_CORS_ALLOWED_METHODS": "*"}, "LYNK_CORS_ALLOWED_METHODS"},
		{"cors unknown method", map[string]string{"LYNK_CORS_ALLOWED_METHODS": "TRACE"}, "LYNK_CORS_ALLOWED_METHODS"},
		{"cors wildcard header", map[string]string{"LYNK_CORS_ALLOWED_HEADERS": "*"}, "LYNK_CORS_ALLOWED_HEADERS"},
		{"cors invalid header", map[string]string{"LYNK_CORS_ALLOWED_HEADERS": "X Bad Header"}, "LYNK_CORS_ALLOWED_HEADERS"},
		{"cors max age malformed", map[string]string{"LYNK_CORS_MAX_AGE": "soon"}, "LYNK_CORS_MAX_AGE"},
		{"cors max age negative", map[string]string{"LYNK_CORS_MAX_AGE": "-1"}, "LYNK_CORS_MAX_AGE"},
		{"cors max age too large", map[string]string{"LYNK_CORS_MAX_AGE": "90000"}, "LYNK_CORS_MAX_AGE"},
		{"multiple invalid values", map[string]string{"LYNK_ENV": "staging", "LYNK_HTTP_ADDR": "localhost"}, "LYNK_ENV"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadFrom(lookupFrom(tt.values))
			if err == nil {
				t.Fatalf("LoadFrom succeeded with %v", tt.values)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to mention %q", err, tt.wantErr)
			}
			if got != (Config{}) {
				t.Errorf("returned partially valid config on error: %+v", got)
			}
		})
	}
}

func TestLoadFromReportsEveryInvalidValue(t *testing.T) {
	_, err := LoadFrom(lookupFrom(map[string]string{
		"LYNK_ENV":            "staging",
		"LYNK_HTTP_ADDR":      "localhost",
		"LYNK_LOG_LEVEL":      "trace",
		"LYNK_HEALTH_TIMEOUT": "0s",
	}))
	if err == nil {
		t.Fatal("LoadFrom succeeded with four invalid values")
	}
	for _, name := range []string{"LYNK_ENV", "LYNK_HTTP_ADDR", "LYNK_LOG_LEVEL", "LYNK_HEALTH_TIMEOUT"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not mention %s", err, name)
		}
	}
}

func TestValidateRejectsProgrammaticChanges(t *testing.T) {
	cfg := Default()
	cfg.LogLevel = slog.Level(2)
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "LYNK_LOG_LEVEL") {
		t.Fatalf("Validate accepted custom slog level: %v", err)
	}

	cfg = Default()
	cfg.HTTPAddr = "localhost"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "LYNK_HTTP_ADDR") {
		t.Fatalf("Validate accepted invalid address: %v", err)
	}

	cfg = Default()
	cfg.HealthTimeout = 0
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "LYNK_HEALTH_TIMEOUT") {
		t.Fatalf("Validate accepted zero health timeout: %v", err)
	}
}

func TestValidateRejectsUnsafeProgrammaticCORSChanges(t *testing.T) {
	cfg := Default()
	cfg.CORS.AllowedOrigins = "*"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "LYNK_CORS_ALLOWED_ORIGINS") {
		t.Fatalf("Validate accepted wildcard origin: %v", err)
	}

	cfg = Default()
	cfg.CORS.AllowedHeaders = "Authorization, *"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "LYNK_CORS_ALLOWED_HEADERS") {
		t.Fatalf("Validate accepted wildcard header: %v", err)
	}
}

func TestLoadFromNilLookup(t *testing.T) {
	if _, err := LoadFrom(nil); err == nil {
		t.Fatal("LoadFrom(nil) succeeded")
	}
}
