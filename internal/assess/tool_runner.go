/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// runToolStdoutOnly executes a tool and captures only stdout (not stderr).
// This is useful for tools that output JSON to stdout but logs/warnings to stderr.
// Non-zero exit codes are treated as success (expected when issues are found).
func runToolStdoutOnly(target, bin string, args []string, timeout time.Duration) ([]byte, error) {
	tctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		tctx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(tctx, bin, args...) // #nosec G204 -- bin/args from controlled tool adapters
	cmd.Dir = target
	out, err := cmd.Output() // Only stdout, stderr is discarded
	if err != nil {
		// Non-zero exit is expected when issues are found
		if _, ok := err.(*exec.ExitError); ok {
			return out, nil
		}
		return nil, fmt.Errorf("%s execution failed: %w", bin, err)
	}
	return out, nil
}
