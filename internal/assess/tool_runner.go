/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// toolRun is the captured result of an external tool invocation.
type toolRun struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// runToolSplit executes a tool and captures stdout and stderr separately.
// A non-zero exit is returned in ExitCode, not as an error: many tools exit
// non-zero when they report findings. The returned error is reserved for
// runs that did not complete (tool not executable, timeout, killed).
// Callers decide whether a non-zero exit means "findings" or "did not run".
func runToolSplit(target, bin string, args []string, timeout time.Duration) (toolRun, error) {
	return runToolSplitEnv(target, bin, args, timeout, nil)
}

// runToolSplitEnv is runToolSplit with extra environment entries appended
// to the inherited environment.
func runToolSplitEnv(target, bin string, args []string, timeout time.Duration, extraEnv []string) (toolRun, error) {
	tctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		tctx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(tctx, bin, args...) // #nosec G204 -- bin/args from controlled tool adapters
	cmd.Dir = target
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	run := toolRun{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if tctx.Err() != nil {
		return run, fmt.Errorf("%s timed out after %v", bin, timeout)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			run.ExitCode = exitErr.ExitCode()
			if run.ExitCode < 0 {
				return run, fmt.Errorf("%s terminated abnormally: %w", bin, err)
			}
			return run, nil
		}
		return run, fmt.Errorf("%s execution failed: %w", bin, err)
	}
	return run, nil
}

// stderrTail returns the last maxLines non-empty stderr lines for error messages.
func (r toolRun) stderrTail(maxLines int) string {
	lines := strings.Split(strings.TrimSpace(string(r.Stderr)), "\n")
	var kept []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			kept = append(kept, strings.TrimRight(line, " \t\r"))
		}
	}
	if len(kept) > maxLines {
		kept = kept[len(kept)-maxLines:]
	}
	return strings.Join(kept, "\n")
}

// failedWithoutOutput reports a non-zero exit that produced no stdout. For
// JSON-reporting tools this means the tool did not run to completion (bad
// config, missing lockfile, crash), not that it found issues.
func (r toolRun) failedWithoutOutput() bool {
	return r.ExitCode != 0 && len(bytes.TrimSpace(r.Stdout)) == 0
}

// runFailure formats a did-not-run error with the stderr tail.
func (r toolRun) runFailure(tool string) error {
	tail := r.stderrTail(10)
	if tail == "" {
		return fmt.Errorf("%s exited %d without producing a report", tool, r.ExitCode)
	}
	return fmt.Errorf("%s exited %d without producing a report:\n%s", tool, r.ExitCode, tail)
}
