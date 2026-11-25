package logging

import (
	"log/slog"
	"regexp"
	"strings"
)

// Redacted replaces every secret value emitted by this package.
const Redacted = "[REDACTED]"

// Sensitive attribute names are replaced wholesale, including whole groups
// such as slog.Group("auth", ...). Matching is case-insensitive and treats
// '-', '.' and ' ' like '_' so "api-key", "API_KEY" and "api key" all match.
func sensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(key))
	for _, part := range []string{
		"authorization", "token", "secret", "password", "passwd",
		"credential", "api_key", "apikey", "cookie", "session", "signature",
	} {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	return normalized == "auth"
}

var (
	// "Authorization: Bearer abc", "token=abc&next=1", "password: 'abc'".
	secretAssignment = regexp.MustCompile(`(?i)(authorization|bearer|token|api[_-]?key|secret|password|passwd|credential|session|cookie)(\s*[:=]\s*)(?:(?:bearer|token)\s+)?(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s,;&"']+)`)
	// A bearer value without an explicit key, e.g. "retry with Bearer abc".
	bearerToken = regexp.MustCompile(`(?i)(\bbearer\s+)[A-Za-z0-9._~+/\-]+=*`)
	// Linkding API tokens are 40 hex characters; scrub even when unlabelled.
	hexToken = regexp.MustCompile(`\b[0-9a-fA-F]{40,}\b`)
)

// Redact removes credentials embedded in free-form strings. It is safe to
// call on already-redacted text and is applied by ReplaceAttr to every string
// attribute and to the log message.
func Redact(s string) string {
	s = secretAssignment.ReplaceAllString(s, "${1}${2}"+Redacted)
	s = bearerToken.ReplaceAllString(s, "${1}"+Redacted)
	return hexToken.ReplaceAllString(s, Redacted)
}

// ReplaceAttr is the slog handler hook that enforces redaction. It is exported
// so callers adding their own slog handler can apply the same rules.
func ReplaceAttr(_ []string, a slog.Attr) slog.Attr {
	if sensitiveKey(a.Key) {
		return slog.String(a.Key, Redacted)
	}
	switch a.Value.Kind() {
	case slog.KindString:
		return slog.String(a.Key, Redact(a.Value.String()))
	case slog.KindAny:
		if err, ok := a.Value.Any().(error); ok {
			return slog.String(a.Key, Redact(err.Error()))
		}
	}
	return a
}
