//go:build integration
// +build integration

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestHooksLicenseEnforcement_PreCommitBlocksViolation tests that pre-commit hooks
// properly block commits containing license violations
func TestHooksLicenseEnforcement_PreCommitBlocksViolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test environment with synthetic project
	tmpDir := t.TempDir()
	fixturePath := filepath.Join("..", "tests", "fixtures", "dependencies", "synthetic-go-project")

	// Copy fixture to temp directory
	copyFixtureToTemp(t, fixturePath, tmpDir)
	projectPath := tmpDir

	// Create a policy that forbids MIT licenses (which testify and yaml.v3 use)
	policyDir := filepath.Join(projectPath, ".goneat")
	if err := os.MkdirAll(policyDir, 0755); err != nil {
		t.Fatalf("Failed to create .goneat directory: %v", err)
	}

	policyContent := `version: v1
licenses:
  forbidden:
    - MIT  # This will cause testify and yaml.v3 to fail
  allowed:
    - BSD-3-Clause
cooling:
  enabled: false
`
	if err := os.WriteFile(filepath.Join(policyDir, "dependencies.yaml"), []byte(policyContent), 0644); err != nil {
		t.Fatalf("Failed to create policy file: %v", err)
	}

	// Change to project directory
	originalWd, _ := os.Getwd()
	if err := os.Chdir(projectPath); err != nil {
		t.Fatalf("Failed to change to project directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Logf("Warning: failed to restore directory: %v", err)
		}
	}()

	// Simulate pre-commit hook execution with license checking
	err := simulateHookExecution(t, "pre-commit", projectPath)

	// Should fail due to MIT license violation
	if err == nil {
		t.Fatal("Expected hook to fail due to license violation, but it succeeded")
	}

	// Verify error indicates analysis failure due to violations
	errMsg := err.Error()
	if !strings.Contains(errMsg, "analysis failed") {
		t.Errorf("Expected error to indicate analysis failure, got: %s", errMsg)
	}

	t.Logf("✅ Pre-commit hook correctly blocked license violation: %s", errMsg)
}

// TestHooksLicenseEnforcement_PrePushBlocksViolation tests that pre-push hooks
// properly block pushes containing license violations
func TestHooksLicenseEnforcement_PrePushBlocksViolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test environment with synthetic project
	tmpDir := t.TempDir()
	fixturePath := filepath.Join("..", "tests", "fixtures", "dependencies", "synthetic-go-project")

	// Copy fixture to temp directory
	copyFixtureToTemp(t, fixturePath, tmpDir)
	projectPath := tmpDir

	// Create a policy that forbids MIT licenses (which testify and yaml.v3 use)
	policyDir := filepath.Join(projectPath, ".goneat")
	if err := os.MkdirAll(policyDir, 0755); err != nil {
		t.Fatalf("Failed to create .goneat directory: %v", err)
	}

	policyContent := `version: v1
licenses:
  forbidden:
    - MIT  # This will cause testify and yaml.v3 to fail
  allowed:
    - BSD-3-Clause
cooling:
  enabled: false
`
	if err := os.WriteFile(filepath.Join(policyDir, "dependencies.yaml"), []byte(policyContent), 0644); err != nil {
		t.Fatalf("Failed to create policy file: %v", err)
	}

	// Change to project directory
	originalWd, _ := os.Getwd()
	if err := os.Chdir(projectPath); err != nil {
		t.Fatalf("Failed to change to project directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Logf("Warning: failed to restore directory: %v", err)
		}
	}()

	// Simulate pre-push hook execution with license checking
	err := simulateHookExecution(t, "pre-push", projectPath)

	// Should fail due to MIT license violation
	if err == nil {
		t.Fatal("Expected hook to fail due to license violation, but it succeeded")
	}

	// Verify error indicates analysis failure due to violations
	errMsg := err.Error()
	if !strings.Contains(errMsg, "analysis failed") {
		t.Errorf("Expected error to indicate analysis failure, got: %s", errMsg)
	}

	t.Logf("✅ Pre-push hook correctly blocked license violation: %s", errMsg)
}

// TestHooksLicenseEnforcement_AllowedLicensesPass tests that hooks allow
// projects with compliant licenses
func TestHooksLicenseEnforcement_AllowedLicensesPass(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test environment with a compliant project
	tmpDir := t.TempDir()
	fixturePath := filepath.Join("..", "tests", "fixtures", "dependencies", "synthetic-go-project")

	// Copy compliant fixture to temp directory
	copyFixtureToTemp(t, fixturePath, tmpDir)
	projectPath := tmpDir

	// Create a policy that allows the licenses in the synthetic project
	policyDir := filepath.Join(projectPath, ".goneat")
	if err := os.MkdirAll(policyDir, 0755); err != nil {
		t.Fatalf("Failed to create .goneat directory: %v", err)
	}

	policyContent := `version: v1
licenses:
  allowed:
    - MIT
    - Apache-2.0
    - BSD-3-Clause
    - ISC
cooling:
  enabled: false
`
	if err := os.WriteFile(filepath.Join(policyDir, "dependencies.yaml"), []byte(policyContent), 0644); err != nil {
		t.Fatalf("Failed to create policy file: %v", err)
	}

	// Change to project directory
	originalWd, _ := os.Getwd()
	if err := os.Chdir(projectPath); err != nil {
		t.Fatalf("Failed to change to project directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Logf("Warning: failed to restore directory: %v", err)
		}
	}()

	// Simulate pre-commit hook execution
	err := simulateHookExecution(t, "pre-commit", projectPath)

	// Should succeed with compliant licenses
	if err != nil {
		t.Fatalf("Expected hook to succeed with compliant licenses, but failed: %v", err)
	}

	t.Logf("✅ Hook correctly allowed project with compliant licenses")
}

// simulateHookExecution simulates hook execution by running goneat dependencies with license checking
func simulateHookExecution(t *testing.T, hookType, projectPath string) error {
	t.Helper()

	// Create cobra command for dependencies
	cmd := &cobra.Command{
		Use:   "dependencies",
		Short: "Dependency policy enforcement and analysis",
		Long:  `Analyze dependencies for license compliance, cooling policy, and generate SBOMs.`,
		RunE:  runDependencies,
	}

	// Set up flags
	cmd.Flags().Bool("licenses", false, "Check licenses")
	cmd.Flags().Bool("cooling", false, "Check cooling")
	cmd.Flags().String("policy", "", "Policy path")
	cmd.Flags().String("format", "json", "Output format")
	cmd.Flags().String("output", "", "Output file")
	cmd.Flags().String("fail-on", "critical", "Fail on severity")

	// Set license checking
	if err := cmd.Flags().Set("licenses", "true"); err != nil {
		t.Fatalf("Failed to set licenses flag: %v", err)
	}

	// Set policy path to the project's .goneat/dependencies.yaml
	policyPath := filepath.Join(projectPath, ".goneat", "dependencies.yaml")
	if err := cmd.Flags().Set("policy", policyPath); err != nil {
		t.Fatalf("Failed to set policy flag: %v", err)
	}

	// Set fail-on high for hook-like behavior
	if err := cmd.Flags().Set("fail-on", "high"); err != nil {
		t.Fatalf("Failed to set fail-on flag: %v", err)
	}

	// Execute the dependencies check
	return cmd.RunE(cmd, []string{})
}

// copyFixtureToTemp copies a fixture directory to a temp location
func copyFixtureToTemp(t *testing.T, src, dst string) {
	t.Helper()

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip root directory
		if relPath == "." {
			return nil
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		// Copy file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(dstPath, data, 0644)
	})

	if err != nil {
		t.Fatalf("Failed to copy fixture from %s to %s: %v", src, dst, err)
	}
}
