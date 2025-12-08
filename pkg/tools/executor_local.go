/*
Copyright © 2025 3 Leaps <info@3leaps.com>
*/
package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// LocalExecutor runs tools installed on the local system
type LocalExecutor struct {
	shimDirs []string
}

// NewLocalExecutor creates a new LocalExecutor
func NewLocalExecutor() *LocalExecutor {
	return &LocalExecutor{
		shimDirs: getShimDirectories(),
	}
}

// Name returns the executor name
func (e *LocalExecutor) Name() string {
	return "local"
}

// IsAvailable checks if the tool is available locally
func (e *LocalExecutor) IsAvailable(tool string) bool {
	return e.FindToolPath(tool) != ""
}

// Execute runs the tool locally
func (e *LocalExecutor) Execute(ctx context.Context, opts ExecuteOptions) (*ExecuteResult, error) {
	toolPath := e.FindToolPath(opts.Tool)
	if toolPath == "" {
		return nil, fmt.Errorf("tool %s not found in PATH or shim directories", opts.Tool)
	}

	// #nosec G204 - toolPath is validated via FindToolPath
	cmd := exec.CommandContext(ctx, toolPath, opts.Args...)

	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}

	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	}

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range opts.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &ExecuteResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		Executor: "local",
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			// Return result with exit code, not an error
			// The caller can check ExitCode to determine success/failure
			return result, nil
		}
		return nil, fmt.Errorf("failed to execute %s: %w", opts.Tool, err)
	}

	return result, nil
}

// FindToolPath finds a tool by name, checking PATH first then known shim directories.
// This handles tools installed via brew, bun, go-install, etc. that may not be in PATH
// (e.g., in CI environments where PATH wasn't updated after bootstrap).
func (e *LocalExecutor) FindToolPath(toolName string) string {
	// First check PATH (fast path for normal case)
	if path, err := exec.LookPath(toolName); err == nil {
		return path
	}

	// Check brew bin directory (covers tools installed via brew)
	if _, brewPath, err := DetectBrew(); err == nil && brewPath != "" {
		binDir := filepath.Dir(brewPath)
		candidate := filepath.Join(binDir, toolName)
		if _, err := os.Stat(candidate); err == nil {
			logger.Debug(fmt.Sprintf("found %s in brew bin: %s", toolName, candidate))
			return candidate
		}
	}

	// Check shim directories
	for _, shimDir := range e.shimDirs {
		if shimDir == "" {
			continue
		}
		candidate := filepath.Join(shimDir, toolName)
		if runtime.GOOS == "windows" && !strings.HasSuffix(candidate, ".exe") {
			candidate += ".exe"
		}
		if _, err := os.Stat(candidate); err == nil {
			logger.Debug(fmt.Sprintf("found %s in shim dir: %s", toolName, candidate))
			return candidate
		}
	}

	return ""
}

// getShimDirectories returns known shim directories for various package managers
// This is a standalone implementation to avoid import cycles with internal/doctor
func getShimDirectories() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	dirs := []string{}

	// bun installs to ~/.bun/bin
	bunDir := filepath.Join(homeDir, ".bun", "bin")
	if _, err := os.Stat(bunDir); err == nil {
		dirs = append(dirs, bunDir)
	}

	// go install uses GOBIN or ~/go/bin
	goDir := os.Getenv("GOBIN")
	if goDir == "" {
		goDir = filepath.Join(homeDir, "go", "bin")
	}
	if _, err := os.Stat(goDir); err == nil {
		dirs = append(dirs, goDir)
	}

	// mise installs to ~/.local/share/mise/shims
	miseDir := filepath.Join(homeDir, ".local", "share", "mise", "shims")
	if _, err := os.Stat(miseDir); err == nil {
		dirs = append(dirs, miseDir)
	}

	// scoop installs to ~/scoop/shims (Windows)
	if runtime.GOOS == "windows" {
		scoopDir := filepath.Join(homeDir, "scoop", "shims")
		if _, err := os.Stat(scoopDir); err == nil {
			dirs = append(dirs, scoopDir)
		}
	}

	return dirs
}
