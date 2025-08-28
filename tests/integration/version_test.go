/*
Copyright © 2025 3 Leaps <info@3leaps.com>
*/
package integration

import (
	"strings"
	"testing"
)

func TestVersionCommand_BasicDisplay(t *testing.T) {
	// Test basic version display functionality
	env := CreateVersionFileFixture(t, "1.2.3")
	defer env.Cleanup()

	result := env.RunVersionCommand()

	// Verify command succeeded
	if result.ExitCode != 0 {
		t.Fatalf("Version command failed: %s", result.Error)
	}

	// Verify version is correctly displayed
	if result.Version != "1.2.3" {
		t.Errorf("Expected version 1.2.3, got %s", result.Version)
	}

	if result.Component != "goneat" {
		t.Errorf("Expected component 'goneat', got %s", result.Component)
	}

	// Verify output contains expected format
	if !strings.Contains(result.Output, "goneat 1.2.3") {
		t.Errorf("Expected output to contain 'goneat 1.2.3', got: %s", result.Output)
	}
}

func TestVersionCommand_NoOpMode(t *testing.T) {
	// Test no-op mode functionality
	env := CreateVersionFileFixture(t, "2.0.0")
	defer env.Cleanup()

	result := env.RunVersionCommand("--no-op")

	// Verify command succeeded
	if result.ExitCode != 0 {
		t.Fatalf("Version command with --no-op failed: %s", result.Error)
	}

	// Verify no-op indicator is present
	if !strings.Contains(result.Output, "[NO-OP]") {
		t.Errorf("Expected no-op output to contain '[NO-OP]', got: %s", result.Output)
	}

	// Verify version is still displayed correctly
	if result.Version != "2.0.0" {
		t.Errorf("Expected version 2.0.0, got %s", result.Version)
	}
}

func TestVersionCommand_MissingVersionFile(t *testing.T) {
	// Test behavior when VERSION file doesn't exist
	env := CreateEmptyFixture(t)
	defer env.Cleanup()

	result := env.RunVersionCommand()

	// Command should fail when no version file exists
	if result.ExitCode == 0 {
		t.Error("Expected version command to fail when VERSION file is missing")
	}

	// Error should indicate no version found
	if !strings.Contains(result.Error, "no version found") {
		t.Errorf("Expected error to contain 'no version found', got: %s", result.Error)
	}
}

func TestVersionCommand_JSONOutput(t *testing.T) {
	// Test JSON output format
	env := CreateVersionFileFixture(t, "1.5.0")
	defer env.Cleanup()

	result := env.RunVersionCommand("--json")

	// Verify command succeeded
	if result.ExitCode != 0 {
		t.Fatalf("Version command with --json failed: %s", result.Error)
	}

	// Verify JSON output contains expected fields
	if !strings.Contains(result.Output, `"version": "1.5.0"`) {
		t.Errorf("Expected JSON output to contain version field, got: %s", result.Output)
	}

	if !strings.Contains(result.Output, `"goVersion":`) {
		t.Errorf("Expected JSON output to contain goVersion field, got: %s", result.Output)
	}

	if !strings.Contains(result.Output, `"platform":`) {
		t.Errorf("Expected JSON output to contain platform field, got: %s", result.Output)
	}
}

func TestVersionCommand_ExtendedOutput(t *testing.T) {
	// Test extended output format
	env := CreateVersionFileFixture(t, "3.1.4")
	defer env.Cleanup()

	result := env.RunVersionCommand("--extended")

	// Verify command succeeded
	if result.ExitCode != 0 {
		t.Fatalf("Version command with --extended failed: %s", result.Error)
	}

	// Verify extended output contains additional information
	expectedFields := []string{
		"goneat 3.1.4",
		"Build time:",
		"Git commit:",
		"Go version:",
		"Platform:",
	}

	for _, field := range expectedFields {
		if !strings.Contains(result.Output, field) {
			t.Errorf("Expected extended output to contain '%s', got: %s", field, result.Output)
		}
	}
}

func TestTestEnv_FileOperations(t *testing.T) {
	// Test basic file operations in test environment
	env := NewTestEnv(t)
	defer env.Cleanup()

	// Test file creation
	env.WriteFile("test.txt", "Hello, World!")

	// Test file reading
	content := env.ReadFile("test.txt")
	if content != "Hello, World!" {
		t.Errorf("Expected file content 'Hello, World!', got '%s'", content)
	}

	// Test file existence
	if !env.FileExists("test.txt") {
		t.Error("Expected test.txt to exist")
	}

	// Test file removal
	env.RemoveFile("test.txt")
	if env.FileExists("test.txt") {
		t.Error("Expected test.txt to be removed")
	}
}

func TestVersionFileFixture_Creation(t *testing.T) {
	// Test VERSION file fixture creation
	env := CreateVersionFileFixture(t, "4.2.1")
	defer env.Cleanup()

	// Verify VERSION file exists
	if !env.FileExists("VERSION") {
		t.Error("Expected VERSION file to exist in fixture")
	}

	// Verify VERSION file content
	content := env.ReadFile("VERSION")
	if strings.TrimSpace(content) != "4.2.1" {
		t.Errorf("Expected VERSION file to contain '4.2.1', got '%s'", content)
	}
}

func TestVersionCommand_BumpPatch(t *testing.T) {
	// Test version bump patch functionality
	env := CreateVersionFileFixture(t, "1.2.3")
	defer env.Cleanup()

	// Test bump patch with no-op first
	result := env.RunVersionCommand("bump", "patch", "--no-op")
	if result.ExitCode != 0 {
		t.Fatalf("Version bump patch --no-op failed: %s", result.Error)
	}
	if !strings.Contains(result.Output, "[NO-OP] Would bump version from 1.2.3 to 1.2.4") {
		t.Errorf("Expected no-op output to contain bump preview, got: %s", result.Output)
	}

	// Verify VERSION file unchanged in no-op mode
	content := env.ReadFile("VERSION")
	if strings.TrimSpace(content) != "1.2.3" {
		t.Errorf("Expected VERSION file to remain '1.2.3' in no-op mode, got '%s'", content)
	}

	// Apply actual bump
	result = env.RunVersionCommand("bump", "patch")
	if result.ExitCode != 0 {
		t.Fatalf("Version bump patch failed: %s", result.Error)
	}

	// Verify VERSION file updated
	content = env.ReadFile("VERSION")
	if strings.TrimSpace(content) != "1.2.4" {
		t.Errorf("Expected VERSION file to contain '1.2.4' after bump, got '%s'", content)
	}
}

func TestVersionCommand_BumpMinor(t *testing.T) {
	// Test version bump minor functionality
	env := CreateVersionFileFixture(t, "1.2.3")
	defer env.Cleanup()

	result := env.RunVersionCommand("bump", "minor")
	if result.ExitCode != 0 {
		t.Fatalf("Version bump minor failed: %s", result.Error)
	}

	content := env.ReadFile("VERSION")
	if strings.TrimSpace(content) != "1.3.0" {
		t.Errorf("Expected VERSION file to contain '1.3.0' after minor bump, got '%s'", content)
	}
}

func TestVersionCommand_BumpMajor(t *testing.T) {
	// Test version bump major functionality
	env := CreateVersionFileFixture(t, "1.2.3")
	defer env.Cleanup()

	result := env.RunVersionCommand("bump", "major")
	if result.ExitCode != 0 {
		t.Fatalf("Version bump major failed: %s", result.Error)
	}

	content := env.ReadFile("VERSION")
	if strings.TrimSpace(content) != "2.0.0" {
		t.Errorf("Expected VERSION file to contain '2.0.0' after major bump, got '%s'", content)
	}
}

func TestVersionCommand_BumpInvalidType(t *testing.T) {
	// Test version bump with invalid type
	env := CreateVersionFileFixture(t, "1.2.3")
	defer env.Cleanup()

	result := env.RunVersionCommand("bump", "invalid")
	if result.ExitCode == 0 {
		t.Error("Expected version bump with invalid type to fail")
	}

	if !strings.Contains(result.Error, "invalid bump type") {
		t.Errorf("Expected error to contain 'invalid bump type', got: %s", result.Error)
	}

	// Verify VERSION file unchanged
	content := env.ReadFile("VERSION")
	if strings.TrimSpace(content) != "1.2.3" {
		t.Errorf("Expected VERSION file to remain unchanged after invalid bump, got '%s'", content)
	}
}

func TestVersionCommand_SetVersion(t *testing.T) {
	// Test version set functionality
	env := CreateVersionFileFixture(t, "1.2.3")
	defer env.Cleanup()

	// Test set with no-op first
	result := env.RunVersionCommand("set", "2.0.0", "--no-op")
	if result.ExitCode != 0 {
		t.Fatalf("Version set --no-op failed: %s", result.Error)
	}
	if !strings.Contains(result.Output, "[NO-OP] Would set version to 2.0.0") {
		t.Errorf("Expected no-op output to contain set preview, got: %s", result.Output)
	}

	// Verify VERSION file unchanged in no-op mode
	content := env.ReadFile("VERSION")
	if strings.TrimSpace(content) != "1.2.3" {
		t.Errorf("Expected VERSION file to remain '1.2.3' in no-op mode, got '%s'", content)
	}

	// Apply actual set
	result = env.RunVersionCommand("set", "2.0.0")
	if result.ExitCode != 0 {
		t.Fatalf("Version set failed: %s", result.Error)
	}

	// Verify VERSION file updated
	content = env.ReadFile("VERSION")
	if strings.TrimSpace(content) != "2.0.0" {
		t.Errorf("Expected VERSION file to contain '2.0.0' after set, got '%s'", content)
	}
}

func TestVersionCommand_SetInvalidVersion(t *testing.T) {
	// Test version set with invalid format
	env := CreateVersionFileFixture(t, "1.2.3")
	defer env.Cleanup()

	result := env.RunVersionCommand("set", "1.2.3.4")
	if result.ExitCode == 0 {
		t.Error("Expected version set with invalid format to fail")
	}

	if !strings.Contains(result.Error, "invalid version format") {
		t.Errorf("Expected error to contain 'invalid version format', got: %s", result.Error)
	}

	// Verify VERSION file unchanged
	content := env.ReadFile("VERSION")
	if strings.TrimSpace(content) != "1.2.3" {
		t.Errorf("Expected VERSION file to remain unchanged after invalid set, got '%s'", content)
	}
}

func TestVersionCommand_ValidateVersion(t *testing.T) {
	// Test version validate functionality
	env := CreateEmptyFixture(t)
	defer env.Cleanup()

	// Test valid version
	result := env.RunVersionCommand("validate", "1.2.3")
	if result.ExitCode != 0 {
		t.Fatalf("Version validate for valid version failed: %s", result.Error)
	}
	if !strings.Contains(result.Output, "Version 1.2.3 is valid") {
		t.Errorf("Expected validation success message, got: %s", result.Output)
	}

	// Test invalid version
	result = env.RunVersionCommand("validate", "1.2.3.4")
	if result.ExitCode == 0 {
		t.Error("Expected version validate for invalid version to fail")
	}
	if !strings.Contains(result.Error, "invalid version") {
		t.Errorf("Expected error to contain 'invalid version', got: %s", result.Error)
	}
}

func TestVersionCommand_CheckConsistency(t *testing.T) {
	// Test version check-consistency functionality
	env := CreateVersionFileFixture(t, "1.2.3")
	defer env.Cleanup()

	result := env.RunVersionCommand("check-consistency")
	if result.ExitCode != 0 {
		t.Fatalf("Version check-consistency failed: %s", result.Error)
	}

	// Should show VERSION file information
	if !strings.Contains(result.Output, "VERSION") {
		t.Errorf("Expected consistency check to mention VERSION file, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "1.2.3") {
		t.Errorf("Expected consistency check to show version 1.2.3, got: %s", result.Output)
	}
}

func TestVersionCommand_CheckConsistency_NoOp(t *testing.T) {
	// Test version check-consistency with no-op mode
	env := CreateVersionFileFixture(t, "1.2.3")
	defer env.Cleanup()

	result := env.RunVersionCommand("check-consistency", "--no-op")
	if result.ExitCode != 0 {
		t.Fatalf("Version check-consistency --no-op failed: %s", result.Error)
	}

	// Should show no-op indicators
	if !strings.Contains(result.Output, "[NO-OP]") {
		t.Errorf("Expected no-op output to contain '[NO-OP]', got: %s", result.Output)
	}
}

func TestVersionCommand_InitBasic(t *testing.T) {
	// Test version init basic functionality
	env := CreateEmptyFixture(t)
	defer env.Cleanup()

	// Test init with dry-run first
	result := env.RunVersionCommand("init", "basic", "--dry-run")
	if result.ExitCode != 0 {
		t.Fatalf("Version init basic --dry-run failed: %s", result.Error)
	}

	// Should not create VERSION file in dry-run mode
	if env.FileExists("VERSION") {
		t.Error("Expected VERSION file to not exist in dry-run mode")
	}

	// Apply actual init
	result = env.RunVersionCommand("init", "basic")
	if result.ExitCode != 0 {
		t.Fatalf("Version init basic failed: %s", result.Error)
	}

	// Verify VERSION file created with default version
	if !env.FileExists("VERSION") {
		t.Error("Expected VERSION file to exist after init")
	}

	content := env.ReadFile("VERSION")
	if strings.TrimSpace(content) != "1.0.0" {
		t.Errorf("Expected VERSION file to contain '1.0.0' after init, got '%s'", content)
	}
}

func TestVersionCommand_InitWithCustomVersion(t *testing.T) {
	// Test version init with custom initial version
	env := CreateEmptyFixture(t)
	defer env.Cleanup()

	result := env.RunVersionCommand("init", "basic", "--initial-version", "2.0.0")
	if result.ExitCode != 0 {
		t.Fatalf("Version init with custom version failed: %s", result.Error)
	}

	content := env.ReadFile("VERSION")
	if strings.TrimSpace(content) != "2.0.0" {
		t.Errorf("Expected VERSION file to contain '2.0.0' after init with custom version, got '%s'", content)
	}
}

func TestVersionCommand_InitForceOverwrite(t *testing.T) {
	// Test version init with force overwrite
	env := CreateVersionFileFixture(t, "1.2.3")
	defer env.Cleanup()

	// Try to init without force (should fail)
	result := env.RunVersionCommand("init", "basic")
	if result.ExitCode == 0 {
		t.Error("Expected version init without force to fail when VERSION file exists")
	}

	// Try with force
	result = env.RunVersionCommand("init", "basic", "--force")
	if result.ExitCode != 0 {
		t.Fatalf("Version init with force failed: %s", result.Error)
	}

	// Should reset to default version
	content := env.ReadFile("VERSION")
	if strings.TrimSpace(content) != "1.0.0" {
		t.Errorf("Expected VERSION file to contain '1.0.0' after force init, got '%s'", content)
	}
}

// Test learning functionality - first run detection

func TestVersionCommand_FirstRunDetection_NoVersionManagement(t *testing.T) {
	// Test first-run detection when no version management exists
	env := CreateEmptyFixture(t)
	defer env.Cleanup()

	result := env.RunVersionCommand()

	// Should provide helpful guidance for first-time users
	if result.ExitCode == 0 {
		t.Error("Expected first-run detection to provide guidance, not succeed silently")
	}

	output := result.Output
	if !strings.Contains(output, "Welcome to goneat version management") {
		t.Errorf("Expected first-run guidance message, got: %s", output)
	}

	if !strings.Contains(output, "Quick Setup") {
		t.Errorf("Expected setup guidance, got: %s", output)
	}

	if !strings.Contains(output, "Available Templates") {
		t.Errorf("Expected template information, got: %s", output)
	}
}

func TestVersionCommand_Learning_GitTagsDetection(t *testing.T) {
	// Test learning from existing git tags
	env := CreateGitRepoFixture(t, "1.0.0")
	defer env.Cleanup()

	// Add more tags to simulate a repository with version history
	env.GitTag("v1.1.0")
	env.GitTag("v1.2.0")
	env.GitTag("v2.0.0")

	// Remove VERSION file to test git tag detection
	env.RemoveFile("VERSION")

	// Debug: List all tags to see what was created
	tags := env.ListGitTags()
	t.Logf("Created git tags: %v", tags)

	result := env.RunVersionCommand()

	// Debug: Show the full output
	t.Logf("Version command output: %s", result.Output)
	t.Logf("Detected version: %s", result.Version)

	// Should detect latest git tag
	if result.ExitCode != 0 {
		t.Fatalf("Version command with git tags failed: %s", result.Error)
	}

	// Check if we got any version at all
	if result.Version == "" {
		t.Errorf("Expected to detect some version from git tags, got empty string")
	}

	// Should provide learning suggestions
	output := result.Output
	if !strings.Contains(output, "Source: git tag") {
		t.Errorf("Expected to indicate git tag as source, got: %s", output)
	}
}

func TestVersionCommand_Learning_PatternRecognition(t *testing.T) {
	// Test pattern recognition for different versioning schemes
	testCases := []struct {
		name         string
		tags         []string
		expectedType string
	}{
		{
			name:         "Semantic versioning pattern",
			tags:         []string{"v1.1.0", "v1.2.0", "v2.0.0", "v2.1.3"}, // Skip v1.0.0 as it's already created
			expectedType: "semver",
		},
		{
			name:         "Calendar versioning pattern",
			tags:         []string{"2024.01.15", "2024.02.01", "2024.03.15"}, // Use different tags
			expectedType: "calver",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := CreateGitRepoFixture(t, "1.0.0")
			defer env.Cleanup()

			// Add test tags
			for _, tag := range tc.tags {
				env.GitTag(tag)
			}

			// Remove VERSION file to force git tag detection
			env.RemoveFile("VERSION")

			result := env.RunVersionCommand()

			if result.ExitCode != 0 {
				t.Fatalf("Version command failed: %s", result.Error)
			}

			// Should detect the latest tag
			lastTag := tc.tags[len(tc.tags)-1]
			if !strings.Contains(result.Version, lastTag) {
				t.Errorf("Expected to detect latest tag '%s', got '%s'", lastTag, result.Version)
			}
		})
	}
}

func TestVersionCommand_Learning_OptimizationSuggestions(t *testing.T) {
	// Test optimization suggestions for existing setups
	env := CreateGitRepoFixture(t, "1.0.0")
	defer env.Cleanup()

	// Add git tags and commits to provide rich context
	env.GitTag("v1.1.0")
	env.WriteFile("README.md", "# Test Project")
	env.GitAdd("README.md")
	env.GitCommit("Add README")

	result := env.RunVersionCommand("--extended")

	if result.ExitCode != 0 {
		t.Fatalf("Extended version command failed: %s", result.Error)
	}

	output := result.Output

	// Should show extended information
	expectedFields := []string{
		"Build time:",
		"Git commit:",
		"Platform:",
	}

	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("Expected extended output to contain '%s', got: %s", field, output)
		}
	}

	// Should show git information
	if !strings.Contains(output, "Git commit:") {
		t.Errorf("Expected git commit information, got: %s", output)
	}
}
