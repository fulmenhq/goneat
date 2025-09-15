package integration

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestDoctorFoundationWorkflow tests the complete foundation tools workflow
func TestDoctorFoundationWorkflow(t *testing.T) {
	// Skip if binary doesn't exist
	if _, err := os.Stat("./dist/goneat"); os.IsNotExist(err) {
		t.Skip("goneat binary not found, skipping integration test")
	}

	// Test the basic foundation scope check
	cmd := exec.Command("./dist/goneat", "doctor", "tools", "--scope", "foundation")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Should not fail with unknown scope
	if err != nil && strings.Contains(outputStr, "unknown scope") {
		t.Fatalf("Foundation scope should be recognized: %s", outputStr)
	}

	// Should mention foundation tools (basic validation, don't require tools to be installed)
	expectedTools := []string{"ripgrep", "jq", "go-licenses"}
	for _, tool := range expectedTools {
		if !strings.Contains(outputStr, tool) {
			t.Errorf("Output should mention foundation tool %s", tool)
		}
	}
}

// TestDoctorFoundationWorkflow_JSON tests JSON output workflow
func TestDoctorFoundationWorkflow_JSON(t *testing.T) {
	// Skip if binary doesn't exist
	if _, err := os.Stat("./dist/goneat"); os.IsNotExist(err) {
		t.Skip("goneat binary not found, skipping integration test")
	}

	cmd := exec.Command("./dist/goneat", "doctor", "tools", "--scope", "foundation", "--json")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		t.Fatalf("JSON foundation check should not fail: %s", outputStr)
	}

	// Should be valid JSON
	if !strings.Contains(outputStr, "{") {
		t.Errorf("Output should be JSON: %s", outputStr)
	}

	// Should contain tools array
	if !strings.Contains(outputStr, "tools") {
		t.Errorf("JSON should contain tools: %s", outputStr)
	}
}

// TestDoctorFoundationWorkflow_DryRun tests dry-run workflow
func TestDoctorFoundationWorkflow_DryRun(t *testing.T) {
	// Skip if binary doesn't exist
	if _, err := os.Stat("./dist/goneat"); os.IsNotExist(err) {
		t.Skip("goneat binary not found, skipping integration test")
	}

	cmd := exec.Command("./dist/goneat", "doctor", "tools", "--scope", "foundation", "--dry-run")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		t.Fatalf("Dry-run should not fail: %s", outputStr)
	}

	// Should indicate dry-run mode
	if !strings.Contains(outputStr, "dry") && !strings.Contains(outputStr, "would") {
		t.Errorf("Dry-run should indicate preview mode: %s", outputStr)
	}
}

// TestDoctorWorkflow_ListScopes tests the list scopes workflow
func TestDoctorWorkflow_ListScopes(t *testing.T) {
	// Skip if binary doesn't exist
	if _, err := os.Stat("./dist/goneat"); os.IsNotExist(err) {
		t.Skip("goneat binary not found, skipping integration test")
	}

	cmd := exec.Command("./dist/goneat", "doctor", "tools", "--list-scopes")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		t.Fatalf("List scopes should not fail: %s", outputStr)
	}

	// Should contain foundation scope
	if !strings.Contains(outputStr, "foundation") {
		t.Errorf("Should list foundation scope: %s", outputStr)
	}

	// Should contain other expected scopes
	requiredScopes := []string{"security", "format", "all"}
	for _, scope := range requiredScopes {
		if !strings.Contains(outputStr, scope) {
			t.Errorf("Should list %s scope: %s", scope, outputStr)
		}
	}
}

// TestDoctorWorkflow_CrossPlatformTools tests individual foundation tools
func TestDoctorWorkflow_CrossPlatformTools(t *testing.T) {
	// Skip if binary doesn't exist
	if _, err := os.Stat("./dist/goneat"); os.IsNotExist(err) {
		t.Skip("goneat binary not found, skipping integration test")
	}

	foundationTools := []string{"ripgrep", "jq", "go-licenses"}

	for _, tool := range foundationTools {
		t.Run(tool, func(t *testing.T) {
			cmd := exec.Command("./dist/goneat", "doctor", "tools", "--tools", tool)
			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			// Should not fail due to unknown tool
			if err != nil && strings.Contains(outputStr, "unknown tool(s)") {
				t.Fatalf("Tool %s should be recognized: %s", tool, outputStr)
			}

			// Should mention the tool
			if !strings.Contains(outputStr, tool) {
				t.Errorf("Should mention tool %s: %s", tool, outputStr)
			}
		})
	}
}

// TestDoctorWorkflow_ErrorHandling tests error handling scenarios
func TestDoctorWorkflow_ErrorHandling(t *testing.T) {
	// Skip if binary doesn't exist
	if _, err := os.Stat("./dist/goneat"); os.IsNotExist(err) {
		t.Skip("goneat binary not found, skipping integration test")
	}

	// Test unknown scope
	cmd := exec.Command("./dist/goneat", "doctor", "tools", "--scope", "nonexistent")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Should fail gracefully with unknown scope
	if err == nil {
		t.Error("Unknown scope should cause error")
	}

	if !strings.Contains(outputStr, "nonexistent") {
		t.Errorf("Error should mention unknown scope: %s", outputStr)
	}
}

// TestDoctorWorkflow_InvalidTool tests invalid tool handling
func TestDoctorWorkflow_InvalidTool(t *testing.T) {
	// Skip if binary doesn't exist
	if _, err := os.Stat("./dist/goneat"); os.IsNotExist(err) {
		t.Skip("goneat binary not found, skipping integration test")
	}

	// Test unknown tool
	cmd := exec.Command("./dist/goneat", "doctor", "tools", "--tools", "nonexistent-tool")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Should fail gracefully with unknown tool
	if err == nil {
		t.Error("Unknown tool should cause error")
	}

	if !strings.Contains(outputStr, "unknown tool") {
		t.Errorf("Error should mention unknown tool: %s", outputStr)
	}
}
