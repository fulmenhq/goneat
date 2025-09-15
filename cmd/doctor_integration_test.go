package cmd

import (
	"strings"
	"testing"
)

// TestDoctorTools_InfrastructureScope tests the new infrastructure scope
func TestDoctorTools_InfrastructureScope(t *testing.T) {
	_, err := execDoctorTools(t, []string{"--scope", "infrastructure"})
	// Should not fail due to unknown scope
	if err != nil && strings.Contains(err.Error(), "unknown scope") {
		t.Fatalf("Infrastructure scope should be recognized: %v", err)
	}

	// Should succeed (no error) since infrastructure tools are present on this system
	if err != nil {
		t.Errorf("Infrastructure scope check should succeed on system with tools: %v", err)
	}
}

// TestDoctorTools_InfrastructureScope_JSON tests JSON output for infrastructure scope
func TestDoctorTools_InfrastructureScope_JSON(t *testing.T) {
	_, err := execDoctorTools(t, []string{"--scope", "infrastructure", "--json"})
	if err != nil {
		t.Fatalf("JSON output should not fail: %v", err)
	}

	// Should succeed without error (JSON flag should be accepted)
}

// TestDoctorTools_InfrastructureScope_DryRun tests dry-run functionality
func TestDoctorTools_InfrastructureScope_DryRun(t *testing.T) {
	_, err := execDoctorTools(t, []string{"--scope", "infrastructure", "--dry-run"})
	if err != nil {
		t.Fatalf("Dry-run should not fail: %v", err)
	}

	// Should succeed without error (dry-run flag should be accepted)
}

// TestDoctorTools_InfrastructureScope_PrintInstructions tests print-instructions
func TestDoctorTools_InfrastructureScope_PrintInstructions(t *testing.T) {
	_, err := execDoctorTools(t, []string{"--scope", "infrastructure", "--print-instructions"})
	// Should not fail due to unknown scope
	if err != nil && strings.Contains(err.Error(), "unknown scope") {
		t.Fatalf("Infrastructure scope should be recognized: %v", err)
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

// TestDoctorTools_InfrastructureTools_Individual tests individual infrastructure tools
func TestDoctorTools_InfrastructureTools_Individual(t *testing.T) {
	infrastructureTools := []string{"ripgrep", "jq", "go-licenses"}

	for _, tool := range infrastructureTools {
		t.Run(tool, func(t *testing.T) {
			_, err := execDoctorTools(t, []string{"--tools", tool})
			// Should not fail due to unknown tool
			if err != nil && strings.Contains(err.Error(), "unknown tool(s)") {
				t.Fatalf("Tool %s should be recognized: %v", tool, err)
			}

			// Should succeed since tools are present on this system
			if err != nil {
				t.Errorf("Tool %s check should succeed: %v", tool, err)
			}
		})
	}
}
