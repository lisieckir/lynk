// Package shutdown provides context-aware signal handling and bounded cleanup
// for Lynk's single process. It contains no HTTP or domain behavior.
package shutdown

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// DefaultTimeout bounds cleanup when no positive timeout is supplied.
const DefaultTimeout = 10 * time.Second

// Context returns a context cancelled by SIGINT, SIGTERM or cancellation of
// parent. The returned stop function releases the signal subscription and
// should be deferred by the owner of the context.
func Context(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}

// Wait blocks until ctx is cancelled and then calls cleanup exactly once with
// a fresh, time-bounded context. Cleanup context values are inherited from ctx
// but its cancellation is not, so cleanup can finish after a shutdown signal.
// Cancellation itself is a normal exit; only cleanup errors are returned.
func Wait(ctx context.Context, timeout time.Duration, cleanup func(context.Context) error) error {
	if ctx == nil {
		return errors.New("shutdown: nil context")
	}
	<-ctx.Done()
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	if cleanup == nil {
		return nil
	}
	if err := cleanup(cleanupCtx); err != nil {
		return fmt.Errorf("shutdown cleanup: %w", err)
	}
	return nil
}
