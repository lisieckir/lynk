package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/lisieckir/lynk/internal/config"
)

func jsonRecords(t *testing.T, output string) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("log line is not JSON (%q): %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

func TestProductionRedactsSensitiveAttributes(t *testing.T) {
	var out bytes.Buffer
	logger := New(slog.LevelDebug, config.EnvProduction, &out)
	logger.Info("request failed",
		"authorization", "Bearer secret-auth-value",
		"token", "raw-token-value",
		"api_key", "key-value",
		"password", "hunter2",
		"request_id", "req-123",
		slog.Group("safe", slog.String("id", "id-1"), slog.String("Authorization", "Bearer nested-secret")),
		slog.Group("auth", slog.String("nested", "nested-secret")),
		"err", errors.New("failed with token=error-token and Bearer error-bearer"),
	)

	records := jsonRecords(t, out.String())
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1: %s", len(records), out.String())
	}
	record := records[0]
	for _, key := range []string{"authorization", "token", "api_key", "password"} {
		if record[key] != Redacted {
			t.Errorf("%s = %v, want redacted", key, record[key])
		}
	}
	sensitiveGroup, ok := record["auth"].(map[string]any)
	if !ok || sensitiveGroup["nested"] != Redacted {
		t.Errorf("sensitive group was not redacted: %v", record["auth"])
	}
	if errValue, ok := record["err"].(string); !ok || !strings.Contains(errValue, Redacted) {
		t.Errorf("error value was not scrubbed: %v", record["err"])
	}
	if record["request_id"] != "req-123" {
		t.Errorf("non-sensitive attribute was changed: %v", record["request_id"])
	}
	group, ok := record["safe"].(map[string]any)
	if !ok {
		t.Fatalf("safe group missing: %v", record["safe"])
	}
	if group["id"] != "id-1" || group["Authorization"] != Redacted {
		t.Errorf("safe group was not selectively redacted: %v", group)
	}

	raw := out.String()
	for _, secret := range []string{"secret-auth-value", "raw-token-value", "key-value", "hunter2", "nested-secret", "error-token", "error-bearer"} {
		if strings.Contains(raw, secret) {
			t.Errorf("secret %q leaked into log output:\n%s", secret, raw)
		}
	}
}

func TestRedactsSecretsInsideMessages(t *testing.T) {
	hexSecret := strings.Repeat("0123456789abcdef", 3)
	message := "call failed: Authorization: Bearer msg-secret token=msg-token " + hexSecret

	var out bytes.Buffer
	New(slog.LevelInfo, config.EnvProduction, &out).Info(message)

	raw := out.String()
	for _, secret := range []string{"msg-secret", "msg-token", hexSecret} {
		if strings.Contains(raw, secret) {
			t.Errorf("secret %q leaked into log message:\n%s", secret, raw)
		}
	}
	if !strings.Contains(raw, Redacted) {
		t.Errorf("no redaction marker in output:\n%s", raw)
	}
}

func TestRedact(t *testing.T) {
	tests := []struct {
		name  string
		input string
		bad   string
		good  string
	}{
		{"authorization header", "Authorization: Bearer abc123", "abc123", "Authorization: "},
		{"query token", "GET /?token=abc123&next=1", "token=abc123", "next=1"},
		{"quoted api key", "api_key: \"abc def\"", "abc def", "api_key"},
		{"bare bearer", "retry with Bearer abc123", "abc123", "Bearer"},
		{"hex token", "token " + strings.Repeat("a1", 20), strings.Repeat("a1", 20), Redacted},
		{"ordinary text", "request req-123 completed in 4ms", "", "request req-123 completed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Redact(tt.input)
			if tt.bad != "" && strings.Contains(got, tt.bad) {
				t.Errorf("Redact(%q) = %q, still contains %q", tt.input, got, tt.bad)
			}
			if !strings.Contains(got, tt.good) {
				t.Errorf("Redact(%q) = %q, missing %q", tt.input, got, tt.good)
			}
			if again := Redact(got); again != got {
				t.Errorf("redaction is not idempotent: %q -> %q", got, again)
			}
		})
	}
}

func TestHeadersAttributeRedactsAuthorization(t *testing.T) {
	header := http.Header{}
	header.Set("Authorization", "Bearer header-secret")
	header.Set("X-Request-Id", "req-123")

	var out bytes.Buffer
	New(slog.LevelInfo, config.EnvProduction, &out).Info("incoming", Headers(header))

	record := jsonRecords(t, out.String())[0]
	headers, ok := record["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers attribute missing: %v", record)
	}
	values, ok := headers["Authorization"].([]any)
	if !ok || len(values) != 1 || values[0] != Redacted {
		t.Errorf("Authorization header = %v, want [%s]", headers["Authorization"], Redacted)
	}
	if !strings.Contains(out.String(), "req-123") {
		t.Errorf("non-sensitive header lost:\n%s", out.String())
	}
	if strings.Contains(out.String(), "header-secret") {
		t.Errorf("header secret leaked:\n%s", out.String())
	}
}

func TestWithAttrsAndGroupAreRedacted(t *testing.T) {
	var out bytes.Buffer
	logger := New(slog.LevelInfo, config.EnvProduction, &out).
		With("token", "with-token").
		WithGroup("safe").
		With("authorization", "Bearer with-auth")
	logger.Info("event")

	record := jsonRecords(t, out.String())[0]
	if record["token"] != Redacted {
		t.Errorf("With token = %v", record["token"])
	}
	group, ok := record["safe"].(map[string]any)
	if !ok || group["authorization"] != Redacted {
		t.Errorf("WithGroup attribute = %v", record["safe"])
	}
	raw := out.String()
	for _, secret := range []string{"with-token", "with-auth"} {
		if strings.Contains(raw, secret) {
			t.Errorf("secret %q leaked via With:\n%s", secret, raw)
		}
	}
}

func TestDevelopmentFormatAndLevelFiltering(t *testing.T) {
	var out bytes.Buffer
	logger := New(slog.LevelInfo, config.EnvDevelopment, &out)
	logger.Debug("hidden-debug-message")
	logger.Info("visible", "request_id", "req-1")

	got := out.String()
	if strings.Contains(got, "hidden-debug-message") {
		t.Errorf("debug record was emitted at info level:\n%s", got)
	}
	for _, want := range []string{"level=INFO", "msg=visible", "request_id=req-1"} {
		if !strings.Contains(got, want) {
			t.Errorf("text output missing %q:\n%s", want, got)
		}
	}
	if strings.HasPrefix(strings.TrimSpace(got), "{") {
		t.Errorf("development output is JSON:\n%s", got)
	}
}

func TestNewWithNilWriterDoesNotPanic(t *testing.T) {
	logger := New(slog.LevelInfo, config.EnvProduction, nil)
	logger.Info("discarded")
}
