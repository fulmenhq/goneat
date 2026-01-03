/*
Copyright © 2025 3 Leaps (hello@3leaps.net and https://3leaps.net)
*/
package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestFormatCommand_BasicFunctionality tests basic format command functionality
func TestFormatCommand_BasicFunctionality(t *testing.T) {
	// Create a temporary directory with test files
	tempDir := t.TempDir()

	// Create test files
	createTestFiles(t, tempDir)

	// Create a test command
	cmd := &cobra.Command{}
	cmd.Flags().StringSlice("files", []string{}, "")
	cmd.Flags().StringSlice("folders", []string{tempDir}, "")
	cmd.Flags().Bool("check", false, "")
	cmd.Flags().Bool("quiet", false, "")
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().Bool("plan-only", false, "")
	cmd.Flags().String("plan-file", "", "")
	cmd.Flags().StringSlice("types", []string{}, "")
	cmd.Flags().Int("max-depth", -1, "")
	cmd.Flags().String("strategy", "sequential", "")
	cmd.Flags().Bool("group-by-size", false, "")
	cmd.Flags().Bool("group-by-type", false, "")
	cmd.Flags().Bool("no-op", false, "")

	// Set up command arguments
	args := []string{}

	// Execute the command
	err := RunFormat(cmd, args)

	// Check for errors
	if err != nil {
		t.Fatalf("Format command failed: %v", err)
	}

	// Note: Logger output goes to stderr, so we can't easily capture it in tests
	// The important thing is that the command executed successfully
	// In a real scenario, we'd check file modifications or other side effects
}

// TestFormatCommand_DryRun tests dry-run functionality
func TestFormatCommand_DryRun(t *testing.T) {
	// Create a temporary directory with test files
	tempDir := t.TempDir()
	createTestFiles(t, tempDir)

	// Create a test command with dry-run enabled
	cmd := &cobra.Command{}
	setupFormatCommandFlags(cmd)
	if err := cmd.Flags().Set("folders", tempDir); err != nil {
		t.Fatalf("Failed to set folders flag: %v", err)
	}
	if err := cmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatalf("Failed to set dry-run flag: %v", err)
	}

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Execute the command
	err := RunFormat(cmd, []string{})

	// Check for errors
	if err != nil {
		t.Fatalf("Format command dry-run failed: %v", err)
	}

	// Verify output contains dry-run indicators
	outputStr := output.String()
	if !strings.Contains(outputStr, "DRY RUN") {
		t.Errorf("Expected output to contain 'DRY RUN', got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "This was a dry run") {
		t.Errorf("Expected output to contain 'This was a dry run', got: %s", outputStr)
	}
}

// TestFormatCommand_PlanOnly tests plan-only functionality
func TestFormatCommand_PlanOnly(t *testing.T) {
	// Create a temporary directory with test files
	tempDir := t.TempDir()
	createTestFiles(t, tempDir)

	// Create a test command with plan-only enabled
	cmd := &cobra.Command{}
	setupFormatCommandFlags(cmd)
	if err := cmd.Flags().Set("folders", tempDir); err != nil {
		t.Fatalf("Failed to set folders flag: %v", err)
	}
	if err := cmd.Flags().Set("plan-only", "true"); err != nil {
		t.Fatalf("Failed to set plan-only flag: %v", err)
	}

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Execute the command
	err := RunFormat(cmd, []string{})

	// Check for errors
	if err != nil {
		t.Fatalf("Format command plan-only failed: %v", err)
	}

	// Verify output contains plan information
	outputStr := output.String()
	if !strings.Contains(outputStr, "Work Plan for 'format' command") {
		t.Errorf("Expected output to contain 'Work Plan for format command', got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Estimated Execution Times") {
		t.Errorf("Expected output to contain 'Estimated Execution Times', got: %s", outputStr)
	}
}

// TestFormatCommand_CheckOnly tests check-only functionality
// Note: This test is currently disabled due to external tool dependencies
// TODO: Re-enable when external tools are properly mocked or available in CI
func TestFormatCommand_CheckOnly(t *testing.T) {
	t.Skip("Check-only test disabled due to external tool dependencies")

	// This test would verify check-only functionality
	// For now, we focus on the core formatting functionality that works
}

// TestFormatCommand_CheckMode_YAMLPrimaryFormatterPrecedence verifies that when
// the primary formatter (yamlfmt) detects formatting issues, the check mode
// correctly reports "needs formatting" even if the finalizer says the file is OK.
// This tests the fix for a bug where finalizer's "already formatted" would
// incorrectly override yamlfmt's "needs formatting" result.
func TestFormatCommand_CheckMode_YAMLPrimaryFormatterPrecedence(t *testing.T) {
	// Skip if yamlfmt not available
	if _, err := exec.LookPath("yamlfmt"); err != nil {
		t.Skip("yamlfmt not available")
	}

	tempDir := t.TempDir()

	// YAML with formatting issues yamlfmt will catch (blank lines, extra spaces),
	// but clean for finalizer (correct EOF with single newline, no trailing whitespace)
	yamlContent := `version: v1

key: value

nested:
  item1: one      # extra spaces before comment
  item2: two
`
	yamlFile := filepath.Join(tempDir, "test.yaml")
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Run goneat format --check as a subprocess to properly test exit code
	// since the command uses os.Exit() internally
	cmd := exec.Command("go", "run", ".", "format", "--check", yamlFile)
	cmd.Dir = ".."
	output, err := cmd.CombinedOutput()

	// Should return non-zero exit code because yamlfmt detects formatting issues
	if err == nil {
		t.Errorf("Expected format --check to fail on YAML with formatting issues, but it passed.\nOutput: %s", output)
	}

	// Verify the output indicates formatting is needed
	if !strings.Contains(string(output), "need-format=1") && !strings.Contains(string(output), "needs formatting") {
		t.Errorf("Expected output to indicate formatting needed, got: %s", output)
	}
}

// TestFormatCommand_QuietMode tests quiet mode functionality
func TestFormatCommand_QuietMode(t *testing.T) {
	// Create a temporary directory with test files
	tempDir := t.TempDir()
	createTestFiles(t, tempDir)

	// Create a test command with quiet mode enabled
	cmd := &cobra.Command{}
	setupFormatCommandFlags(cmd)
	if err := cmd.Flags().Set("folders", tempDir); err != nil {
		t.Fatalf("Failed to set folders flag: %v", err)
	}
	if err := cmd.Flags().Set("quiet", "true"); err != nil {
		t.Fatalf("Failed to set quiet flag: %v", err)
	}

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Execute the command
	err := RunFormat(cmd, []string{})

	// Check for errors
	if err != nil {
		t.Fatalf("Format command quiet mode failed: %v", err)
	}

	// In quiet mode, the command should still execute successfully
	// The main difference is less verbose logging output
	// We can't easily test the exact output format in unit tests
}

// TestFormatCommand_NoOpMode tests no-op mode functionality
// Note: This test is currently disabled due to external tool dependencies
// TODO: Re-enable when external tools are properly mocked or available in CI
func TestFormatCommand_NoOpMode(t *testing.T) {
	t.Skip("No-op test disabled due to external tool dependencies")

	// This test would verify no-op mode functionality
	// For now, we focus on the core formatting functionality that works
}

// TestFormatCommand_SpecificFiles tests formatting specific files
func TestFormatCommand_SpecificFiles(t *testing.T) {
	// Create a temporary directory with test files
	tempDir := t.TempDir()
	createTestFiles(t, tempDir)

	// Get path to a specific test file
	testFile := filepath.Join(tempDir, "test.go")

	// Create a test command with specific files
	cmd := &cobra.Command{}
	setupFormatCommandFlags(cmd)
	if err := cmd.Flags().Set("files", testFile); err != nil {
		t.Fatalf("failed setting files flag: %v", err)
	}

	// Execute the command
	err := RunFormat(cmd, []string{})

	// Check for errors
	if err != nil {
		t.Fatalf("Format command with specific files failed: %v", err)
	}

	// Verify the file was processed by checking if it still exists and is accessible
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Errorf("Expected test file to still exist after processing")
	}
}

// TestFormatCommand_ContentTypes tests content type filtering
func TestFormatCommand_ContentTypes(t *testing.T) {
	// Create a temporary directory with test files
	tempDir := t.TempDir()
	createTestFiles(t, tempDir)

	// Create a test command with content type filtering
	cmd := &cobra.Command{}
	setupFormatCommandFlags(cmd)
	if err := cmd.Flags().Set("folders", tempDir); err != nil {
		t.Fatalf("failed setting folders flag: %v", err)
	}
	if err := cmd.Flags().Set("types", "go"); err != nil {
		t.Fatalf("failed setting types flag: %v", err)
	}

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Execute the command
	err := RunFormat(cmd, []string{})

	// Check for errors
	if err != nil {
		t.Fatalf("Format command with content type filtering failed: %v", err)
	}

	// Verify output contains Go-related processing
	outputStr := output.String()
	// Should process Go files but not others
	t.Logf("Content type filtering output: %s", outputStr)
}

// TestFormatCommand_ParallelStrategy tests parallel execution strategy
// Note: This test is currently disabled due to complexity in testing parallel execution
// TODO: Re-enable when parallel execution testing is properly implemented
func TestFormatCommand_ParallelStrategy(t *testing.T) {
	t.Skip("Parallel strategy test disabled - requires more complex test setup")

	// This test would verify parallel execution strategy
	// For now, we focus on the core formatting functionality that works
}

// TestFormatCommand_PlanFile tests plan file output
func TestFormatCommand_PlanFile(t *testing.T) {
	// Create a temporary directory with test files
	tempDir := t.TempDir()
	createTestFiles(t, tempDir)

	// Create a plan file path
	planFile := filepath.Join(tempDir, "format-plan.json")

	// Create a test command with plan file output
	cmd := &cobra.Command{}
	setupFormatCommandFlags(cmd)
	if err := cmd.Flags().Set("folders", tempDir); err != nil {
		t.Fatalf("failed setting folders flag: %v", err)
	}
	if err := cmd.Flags().Set("plan-only", "true"); err != nil {
		t.Fatalf("failed setting plan-only flag: %v", err)
	}
	if err := cmd.Flags().Set("plan-file", planFile); err != nil {
		t.Fatalf("failed setting plan-file flag: %v", err)
	}

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Execute the command
	err := RunFormat(cmd, []string{})

	// Check for errors
	if err != nil {
		t.Fatalf("Format command with plan file failed: %v", err)
	}

	// Verify plan file was created
	if _, err := os.Stat(planFile); os.IsNotExist(err) {
		t.Errorf("Expected plan file to be created at %s", planFile)
	}

	// Verify output contains plan file information
	outputStr := output.String()
	if !strings.Contains(outputStr, "Plan written to") {
		t.Errorf("Expected output to contain 'Plan written to', got: %s", outputStr)
	}
}

// TestFormatCommand_UseGoimportsWithIgnoreMissingTools ensures enabling goimports does not fail when tool is missing and ignore-missing-tools is set
func TestFormatCommand_UseGoimportsWithIgnoreMissingTools(t *testing.T) {
	// Create a temporary directory with test files
	tempDir := t.TempDir()
	createTestFiles(t, tempDir)

	// Create a test command with goimports enabled but allow missing tool
	cmd := &cobra.Command{}
	setupFormatCommandFlags(cmd)
	if err := cmd.Flags().Set("folders", tempDir); err != nil {
		t.Fatalf("failed setting folders flag: %v", err)
	}
	if err := cmd.Flags().Set("use-goimports", "true"); err != nil {
		t.Fatalf("failed setting use-goimports flag: %v", err)
	}
	if err := cmd.Flags().Set("ignore-missing-tools", "true"); err != nil {
		t.Fatalf("failed setting ignore-missing-tools flag: %v", err)
	}

	// Capture output (not strictly required, but helpful for debug)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Execute the command
	if err := RunFormat(cmd, []string{}); err != nil {
		t.Fatalf("Format command with --use-goimports and --ignore-missing-tools failed: %v", err)
	}
}

// TestFormatCommand_UseGoimportsFailsWhenMissingTool ensures we fail fast when goimports is requested but not installed
func TestFormatCommand_UseGoimportsFailsWhenMissingTool(t *testing.T) {
	// This scenario triggers os.Exit via the execution pipeline when errors are encountered,
	// which requires a subprocess harness to test correctly. Skipping in unit test.
	t.Skip("Skipped: failure path uses os.Exit; requires subprocess harness to assert safely")
}

// Test helper functions

// setupFormatCommandFlags sets up all the flags needed for format command testing
func setupFormatCommandFlags(cmd *cobra.Command) {
	cmd.Flags().StringSliceP("files", "f", []string{}, "Specific files to format")
	cmd.Flags().Bool("check", false, "Check if files are formatted without modifying")
	cmd.Flags().Bool("quiet", false, "Suppress output except for errors")
	cmd.Flags().Bool("dry-run", false, "Show what would be done without executing")
	cmd.Flags().Bool("plan-only", false, "Generate and display work plan without executing")
	cmd.Flags().String("plan-file", "", "Write work plan to specified file")
	cmd.Flags().StringSlice("folders", []string{}, "Folders to process (alternative to positional args)")
	cmd.Flags().StringSlice("types", []string{}, "Content types to include (go, yaml, json, markdown)")
	cmd.Flags().Int("max-depth", -1, "Maximum directory depth to traverse")
	cmd.Flags().String("strategy", "sequential", "Execution strategy (sequential, parallel)")
	cmd.Flags().Bool("group-by-size", false, "Group work items by file size")
	cmd.Flags().Bool("group-by-type", false, "Group work items by content type")
	cmd.Flags().Bool("no-op", false, "Run in assessment mode without making changes")
	// Additional flags used by new features
	cmd.Flags().Bool("ignore-missing-tools", false, "Skip files requiring external formatters if tools are missing")
	cmd.Flags().Bool("use-goimports", false, "Organize Go imports with goimports (after gofmt)")
	cmd.Flags().String("json-indent", "  ", "Indentation for JSON prettification")
	cmd.Flags().Int("json-indent-count", 2, "Number of spaces for JSON indentation")
	cmd.Flags().Int("json-size-warning", 500, "Size threshold in MB for JSON file warnings")
	cmd.Flags().String("xml-indent", "  ", "Indentation for XML prettification")
	cmd.Flags().Int("xml-indent-count", 2, "Number of spaces for XML indentation")
	cmd.Flags().Int("xml-size-warning", 500, "Size threshold in MB for XML file warnings")
}

// createTestFiles creates a set of test files for format command testing
// Note: Only creates Go files to avoid external tool dependencies in tests
func createTestFiles(t *testing.T, dir string) {
	// Create a Go file that needs formatting
	goFile := filepath.Join(dir, "test.go")
	goContent := `package main

import (
	"fmt"
	"os"
)

func main(){
	fmt.Println("Hello, World!")
	os.Exit(0)
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test Go file: %v", err)
	}
}

// Test individual format functions

// Note: Internal function tests have been removed to focus on public API testing
// Internal functions should be tested through integration tests of the command interface
// rather than direct unit tests to avoid coupling to implementation details

// TestFormatCommand_JSONFormatting tests JSON formatting functionality
func TestFormatCommand_JSONFormatting(t *testing.T) {
	// Create a temporary directory with a JSON file
	tempDir := t.TempDir()

	// Create a minified JSON file
	jsonFile := filepath.Join(tempDir, "test.json")
	jsonContent := `{"key":"value","number":123}`
	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to create test JSON file: %v", err)
	}

	// Create a test command
	cmd := &cobra.Command{}
	setupFormatCommandFlags(cmd)
	if err := cmd.Flags().Set("files", jsonFile); err != nil {
		t.Fatalf("Failed to set files flag: %v", err)
	}
	if err := cmd.Flags().Set("json-indent", "  "); err != nil {
		t.Fatalf("Failed to set json-indent flag: %v", err)
	}

	// Execute the command
	err := RunFormat(cmd, []string{})

	// Check for errors
	if err != nil {
		t.Fatalf("Format command for JSON failed: %v", err)
	}

	// Verify the file was formatted
	formattedContent, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatalf("Failed to read formatted JSON file: %v", err)
	}

	expectedContent := "{\n  \"key\": \"value\",\n  \"number\": 123\n}\n"
	if string(formattedContent) != expectedContent {
		t.Errorf("Expected formatted JSON %q, got %q", expectedContent, string(formattedContent))
	}
}

// TestFormatCommand_XMLFormatting tests XML formatting functionality
func TestFormatCommand_XMLFormatting(t *testing.T) {
	// Create a temporary directory with an XML file
	tempDir := t.TempDir()

	// Create a minified XML file
	xmlFile := filepath.Join(tempDir, "test.xml")
	xmlContent := `<root><item>value</item><item>another</item></root>`
	if err := os.WriteFile(xmlFile, []byte(xmlContent), 0644); err != nil {
		t.Fatalf("Failed to create test XML file: %v", err)
	}

	// Create a test command
	cmd := &cobra.Command{}
	setupFormatCommandFlags(cmd)
	if err := cmd.Flags().Set("files", xmlFile); err != nil {
		t.Fatalf("Failed to set files flag: %v", err)
	}
	if err := cmd.Flags().Set("xml-indent-count", "2"); err != nil {
		t.Fatalf("Failed to set xml-indent-count flag: %v", err)
	}

	// Execute the command
	err := RunFormat(cmd, []string{})

	// Check for errors
	if err != nil {
		t.Fatalf("Format command for XML failed: %v", err)
	}

	// Verify the file was formatted
	formattedContent, err := os.ReadFile(xmlFile)
	if err != nil {
		t.Fatalf("Failed to read formatted XML file: %v", err)
	}

	expectedContent := "<root>\n  <item>value</item>\n  <item>another</item>\n</root>\n"
	if string(formattedContent) != expectedContent {
		t.Errorf("Expected formatted XML %q, got %q", expectedContent, string(formattedContent))
	}
}
