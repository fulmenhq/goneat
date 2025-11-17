package doctor

import (
	"testing"
)

func TestLoadToolsDefaultsConfig(t *testing.T) {
	config, err := LoadToolsDefaultsConfig()
	if err != nil {
		t.Fatalf("Failed to load tools defaults config: %v", err)
	}

	if config.Version == "" {
		t.Error("Config version is empty")
	}

	if len(config.FoundationTools) == 0 {
		t.Error("No foundation tools defined")
	}

	if len(config.Scopes) == 0 {
		t.Error("No scopes defined")
	}

	// Verify expected scopes
	expectedScopes := []string{"foundation", "security", "format", "all"}
	for _, scope := range expectedScopes {
		if _, exists := config.Scopes[scope]; !exists {
			t.Errorf("Expected scope %s not found", scope)
		}
	}
}

func TestGetAllTools(t *testing.T) {
	config, err := LoadToolsDefaultsConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	allTools := config.GetAllTools()

	// Should have tools from all categories
	expectedMinimum := len(config.FoundationTools) + len(config.SecurityTools) + len(config.FormatTools)
	if len(allTools) < expectedMinimum {
		t.Errorf("Expected at least %d tools, got %d", expectedMinimum, len(allTools))
	}

	// Verify some expected tools are present
	expectedTools := map[string]bool{
		"ripgrep":       false,
		"jq":            false,
		"go":            false,
		"golangci-lint": false,
		"gosec":         false,
	}

	for _, tool := range allTools {
		if _, exists := expectedTools[tool.Name]; exists {
			expectedTools[tool.Name] = true
		}
	}

	for toolName, found := range expectedTools {
		if !found {
			t.Errorf("Expected tool %s not found in GetAllTools()", toolName)
		}
	}
}

func TestGetToolsForScope(t *testing.T) {
	config, err := LoadToolsDefaultsConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	tests := []struct {
		scope       string
		expectError bool
		minTools    int
	}{
		{"foundation", false, 5},
		{"security", false, 2},
		{"format", false, 2},
		{"all", false, 10},
		{"nonexistent", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			tools, err := config.GetToolsForScope(tt.scope)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error for nonexistent scope")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if len(tools) < tt.minTools {
				t.Errorf("Expected at least %d tools for scope %s, got %d",
					tt.minTools, tt.scope, len(tools))
			}

			t.Logf("Scope %s has %d tools", tt.scope, len(tools))
		})
	}
}

func TestFilterToolsByLanguage(t *testing.T) {
	config, err := LoadToolsDefaultsConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	tests := []struct {
		language   string
		expectTool string // Tool we expect to be included
		dontExpect string // Tool we expect to be excluded
	}{
		{
			language:   "go",
			expectTool: "go",
			dontExpect: "ruff", // Python tool
		},
		{
			language:   "python",
			expectTool: "ruff",
			dontExpect: "golangci-lint", // Go tool
		},
		{
			language:   "unknown",
			expectTool: "ripgrep", // Universal tool
			dontExpect: "go",      // Language-specific tool
		},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			// Get all tools for this test
			allTools := config.GetAllTools()
			filtered := FilterToolsByLanguage(allTools, tt.language)

			foundExpected := false
			foundUnexpected := false

			for _, tool := range filtered {
				if tool.Name == tt.expectTool {
					foundExpected = true
				}
				if tool.Name == tt.dontExpect {
					foundUnexpected = true
				}
			}

			if !foundExpected && tt.expectTool != "" {
				t.Errorf("Expected tool %s not found for language %s", tt.expectTool, tt.language)
			}

			if foundUnexpected {
				t.Errorf("Unexpected tool %s found for language %s", tt.dontExpect, tt.language)
			}

			t.Logf("Language %s: filtered to %d tools (from %d)", tt.language, len(filtered), len(allTools))
		})
	}
}

func TestGetMinimalToolsForLanguage(t *testing.T) {
	config, err := LoadToolsDefaultsConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	tests := []struct {
		language string
		minTools int
		maxTools int
	}{
		{"go", 1, 10},        // Should include go-install, golangci-lint, etc.
		{"python", 1, 10},    // Should include uv, ruff, etc.
		{"typescript", 1, 5}, // Should include npm, eslint, prettier
		{"unknown", 0, 0},    // Should have nothing
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			allTools := config.GetAllTools()
			minimal := GetMinimalToolsForLanguage(allTools, tt.language)

			if len(minimal) < tt.minTools {
				t.Errorf("Expected at least %d minimal tools for %s, got %d",
					tt.minTools, tt.language, len(minimal))
			}

			if tt.maxTools > 0 && len(minimal) > tt.maxTools {
				t.Errorf("Expected at most %d minimal tools for %s, got %d",
					tt.maxTools, tt.language, len(minimal))
			}

			t.Logf("Minimal tools for %s: %d", tt.language, len(minimal))
			for _, tool := range minimal {
				t.Logf("  - %s (kind: %s)", tool.Name, tool.Kind)
			}
		})
	}
}

func TestConvertToToolsConfig(t *testing.T) {
	config, err := LoadToolsDefaultsConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	foundationTools, err := config.GetToolsForScope("foundation")
	if err != nil {
		t.Fatalf("Failed to get foundation tools: %v", err)
	}

	// Filter for Go language
	goTools := FilterToolsByLanguage(foundationTools, "go")

	toolsConfig := ConvertToToolsConfig(goTools, "foundation", "Foundation tools for Go projects")

	if len(toolsConfig.Tools) == 0 {
		t.Error("Converted config has no tools")
	}

	if len(toolsConfig.Scopes) == 0 {
		t.Error("Converted config has no scopes")
	}

	foundationScope, exists := toolsConfig.Scopes["foundation"]
	if !exists {
		t.Fatal("Foundation scope not created")
	}

	if foundationScope.Description == "" {
		t.Error("Foundation scope has no description")
	}

	if len(foundationScope.Tools) != len(toolsConfig.Tools) {
		t.Errorf("Scope has %d tools but config has %d tools",
			len(foundationScope.Tools), len(toolsConfig.Tools))
	}

	// Verify all scope tools exist in tools list
	toolNames := make(map[string]bool)
	for _, tool := range toolsConfig.Tools {
		toolNames[tool.Name] = true
	}

	for _, scopeTool := range foundationScope.Tools {
		if !toolNames[scopeTool] {
			t.Errorf("Scope references tool %s which doesn't exist in tools list", scopeTool)
		}
	}

	t.Logf("Converted %d tools to ToolsConfig format", len(toolsConfig.Tools))
}

func TestConvertToToolsConfig_PackageManagers(t *testing.T) {
	config, err := LoadToolsDefaultsConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	allTools := config.GetAllTools()

	// Find a tool with platform-specific package managers
	var testTool ToolDefinition
	for _, tool := range allTools {
		if tool.Name == "ripgrep" {
			testTool = tool
			break
		}
	}

	if testTool.Name == "" {
		t.Skip("ripgrep tool not found for testing")
	}

	converted := ConvertToToolsConfig([]ToolDefinition{testTool}, "test", "Test scope")

	if len(converted.Tools) == 0 {
		t.Fatal("No tools in converted config")
	}

	tool, exists := converted.Tools["ripgrep"]
	if !exists {
		t.Fatal("ripgrep tool not found in converted config")
	}

	if len(tool.InstallerPriority) == 0 {
		t.Error("Installer priority (package managers) not converted")
	}

	// Verify platform-specific package managers were converted
	if managers, exists := tool.InstallerPriority["darwin"]; exists {
		if len(managers) == 0 {
			t.Error("Darwin package managers empty")
		}
		t.Logf("Darwin package managers: %v", managers)
	}
}
