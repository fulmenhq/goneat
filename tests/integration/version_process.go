package integration

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// executableDigest observes pathname bytes, not an immutable process image.
// A missing or changing file is diagnostic evidence, never a replacement error.
func executableDigest(path string) string {
	file, err := os.Open(path) // #nosec G304 - selected test executable path
	if err != nil {
		return fmt.Sprintf("unavailable: %v", err)
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Sprintf("unavailable: %v", err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

// runVersionProcess preserves exit/output behavior and adds failure-only evidence.
// The caller retains the existing 30s timeout. Hash observations bracket execution
// but cannot establish that the pathname was unchanged at every instant between.
func runVersionProcess(ctx context.Context, path, dir string, args []string) (VersionCommandResult, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return VersionCommandResult{ExitCode: -1, Error: fmt.Sprintf("resolve executable: %v", err)}, err
	}
	before := executableDigest(absolute)
	started := time.Now()
	cmd := exec.CommandContext(ctx, absolute, args...) // #nosec G204 - test executable and controlled test arguments
	cmd.Dir = dir
	output, execErr := cmd.CombinedOutput()
	elapsed := time.Since(started)
	contextErr := ctx.Err()
	after := executableDigest(absolute)
	result := VersionCommandResult{Output: strings.TrimSpace(string(output))}
	if execErr == nil {
		return result, nil
	}
	result.ExitCode = -1
	var exitErr *exec.ExitError
	var spawnErr error
	if errors.As(execErr, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
	} else {
		spawnErr = execErr
	}
	state, pid := "unavailable (process did not start)", "unavailable"
	if cmd.ProcessState != nil {
		state = cmd.ProcessState.String() // Includes signal information where supported.
	}
	if cmd.Process != nil {
		pid = fmt.Sprint(cmd.Process.Pid)
	}
	result.Error = fmt.Sprintf("%s\ncommand failure: executable=%q argv=%q pid=%s started=%s elapsed=%s exit=%d state=%q exec_error=%v context_error=%v sha256_before=%q sha256_after=%q pathname_bytes_equal=%t (not immutable process-image proof)",
		result.Output, absolute, args, pid, started.UTC().Format(time.RFC3339Nano), elapsed, result.ExitCode, state, execErr, contextErr, before, after,
		before == after && !strings.HasPrefix(before, "unavailable:"))
	return result, spawnErr
}
