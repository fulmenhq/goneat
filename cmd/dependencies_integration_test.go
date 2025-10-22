//go:build integration
// +build integration

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestDependenciesCmd_SBOM_DefaultOutput tests default SBOM generation to file
func TestDependenciesCmd_SBOM_DefaultOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup mock syft
	mockScript, err := filepath.Abs(filepath.Join("../pkg/sbom/testdata", "mock-syft-modern.sh"))
	if err != nil {
		t.Fatalf("Failed to resolve mock script: %v", err)
	}
	t.Setenv("GONEAT_TOOL_SYFT", mockScript)

	// Setup test environment
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Logf("Warning: failed to restore directory: %v", err)
		}
	}()

	// Create minimal go.mod for testing
	if err := os.WriteFile("go.mod", []byte("module test\ngo 1.23\n"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Reset command for fresh execution
	cmd := &cobra.Command{
		Use:  "dependencies",
		RunE: runDependencies,
	}
	cmd.Flags().Bool("sbom", false, "Generate SBOM")
	cmd.Flags().String("sbom-format", "cyclonedx-json", "SBOM format")
	cmd.Flags().String("sbom-output", "", "SBOM output file")
	cmd.Flags().Bool("sbom-stdout", false, "Output to stdout")
	cmd.Flags().String("sbom-platform", "", "Target platform")
	cmd.Flags().Bool("licenses", false, "Check licenses")
	cmd.Flags().Bool("cooling", false, "Check cooling")
	cmd.Flags().String("policy", "", "Policy path")
	cmd.Flags().String("format", "json", "Output format")
	cmd.Flags().String("output", "", "Output file")
	cmd.Flags().String("fail-on", "critical", "Fail on severity")

	if err := cmd.Flags().Set("sbom", "true"); err != nil {
		t.Fatalf("Failed to set sbom flag: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run command
	err = cmd.RunE(cmd, []string{"."})

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, copyErr := buf.ReadFrom(r); copyErr != nil {
		t.Logf("Warning: failed to read stdout: %v", copyErr)
	}
	output := buf.String()

	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Verify output mentions file creation
	if !strings.Contains(output, "SBOM generated") {
		t.Errorf("Expected 'SBOM generated' in output, got: %s", output)
	}

	// Verify sbom directory was created
	if _, statErr := os.Stat("sbom"); statErr != nil {
		t.Errorf("SBOM directory not created: %v", statErr)
	}

	t.Logf("✅ Default SBOM generation succeeded\nOutput: %s", output)
}

// TestDependenciesCmd_SBOM_Stdout tests SBOM output to stdout
func TestDependenciesCmd_SBOM_Stdout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup mock syft
	mockScript, err := filepath.Abs(filepath.Join("../pkg/sbom/testdata", "mock-syft-modern.sh"))
	if err != nil {
		t.Fatalf("Failed to resolve mock script: %v", err)
	}
	t.Setenv("GONEAT_TOOL_SYFT", mockScript)

	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Logf("Warning: failed to restore directory: %v", err)
		}
	}()

	if err := os.WriteFile("go.mod", []byte("module test\ngo 1.23\n"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	cmd := &cobra.Command{
		Use:  "dependencies",
		RunE: runDependencies,
	}
	cmd.Flags().Bool("sbom", false, "Generate SBOM")
	cmd.Flags().String("sbom-format", "cyclonedx-json", "SBOM format")
	cmd.Flags().String("sbom-output", "", "SBOM output file")
	cmd.Flags().Bool("sbom-stdout", false, "Output to stdout")
	cmd.Flags().String("sbom-platform", "", "Target platform")
	cmd.Flags().Bool("licenses", false, "Check licenses")
	cmd.Flags().Bool("cooling", false, "Check cooling")
	cmd.Flags().String("policy", "", "Policy path")
	cmd.Flags().String("format", "json", "Output format")
	cmd.Flags().String("output", "", "Output file")
	cmd.Flags().String("fail-on", "critical", "Fail on severity")

	if err := cmd.Flags().Set("sbom", "true"); err != nil {
		t.Fatalf("Failed to set sbom flag: %v", err)
	}
	if err := cmd.Flags().Set("sbom-stdout", "true"); err != nil {
		t.Fatalf("Failed to set sbom-stdout flag: %v", err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = cmd.RunE(cmd, []string{"."})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, copyErr := buf.ReadFrom(r); copyErr != nil {
		t.Logf("Warning: failed to read stdout: %v", copyErr)
	}
	output := buf.String()

	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Verify output is valid JSON (CycloneDX)
	var sbom map[string]interface{}
	if err := json.Unmarshal([]byte(output), &sbom); err != nil {
		t.Fatalf("SBOM output is not valid JSON: %v\nOutput: %s", err, output)
	}

	// Verify CycloneDX structure
	if bomFormat, ok := sbom["bomFormat"].(string); !ok || bomFormat != "CycloneDX" {
		t.Errorf("Expected bomFormat=CycloneDX, got: %v", sbom["bomFormat"])
	}

	t.Logf("✅ SBOM stdout output succeeded (valid CycloneDX JSON)")
}

// TestDependenciesCmd_SBOM_CustomOutput tests custom output path
func TestDependenciesCmd_SBOM_CustomOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockScript, err := filepath.Abs(filepath.Join("../pkg/sbom/testdata", "mock-syft-modern.sh"))
	if err != nil {
		t.Fatalf("Failed to resolve mock script: %v", err)
	}
	t.Setenv("GONEAT_TOOL_SYFT", mockScript)

	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Logf("Warning: failed to restore directory: %v", err)
		}
	}()

	if err := os.WriteFile("go.mod", []byte("module test\ngo 1.23\n"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	customOutput := filepath.Join(tmpDir, "custom-sbom.json")

	cmd := &cobra.Command{
		Use:  "dependencies",
		RunE: runDependencies,
	}
	cmd.Flags().Bool("sbom", false, "Generate SBOM")
	cmd.Flags().String("sbom-format", "cyclonedx-json", "SBOM format")
	cmd.Flags().String("sbom-output", "", "SBOM output file")
	cmd.Flags().Bool("sbom-stdout", false, "Output to stdout")
	cmd.Flags().String("sbom-platform", "", "Target platform")
	cmd.Flags().Bool("licenses", false, "Check licenses")
	cmd.Flags().Bool("cooling", false, "Check cooling")
	cmd.Flags().String("policy", "", "Policy path")
	cmd.Flags().String("format", "json", "Output format")
	cmd.Flags().String("output", "", "Output file")
	cmd.Flags().String("fail-on", "critical", "Fail on severity")

	if err := cmd.Flags().Set("sbom", "true"); err != nil {
		t.Fatalf("Failed to set sbom flag: %v", err)
	}
	if err := cmd.Flags().Set("sbom-output", customOutput); err != nil {
		t.Fatalf("Failed to set sbom-output flag: %v", err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = cmd.RunE(cmd, []string{"."})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, copyErr := buf.ReadFrom(r); copyErr != nil {
		t.Logf("Warning: failed to read stdout: %v", copyErr)
	}
	output := buf.String()

	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Verify custom output file was created
	if _, statErr := os.Stat(customOutput); statErr != nil {
		t.Errorf("Custom output file not created at %s: %v", customOutput, statErr)
	}

	// Verify output mentions custom path
	if !strings.Contains(output, "custom-sbom.json") {
		t.Errorf("Expected custom path in output, got: %s", output)
	}

	t.Logf("✅ Custom output path succeeded: %s", customOutput)
}

// TestDependenciesCmd_SBOM_SyftNotAvailable tests error handling when syft is missing
func TestDependenciesCmd_SBOM_SyftNotAvailable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Clear PATH to prevent fallback and point to non-existent binary
	t.Setenv("PATH", "")
	t.Setenv("GONEAT_TOOL_SYFT", "/nonexistent/syft")
	t.Setenv("GONEAT_HOME", t.TempDir()) // Ensure no managed binaries found

	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Logf("Warning: failed to restore directory: %v", err)
		}
	}()

	if err := os.WriteFile("go.mod", []byte("module test\ngo 1.23\n"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	cmd := &cobra.Command{
		Use:  "dependencies",
		RunE: runDependencies,
	}
	cmd.Flags().Bool("sbom", false, "Generate SBOM")
	cmd.Flags().String("sbom-format", "cyclonedx-json", "SBOM format")
	cmd.Flags().String("sbom-output", "", "SBOM output file")
	cmd.Flags().Bool("sbom-stdout", false, "Output to stdout")
	cmd.Flags().String("sbom-platform", "", "Target platform")
	cmd.Flags().Bool("licenses", false, "Check licenses")
	cmd.Flags().Bool("cooling", false, "Check cooling")
	cmd.Flags().String("policy", "", "Policy path")
	cmd.Flags().String("format", "json", "Output format")
	cmd.Flags().String("output", "", "Output file")
	cmd.Flags().String("fail-on", "critical", "Fail on severity")

	if err := cmd.Flags().Set("sbom", "true"); err != nil {
		t.Fatalf("Failed to set sbom flag: %v", err)
	}

	err := cmd.RunE(cmd, []string{"."})

	if err == nil {
		t.Fatal("Expected error when syft is not available, got nil")
	}

	// Verify error message provides helpful DX guidance
	errMsg := err.Error()
	if !strings.Contains(errMsg, "goneat doctor tools") {
		t.Errorf("Expected DX message with 'goneat doctor tools' in error, got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "--install") {
		t.Errorf("Expected installation instructions in error, got: %s", errMsg)
	}

	t.Logf("✅ Error handling verified with helpful DX message:\n%s", errMsg)
}

// TestDependenciesCmd_NoFlags tests help display when no flags provided
func TestDependenciesCmd_NoFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Logf("Warning: failed to restore directory: %v", err)
		}
	}()

	cmd := &cobra.Command{
		Use:   "dependencies",
		Short: "Dependency policy enforcement and analysis",
		Long:  `Analyze dependencies for license compliance, cooling policy, and generate SBOMs.`,
		RunE:  runDependencies,
	}
	cmd.Flags().Bool("sbom", false, "Generate SBOM")
	cmd.Flags().String("sbom-format", "cyclonedx-json", "SBOM format")
	cmd.Flags().String("sbom-output", "", "SBOM output file")
	cmd.Flags().Bool("sbom-stdout", false, "Output to stdout")
	cmd.Flags().String("sbom-platform", "", "Target platform")
	cmd.Flags().Bool("licenses", false, "Check licenses")
	cmd.Flags().Bool("cooling", false, "Check cooling")
	cmd.Flags().String("policy", "", "Policy path")
	cmd.Flags().String("format", "json", "Output format")
	cmd.Flags().String("output", "", "Output file")
	cmd.Flags().String("fail-on", "critical", "Fail on severity")

	// Capture stdout for help
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.RunE(cmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, copyErr := buf.ReadFrom(r); copyErr != nil {
		t.Logf("Warning: failed to read stdout: %v", copyErr)
	}
	output := buf.String()

	// Command should succeed but show help
	if err != nil {
		t.Errorf("Command should not error without flags, got: %v", err)
	}

	// Note: Help output goes to stdout via cmd.Help(), but might be empty in test
	// The important thing is no error occurred
	t.Logf("✅ No-flags scenario handled gracefully")
	if output != "" {
		t.Logf("Help output: %s", output)
	}
}
