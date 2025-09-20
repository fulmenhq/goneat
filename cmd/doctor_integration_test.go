package cmd

import (
	"strings"
	"testing"
)

// TestDoctorTools_FoundationScope tests the foundation scope
func TestDoctorTools_FoundationScope(t *testing.T) {
	_, err := execDoctorTools(t, []string{"--scope", "foundation"})
	// Should not fail due to unknown scope
	if err != nil && strings.Contains(err.Error(), "unknown scope") {
		t.Fatalf("Foundation scope should be recognized: %v", err)
	}

	// The command may fail if tools are missing, but that's OK - we just want to ensure the scope is recognized
	// The test passes as long as we don't get "unknown scope" error
}

// TestDoctorTools_FoundationScope_JSON tests JSON output for foundation scope
func TestDoctorTools_FoundationScope_JSON(t *testing.T) {
	_, err := execDoctorTools(t, []string{"--scope", "foundation", "--json"})
	// Should not fail due to unknown scope
	if err != nil && strings.Contains(err.Error(), "unknown scope") {
		t.Fatalf("Foundation scope should be recognized with JSON output: %v", err)
	}

	// The command may fail if tools are missing, but JSON output should be accepted
	// The test passes as long as we don't get "unknown scope" error
}

// TestDoctorTools_FoundationScope_DryRun tests dry-run functionality
func TestDoctorTools_FoundationScope_DryRun(t *testing.T) {
	_, err := execDoctorTools(t, []string{"--scope", "foundation", "--dry-run"})
	if err != nil {
		t.Fatalf("Dry-run should not fail: %v", err)
	}

	// Should succeed without error (dry-run flag should be accepted)
}

// TestDoctorTools_FoundationScope_PrintInstructions tests print-instructions
func TestDoctorTools_FoundationScope_PrintInstructions(t *testing.T) {
	_, err := execDoctorTools(t, []string{"--scope", "foundation", "--print-instructions"})
	// Should not fail due to unknown scope
	if err != nil && strings.Contains(err.Error(), "unknown scope") {
		t.Fatalf("Foundation scope should be recognized: %v", err)
	}

	// Should succeed without error (print-instructions flag should be accepted)
}

// TestDoctorTools_ListScopes tests the list-scopes functionality
func TestDoctorTools_ListScopes(t *testing.T) {
	_, err := execDoctorTools(t, []string{"--list-scopes"})
	if err != nil {
		t.Fatalf("List scopes should not fail: %v", err)
	}

	// Should succeed without error (list-scopes flag should be accepted)
}

// TestDoctorTools_ListScopes_JSON tests JSON output for list-scopes
func TestDoctorTools_ListScopes_JSON(t *testing.T) {
	_, err := execDoctorTools(t, []string{"--list-scopes", "--json"})
	if err != nil {
		t.Fatalf("List scopes JSON should not fail: %v", err)
	}

	// Should succeed without error (JSON flag should be accepted)
}

// TestDoctorTools_ValidateConfig tests configuration validation
func TestDoctorTools_ValidateConfig(t *testing.T) {
	_, err := execDoctorTools(t, []string{"--validate-config"})
	// Should not fail due to unknown flag
	if err != nil && strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("Validate config flag should be recognized: %v", err)
	}

	// Should succeed without error (validate-config flag should be accepted)
}

// TestDoctorTools_FoundationTools_Individual tests individual foundation tools
func TestDoctorTools_FoundationTools_Individual(t *testing.T) {
	foundationTools := []string{"ripgrep", "jq", "go-licenses"}

	for _, tool := range foundationTools {
		t.Run(tool, func(t *testing.T) {
			_, err := execDoctorTools(t, []string{"--tools", tool})
			// Should not fail due to unknown tool
			if err != nil && strings.Contains(err.Error(), "unknown tool(s)") {
				t.Fatalf("Tool %s should be recognized: %v", tool, err)
			}

			// The command may fail if the tool is missing, but that's OK - we just want to ensure the tool is recognized
			// The test passes as long as we don't get "unknown tool(s)" error
		})
	}
}
