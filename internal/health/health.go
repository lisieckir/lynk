// Package health implements Lynk's unauthenticated readiness endpoint.
//
// The handler deliberately serializes only fixed status bodies. Database
// errors, configuration values and filesystem paths are never returned to the
// caller.
package health

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"time"
)

const (
	// Path is the canonical readiness route.
	Path = "/health"

	// DefaultTimeout bounds one readiness check so an unavailable database
	// cannot hold an HTTP request open indefinitely.
	DefaultTimeout = 2 * time.Second

	okBody               = "{\"status\":\"ok\"}\n"
	unavailableBody      = "{\"status\":\"unavailable\"}\n"
	methodNotAllowedBody = "{\"status\":\"method_not_allowed\"}\n"
)

// Checker is the minimal readiness contract fulfilled by *sql.DB. Keeping the
// interface small lets tests model an unavailable database without a driver.
type Checker interface {
	PingContext(context.Context) error
}

// NewHandler returns a handler that reports 200 when checker is ready and 503
// otherwise. It uses DefaultTimeout for the readiness check.
func NewHandler(checker Checker) http.Handler {
	return NewHandlerWithTimeout(checker, DefaultTimeout)
}

// NewHandlerWithTimeout is NewHandler with an explicit check timeout. A
// non-positive timeout falls back to DefaultTimeout.
func NewHandlerWithTimeout(checker Checker, timeout time.Duration) http.Handler {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	// A typed nil (*sql.DB)(nil) satisfies Checker but would panic on Ping.
	// Normalize it to a nil interface so readiness reports 503 instead.
	if isNilChecker(checker) {
		checker = nil
	}
	return &handler{checker: checker, timeout: timeout}
}

func isNilChecker(checker Checker) bool {
	if checker == nil {
		return true
	}
	value := reflect.ValueOf(checker)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

type handler struct {
	checker Checker
	timeout time.Duration
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The server registers this handler with a GET pattern, which also admits
	// HEAD. Explicit guarding keeps direct handler use safe without breaking
	// ordinary healthcheck clients.
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeHeaders(w)
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		writeBody(w, r.Method, methodNotAllowedBody)
		return
	}

	ctx := r.Context()
	if h.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.timeout)
		defer cancel()
	}

	status := http.StatusServiceUnavailable
	body := unavailableBody
	if h.checker != nil && h.checker.PingContext(ctx) == nil {
		status = http.StatusOK
		body = okBody
	}

	writeHeaders(w)
	w.WriteHeader(status)
	writeBody(w, r.Method, body)
}

func writeHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func writeBody(w http.ResponseWriter, method, body string) {
	if method == http.MethodHead {
		return
	}
	_, _ = io.WriteString(w, body)
}
