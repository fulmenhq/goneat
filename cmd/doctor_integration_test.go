package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createTestConfig creates a temporary tools configuration for testing
func createTestConfig(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tools.yaml")

	configContent := `
scopes:
  foundation:
    description: "Foundation tools"
    tools: ["ripgrep", "jq", "go-licenses"]
tools:
  ripgrep:
    name: "ripgrep"
    description: "Fast text search tool"
    kind: "system"
    detect_command: "rg --version"
  jq:
    name: "jq"
    description: "JSON processor"
    kind: "system"
    detect_command: "jq --version"
  go-licenses:
    name: "go-licenses"
    description: "License compliance tool"
    kind: "go"
    detect_command: "go-licenses -h"
    install_package: "github.com/google/go-licenses@latest"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}
	return configPath
}

// TestDoctorTools_FoundationScope tests the foundation scope
func TestDoctorTools_FoundationScope(t *testing.T) {
	configPath := createTestConfig(t)
	_, err := execDoctorTools(t, []string{"--scope", "foundation", "--config", configPath})
	// Should not fail due to unknown scope
	if err != nil && strings.Contains(err.Error(), "unknown scope") {
		t.Fatalf("Foundation scope should be recognized: %v", err)
	}
}

// TestDoctorTools_FoundationScope_JSON tests JSON output for foundation scope
func TestDoctorTools_FoundationScope_JSON(t *testing.T) {
	configPath := createTestConfig(t)
	_, err := execDoctorTools(t, []string{"--scope", "foundation", "--json", "--config", configPath})
	// Should not fail due to unknown scope
	if err != nil && strings.Contains(err.Error(), "unknown scope") {
		t.Fatalf("Foundation scope should be recognized with JSON output: %v", err)
	}
}

// TestDoctorTools_FoundationScope_DryRun tests dry-run functionality
func TestDoctorTools_FoundationScope_DryRun(t *testing.T) {
	configPath := createTestConfig(t)
	_, err := execDoctorTools(t, []string{"--scope", "foundation", "--dry-run", "--config", configPath})
	if err != nil {
		t.Fatalf("Dry-run should not fail: %v", err)
	}
}

// TestDoctorTools_FoundationScope_PrintInstructions tests print-instructions
func TestDoctorTools_FoundationScope_PrintInstructions(t *testing.T) {
	configPath := createTestConfig(t)
	_, err := execDoctorTools(t, []string{"--scope", "foundation", "--print-instructions", "--config", configPath})
	// Should not fail due to unknown scope
	if err != nil && strings.Contains(err.Error(), "unknown scope") {
		t.Fatalf("Foundation scope should be recognized: %v", err)
	}
}

// TestDoctorTools_ListScopes tests the list-scopes functionality
func TestDoctorTools_ListScopes(t *testing.T) {
	configPath := createTestConfig(t)
	_, err := execDoctorTools(t, []string{"--list-scopes", "--config", configPath})
	if err != nil {
		t.Fatalf("List scopes should not fail: %v", err)
	}
}

// TestDoctorTools_ListScopes_JSON tests JSON output for list-scopes
func TestDoctorTools_ListScopes_JSON(t *testing.T) {
	configPath := createTestConfig(t)
	_, err := execDoctorTools(t, []string{"--list-scopes", "--json", "--config", configPath})
	if err != nil {
		t.Fatalf("List scopes JSON should not fail: %v", err)
	}
}

// TestDoctorTools_ValidateConfig tests configuration validation
func TestDoctorTools_ValidateConfig(t *testing.T) {
	configPath := createTestConfig(t)
	_, err := execDoctorTools(t, []string{"--validate-config", "--config", configPath})
	if err != nil {
		t.Fatalf("Validate config should not fail: %v", err)
	}
}

// TestDoctorTools_FoundationTools_Individual tests individual foundation tools
func TestDoctorTools_FoundationTools_Individual(t *testing.T) {
	configPath := createTestConfig(t)
	foundationTools := []string{"ripgrep", "jq", "go-licenses"}

	for _, tool := range foundationTools {
		t.Run(tool, func(t *testing.T) {
			_, err := execDoctorTools(t, []string{"--tools", tool, "--config", configPath})
			// Should not fail due to unknown tool
			if err != nil && strings.Contains(err.Error(), "unknown tool(s)") {
				t.Fatalf("Tool %s should be recognized: %v", tool, err)
			}
		})
	}
}
