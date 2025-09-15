package assess

import (
	"context"
	"testing"

	"github.com/fulmenhq/goneat/internal/doctor"
)

func TestToolsRunner_NewToolsRunner(t *testing.T) {
	runner := NewToolsRunner()
	if runner == nil {
		t.Fatal("NewToolsRunner should return a non-nil runner")
	}
}

func TestToolsRunner_GetCategory(t *testing.T) {
	runner := NewToolsRunner()
	if runner.GetCategory() != CategoryTools {
		t.Errorf("Expected category %s, got %s", CategoryTools, runner.GetCategory())
	}
}

func TestToolsRunner_CanRunInParallel(t *testing.T) {
	runner := NewToolsRunner()
	if !runner.CanRunInParallel() {
		t.Error("ToolsRunner should be able to run in parallel")
	}
}

func TestToolsRunner_IsAvailable(t *testing.T) {
	runner := NewToolsRunner()
	if !runner.IsAvailable() {
		t.Error("ToolsRunner should always be available")
	}
}

func TestToolsRunner_GetEstimatedTime(t *testing.T) {
	runner := NewToolsRunner()
	estimatedTime := runner.GetEstimatedTime(".")
	if estimatedTime <= 0 {
		t.Error("Estimated time should be positive")
	}
}

func TestToolsRunner_Assess_NoFoundationTools(t *testing.T) {
	// Mock the tools config to return no foundation tools
	runner := NewToolsRunner()

	// We can't easily mock the LoadToolsConfig function, so we'll test the basic structure
	// The actual assessment will be tested through integration tests

	result, err := runner.Assess(context.Background(), ".", AssessmentConfig{})
	if err != nil {
		t.Errorf("Assess should not return an error: %v", err)
	}

	if result.CommandName != "tools" {
		t.Errorf("Expected command name 'tools', got '%s'", result.CommandName)
	}

	if result.Category != CategoryTools {
		t.Errorf("Expected category %s, got %s", CategoryTools, result.Category)
	}
}

func TestToolsRunner_Assess_BasicFunctionality(t *testing.T) {
	// This test verifies that the assessment completes and returns metrics
	runner := NewToolsRunner()

	result, err := runner.Assess(context.Background(), ".", AssessmentConfig{})
	if err != nil {
		t.Errorf("Assess should not return an error: %v", err)
	}

	// Should have metrics about tools checked
	if result.Metrics == nil {
		t.Error("Result should contain metrics")
	}

	// Should have tools_checked metric
	if _, exists := result.Metrics["tools_checked"]; !exists {
		t.Error("Result should contain tools_checked metric")
	}
}

func TestConvertToolsConfigToLegacy(t *testing.T) {
	// Test the conversion function
	toolConfigs := []doctor.ToolConfig{
		{
			Name:          "test-tool",
			Description:   "A test tool",
			Kind:          "system",
			DetectCommand: "test-tool --version",
			InstallCommands: map[string]string{
				"linux":  "apt-get install test-tool",
				"darwin": "brew install test-tool",
			},
		},
	}

	tools, err := convertToolsConfigToLegacy(toolConfigs)
	if err != nil {
		t.Errorf("convertToolsConfigToLegacy should not return error: %v", err)
	}

	if len(tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(tools))
	}

	tool := tools[0]
	if tool.Name != "test-tool" {
		t.Errorf("Expected tool name 'test-tool', got '%s'", tool.Name)
	}

	if tool.Kind != "system" {
		t.Errorf("Expected tool kind 'system', got '%s'", tool.Kind)
	}

	// Check that install methods were converted
	if tool.InstallMethods == nil {
		t.Error("Install methods should not be nil")
	}

	if len(tool.InstallMethods) == 0 {
		t.Error("Install methods should not be empty")
	}
}
