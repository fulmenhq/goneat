/*
Copyright © 2025 3 Leaps <info@3leaps.com>
*/
package integration

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// GitRepoFixture represents a test environment with git repository setup
type GitRepoFixture struct {
	*TestEnv
	repoDir string
}

// CreateGitRepoFixture creates a test environment with an initialized git repository
func CreateGitRepoFixture(t *testing.T, initialVersion string) *GitRepoFixture {
	env := NewTestEnv(t)

	// Initialize git repository
	env.runCommand("git", "init")
	env.runCommand("git", "config", "user.name", "Test User")
	env.runCommand("git", "config", "user.email", "test@example.com")

	// Create VERSION file and initial commit
	env.WriteFile("VERSION", initialVersion)
	env.runCommand("git", "add", "VERSION")
	env.runCommand("git", "commit", "-m", fmt.Sprintf("Initial commit with version %s", initialVersion))

	// Create initial tag
	env.runCommand("git", "tag", fmt.Sprintf("v%s", initialVersion))

	return &GitRepoFixture{
		TestEnv: env,
		repoDir: env.Dir,
	}
}

// GitTag creates a git tag in the repository
func (f *GitRepoFixture) GitTag(tag string) {
	f.runGitCommand("tag", tag)
}

// GitAdd stages files for commit
func (f *GitRepoFixture) GitAdd(files ...string) {
	args := append([]string{"add"}, files...)
	f.runGitCommand(args...)
}

// GitCommit creates a commit with the given message
func (f *GitRepoFixture) GitCommit(message string) {
	f.runGitCommand("commit", "-m", message)
}

// GitCheckout switches to a different branch/commit
func (f *GitRepoFixture) GitCheckout(target string) {
	f.runGitCommand("checkout", target)
}

// ListGitTags returns a list of all git tags
func (f *GitRepoFixture) ListGitTags() []string {
	output := f.runGitCommand("tag", "-l")
	return strings.Split(strings.TrimSpace(output), "\n")
}

// runGitCommand executes a git command in the repository
func (f *GitRepoFixture) runGitCommand(args ...string) string {
	// Add timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204 - test helper; args are literal git subcommands in test setup (e.g. "init", "add", "commit"), not user-controlled input
	cmd.Dir = f.repoDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("Git command failed: git %s\nOutput: %s\nError: %v", strings.Join(args, " "), string(output), err)
	}

	return string(output)
}

// MultiSourceFixture represents a test environment with multiple version sources
type MultiSourceFixture struct {
	*TestEnv
}

// CreateMultiSourceFixture creates a test environment with multiple version sources
func CreateMultiSourceFixture(t *testing.T) *MultiSourceFixture {
	env := NewTestEnv(t)
	return &MultiSourceFixture{TestEnv: env}
}

// WriteGoConst creates a Go file with a version constant
func (f *MultiSourceFixture) WriteGoConst(filename, version string) {
	content := fmt.Sprintf(`package main

const Version = "%s"
`, version)
	f.WriteFile(filename, content)
}

// WriteGoMod creates a go.mod file with a version
func (f *MultiSourceFixture) WriteGoMod(moduleName, version string) {
	content := fmt.Sprintf(`module %s

go 1.21

require (
	github.com/spf13/cobra v1.8.0
)
`, moduleName)
	f.WriteFile("go.mod", content)
}

// WritePackageJSON creates a package.json file with a version
func (f *MultiSourceFixture) WritePackageJSON(version string) {
	content := fmt.Sprintf(`{
  "name": "test-package",
  "version": "%s",
  "description": "Test package for version management"
}`, version)
	f.WriteFile("package.json", content)
}

// CreateComplexVersionFileFixture creates a fixture with various version file scenarios
func CreateComplexVersionFileFixture(t *testing.T, scenarios []VersionScenario) *TestEnv {
	env := NewTestEnv(t)

	for _, scenario := range scenarios {
		env.WriteFile(scenario.Filename, scenario.Content)
	}

	return env
}

// VersionScenario represents a test scenario for version files
type VersionScenario struct {
	Filename string
	Content  string
}

// Common version scenarios for testing
var (
	// Standard semver versions
	StandardVersions = []string{
		"1.2.3",
		"v1.2.3",
		"1.2.3-alpha",
		"1.2.3-rc.1",
		"1.2.3+build.1",
		"v1.2.3-beta.2+build.456",
	}

	// Invalid versions for error testing
	InvalidVersions = []string{
		"1.2.3.4",                    // Too many segments
		"1.2",                        // Missing patch
		"a.b.c",                      // Non-numeric
		"1.2.3-beta.1+build.1+extra", // Multiple build metadata
		"",                           // Empty
		"not-a-version",              // Non-numeric
	}

	// Edge cases
	EdgeCaseVersions = []string{
		"0.0.0",                            // Zero version
		"999.999.999",                      // Large version
		"1.0.0-alpha.123.beta.456",         // Complex prerelease
		"2.0.0+build.with.dots.and.dashes", // Complex build metadata
	}
)
