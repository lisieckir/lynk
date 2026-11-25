package shutdown

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestContextCanceledByParent(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	ctx, stop := Context(parent)
	defer stop()

	cancelParent()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("derived context was not cancelled with its parent")
	}
}

func TestContextCanceledByInterrupt(t *testing.T) {
	ctx, stop := Context(context.Background())
	defer stop()

	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("find test process: %v", err)
	}
	if err := process.Signal(os.Interrupt); err != nil {
		t.Fatalf("send interrupt: %v", err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("context was not cancelled by SIGINT")
	}
}

func TestWaitRunsCleanupWithBoundedContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := 0
	err := Wait(ctx, 500*time.Millisecond, func(cleanupCtx context.Context) error {
		called++
		if cleanupCtx.Err() != nil {
			t.Errorf("cleanup context already cancelled: %v", cleanupCtx.Err())
		}
		deadline, ok := cleanupCtx.Deadline()
		if !ok {
			t.Fatal("cleanup context has no deadline")
		}
		if remaining := time.Until(deadline); remaining <= 0 || remaining > time.Second {
			t.Errorf("cleanup deadline is %v away", remaining)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if called != 1 {
		t.Errorf("cleanup called %d times, want 1", called)
	}
}

func TestWaitDoesNotCleanupBeforeCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- Wait(ctx, time.Second, func(context.Context) error { return nil })
	}()

	select {
	case err := <-done:
		t.Fatalf("Wait returned before cancellation: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Wait: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait did not return after cancellation")
	}
}

func TestWaitPropagatesCleanupError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	want := errors.New("cleanup failed")
	err := Wait(ctx, 0, func(context.Context) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("Wait error = %v, want wrapped %v", err, want)
	}
	if !strings.Contains(err.Error(), "shutdown cleanup") {
		t.Errorf("error lacks context: %v", err)
	}
}

func TestWaitWithoutCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Wait(ctx, time.Second, nil); err != nil {
		t.Fatalf("Wait with nil cleanup: %v", err)
	}
}

func TestWaitRejectsNilContext(t *testing.T) {
	if err := Wait(nil, time.Second, nil); err == nil {
		t.Fatal("Wait accepted a nil context")
	}
}
