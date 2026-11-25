package main

import "testing"

// TestBootstrapEntrypoint is the P1-001 smoke test: the command package must
// compile as a test binary and the empty entrypoint must return cleanly.
// Domain and integration tests are deferred to P1-003 and later tasks.
func TestBootstrapEntrypoint(t *testing.T) {
	main()
}
