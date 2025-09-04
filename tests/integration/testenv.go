/*
Copyright © 2025 3 Leaps <info@3leaps.com>
*/
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnv provides a temporary directory environment for integration testing
type TestEnv struct {
	Dir     string
	t       *testing.T
	cleanup func()
}

// NewTestEnv creates a new test environment with a temporary directory
func NewTestEnv(t *testing.T) *TestEnv {
	dir, err := os.MkdirTemp("", "goneat-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	env := &TestEnv{
		Dir: dir,
		t:   t,
	}

	// Set up cleanup
	env.cleanup = func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("Failed to cleanup test dir %s: %v", dir, err)
		}
	}

	// Register cleanup with testing framework
	t.Cleanup(env.cleanup)

	return env
}

// Cleanup manually cleans up the test environment
func (env *TestEnv) Cleanup() {
	if env.cleanup != nil {
		env.cleanup()
		env.cleanup = nil
	}
}

// WriteFile writes content to a file in the test environment
func (env *TestEnv) WriteFile(filename, content string) {
	path := filepath.Join(env.Dir, filename)
	dir := filepath.Dir(path)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0750); err != nil {
		env.t.Fatalf("Failed to create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		env.t.Fatalf("Failed to write file %s: %v", path, err)
	}
}

// ReadFile reads content from a file in the test environment
func (env *TestEnv) ReadFile(filename string) string {
	path := filepath.Join(env.Dir, filename)
	// Validate path to prevent path traversal in tests
	path = filepath.Clean(path)
	if strings.Contains(path, "..") {
		env.t.Fatalf("Invalid path with traversal: %s", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		env.t.Fatalf("Failed to read file %s: %v", path, err)
	}
	return string(content)
}

// FileExists checks if a file exists in the test environment
func (env *TestEnv) FileExists(filename string) bool {
	path := filepath.Join(env.Dir, filename)
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// RemoveFile removes a file from the test environment
func (env *TestEnv) RemoveFile(filename string) {
	path := filepath.Join(env.Dir, filename)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		env.t.Errorf("Failed to remove file %s: %v", path, err)
	}
}

// VersionCommandResult represents the result of running a version command
type VersionCommandResult struct {
	Version   string
	Component string
	Output    string
	Error     string
	ExitCode  int
}

// RunVersionCommand runs a goneat version command in the test environment
func (env *TestEnv) RunVersionCommand(args ...string) VersionCommandResult {
	// Build the command
	cmdArgs := []string{"version"}
	cmdArgs = append(cmdArgs, args...)

	// Find the goneat binary - assume it's in the project root
	goneatPath := env.findGoneatBinary()
	if goneatPath == "" {
		env.t.Fatalf("Could not find goneat binary")
	}

	// Clean path to prevent path traversal issues
	goneatPath = filepath.Clean(goneatPath)
	cmd := exec.Command(goneatPath, cmdArgs...) // #nosec G204
	cmd.Dir = env.Dir

	// Capture output
	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			env.t.Fatalf("Failed to run command: %v", err)
		}
	}

	result := VersionCommandResult{
		Output:   strings.TrimSpace(string(output)),
		ExitCode: exitCode,
	}

	// Parse version from output if successful
	if exitCode == 0 && result.Output != "" {
		env.parseVersionOutput(&result)
	}

	// Extract error message if command failed
	if exitCode != 0 {
		result.Error = result.Output
	}

	return result
}

// parseVersionOutput extracts version information from command output
func (env *TestEnv) parseVersionOutput(result *VersionCommandResult) {
	for line := range strings.SplitSeq(result.Output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "goneat ") {
			// Extract version from "goneat 1.2.3" format
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				result.Component = parts[0]
				result.Version = parts[1]
			}
		} else if strings.Contains(line, "Current version:") || strings.Contains(line, "[NO-OP] Current version:") {
			// Extract version from "Current version: 1.2.3" format
			if idx := strings.LastIndex(line, ":"); idx != -1 {
				result.Version = strings.TrimSpace(line[idx+1:])
			}
		}
	}
}

// runCommand executes a command in the test environment directory
func (env *TestEnv) runCommand(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Dir = env.Dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		env.t.Fatalf("Command failed: %s %s\nOutput: %s\nError: %v", name, strings.Join(args, " "), string(output), err)
	}

	return string(output)
}

// findGoneatBinary locates the goneat binary for testing
func (env *TestEnv) findGoneatBinary() string {
	// First, try to find it relative to the current working directory
	// This assumes we're running tests from the goneat module directory
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Look for goneat binary in various locations
	possiblePaths := []string{
		filepath.Join(cwd, "..", "..", "dist", "goneat"), // dist/ directory from tests/integration
		filepath.Join(cwd, "..", "dist", "goneat"),       // dist/ directory from tests
		filepath.Join(cwd, "dist", "goneat"),             // dist/ directory from root
		filepath.Join(cwd, "..", "goneat"),               // Parent directory (legacy)
		filepath.Join(cwd, "bin", "goneat"),              // bin/ directory (legacy)
		"goneat",                                         // In PATH
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			// Check if it's executable
			if info, err := os.Stat(path); err == nil && (info.Mode()&0111 != 0) {
				return path
			}
		}
	}

	return ""
}

// CreateVersionFileFixture creates a test environment with a VERSION file
func CreateVersionFileFixture(t *testing.T, version string) *TestEnv {
	env := NewTestEnv(t)
	env.WriteFile("VERSION", version)
	return env
}

// CreateEmptyFixture creates a test environment with no VERSION file
func CreateEmptyFixture(t *testing.T) *TestEnv {
	return NewTestEnv(t)
}
