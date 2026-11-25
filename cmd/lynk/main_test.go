package main

import "testing"

// TestBootstrapEntrypoint is the P1-001 smoke test: the command package must
// compile as a test binary and the empty entrypoint must return cleanly.
// SQLite foundation tests live in internal/database.
func TestBootstrapEntrypoint(t *testing.T) {
	main()
}
