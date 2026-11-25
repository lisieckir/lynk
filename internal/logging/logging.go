// Package logging configures log/slog for Lynk and redacts secrets before
// records reach a handler.
//
// All application logs go through a logger created here. A handler wrapper and
// the ReplaceAttr hook remove sensitive fields, scrub credentials embedded in
// messages/error strings, and the Headers helper sanitizes raw HTTP headers.
package logging

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/lisieckir/lynk/internal/config"
)

// New returns a logger at the configured level. Production uses JSON records;
// development uses slog's human-readable text format. Sources are not added so
// log records never expose filesystem paths.
func New(level slog.Level, env config.Environment, w io.Writer) *slog.Logger {
	if w == nil {
		w = io.Discard
	}
	options := &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: ReplaceAttr,
	}
	var handler slog.Handler
	if env.IsProduction() {
		handler = slog.NewJSONHandler(w, options)
	} else {
		handler = slog.NewTextHandler(w, options)
	}
	return slog.New(redactingHandler{Handler: handler})
}

// redactingHandler sanitizes groups before delegating. slog's ReplaceAttr hook
// intentionally skips group attributes, so without this wrapper a group named
// "auth" could carry non-sensitive-looking children containing secrets.
type redactingHandler struct {
	slog.Handler
}

func (h redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

func (h redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	sanitized := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(a slog.Attr) bool {
		sanitized.AddAttrs(sanitizeAttr(a))
		return true
	})
	return h.Handler.Handle(ctx, sanitized)
}

func (h redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return redactingHandler{Handler: h.Handler.WithAttrs(sanitizeAttrs(attrs))}
}

func (h redactingHandler) WithGroup(name string) slog.Handler {
	return redactingHandler{Handler: h.Handler.WithGroup(name)}
}

func sanitizeAttrs(attrs []slog.Attr) []slog.Attr {
	safe := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		safe[i] = sanitizeAttr(attr)
	}
	return safe
}

func sanitizeAttr(attr slog.Attr) slog.Attr {
	if attr.Value.Kind() != slog.KindGroup {
		return ReplaceAttr(nil, attr)
	}
	children := attr.Value.Group()
	safe := make([]any, len(children))
	for i, child := range children {
		if sensitiveKey(attr.Key) {
			// The group itself is sensitive: do not inspect child keys.
			safe[i] = slog.String(child.Key, Redacted)
			continue
		}
		safe[i] = sanitizeAttr(child)
	}
	return slog.Group(attr.Key, safe...)
}

// Headers returns a log attribute containing a sanitized copy of h. Use it
// instead of logging http.Header directly: slog treats a header map as one
// opaque attribute, so its individual keys would not reach ReplaceAttr.
func Headers(h http.Header) slog.Attr {
	safe := make(http.Header, len(h))
	for name, values := range h {
		if sensitiveKey(name) {
			safe[name] = []string{Redacted}
			continue
		}
		scrubbed := make([]string, len(values))
		for i, value := range values {
			scrubbed[i] = Redact(value)
		}
		safe[name] = scrubbed
	}
	return slog.Any("headers", safe)
}
