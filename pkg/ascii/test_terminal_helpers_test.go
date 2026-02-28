package ascii

import "testing"

func forceTestTerminal(t *testing.T, termProgram string) {
	t.Helper()
	// Register reload cleanup before Setenv so it runs after env restoration.
	t.Cleanup(func() {
		ReloadTerminalDetection()
	})
	t.Setenv("TERM_PROGRAM", termProgram)
	ReloadTerminalDetection()
}
