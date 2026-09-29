package cli

import (
	"os"
	"testing"
)

// TestMain keeps the suite from ever reading the real terminal: run as a
// compiled binary from a shell, or under a debugger, stdin is a terminal and
// onboard would wait for answers. Tests that want questions stub it back.
func TestMain(m *testing.M) {
	stdinEhTerminal = func() bool { return false }
	os.Exit(m.Run())
}
