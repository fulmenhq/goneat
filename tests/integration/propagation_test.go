/*
Copyright © 2025 3 Leaps <info@3leaps.com>
*/
package integration

import (
	"strings"
	"testing"
)

func TestPropagation_BasicPackageJson(t *testing.T) {
	// Test basic propagation to package.json
	env := CreateVersionFileFixture(t, "1.2.3")
	defer env.Cleanup()

	// Create package.json
	env.WriteFile("package.json", `{
  "name": "test-package",
  "version": "0.1.0"
}`)

	// Run propagation
	result := env.RunVersionCommand("propagate")
	if result.ExitCode != 0 {
		t.Fatalf("Propagation failed: %s", result.Error)
	}

	// Verify package.json was updated
	content := env.ReadFile("package.json")
	if !strings.Contains(content, `"version": "1.2.3"`) {
		t.Errorf("Expected package.json to contain version 1.2.3, got: %s", content)
	}
}

func TestPropagation_BasicPyprojectToml(t *testing.T) {
	// Test basic propagation to pyproject.toml
	env := CreateVersionFileFixture(t, "2.0.0")
	defer env.Cleanup()

	// Create pyproject.toml
	env.WriteFile("pyproject.toml", `[project]
name = "test-package"
version = "0.1.0"
`)

	// Run propagation
	result := env.RunVersionCommand("propagate")
	if result.ExitCode != 0 {
		t.Fatalf("Propagation failed: %s", result.Error)
	}

	// Verify pyproject.toml was updated
	content := env.ReadFile("pyproject.toml")
	if !strings.Contains(content, `version = "2.0.0"`) {
		t.Errorf("Expected pyproject.toml to contain version 2.0.0, got: %s", content)
	}
}

func TestPropagation_DryRun(t *testing.T) {
	// Test propagation dry-run mode
	env := CreateVersionFileFixture(t, "1.5.0")
	defer env.Cleanup()

	// Create package.json
	env.WriteFile("package.json", `{
  "name": "test-package",
  "version": "0.1.0"
}`)

	// Run propagation with dry-run
	result := env.RunVersionCommand("propagate", "--dry-run")
	if result.ExitCode != 0 {
		t.Fatalf("Propagation dry-run failed: %s", result.Error)
	}

	// Verify package.json was NOT updated
	content := env.ReadFile("package.json")
	if !strings.Contains(content, `"version": "0.1.0"`) {
		t.Errorf("Expected package.json to remain unchanged in dry-run, got: %s", content)
	}
}

func TestPropagation_ValidateOnly(t *testing.T) {
	// Test propagation validate-only mode
	env := CreateVersionFileFixture(t, "1.2.3")
	defer env.Cleanup()

	// Create package.json with matching version
	env.WriteFile("package.json", `{
  "name": "test-package",
  "version": "1.2.3"
}`)

	// Run propagation with validate-only
	result := env.RunVersionCommand("propagate", "--validate-only")
	if result.ExitCode != 0 {
		t.Fatalf("Propagation validate-only failed: %s", result.Error)
	}

	// Should succeed since versions match
	if !strings.Contains(result.Output, "Validation completed") {
		t.Errorf("Expected validation completion message, got: %s", result.Output)
	}
}

func TestPropagation_ValidateOnlyMismatch(t *testing.T) {
	// Test propagation validate-only with version mismatch
	env := CreateVersionFileFixture(t, "1.2.3")
	defer env.Cleanup()

	// Create package.json with different version
	env.WriteFile("package.json", `{
  "name": "test-package",
  "version": "0.1.0"
}`)

	// Run propagation with validate-only
	result := env.RunVersionCommand("propagate", "--validate-only")
	if result.ExitCode == 0 {
		t.Error("Expected propagation validate-only to fail with version mismatch")
	}

	// Check that the output contains the error details
	if !strings.Contains(result.Output, "version validation failed") {
		t.Errorf("Expected validation error in output, got: %s", result.Output)
	}
}
