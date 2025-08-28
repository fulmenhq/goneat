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
