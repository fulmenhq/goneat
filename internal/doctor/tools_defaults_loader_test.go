package doctor

import (
	"testing"
)

func TestLoadToolsDefaultsConfig(t *testing.T) {
	t.Parallel()
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

	// Verify expected scopes (v0.4.4+ toolchain scopes)
	expectedScopes := []string{"foundation", "go", "rust", "python", "typescript", "security", "sbom", "cicd", "all"}
	for _, scope := range expectedScopes {
		if _, exists := config.Scopes[scope]; !exists {
			t.Errorf("Expected scope %s not found", scope)
		}
	}
}

func TestGetAllTools(t *testing.T) {
	t.Parallel()
	config, err := LoadToolsDefaultsConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	allTools := config.GetAllTools()

	// Should have tools from all categories (v0.4.4+: foundation, go, rust, python, typescript, security, sbom, cicd)
	expectedMinimum := len(config.FoundationTools) + len(config.GoTools) + len(config.RustTools) +
		len(config.PythonTools) + len(config.TypeScriptTools) + len(config.SecurityTools) +
		len(config.SbomTools) + len(config.CicdTools)
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
	t.Parallel()
	config, err := LoadToolsDefaultsConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	tests := []struct {
		scope       string
		expectError bool
		minTools    int
	}{
		{"foundation", false, 5}, // Language-agnostic tools
		{"go", false, 5},         // Go toolchain
		{"rust", false, 2},       // Cargo plugins
		{"python", false, 1},     // ruff
		{"typescript", false, 1}, // biome
		{"security", false, 1},   // gitleaks (cross-language)
		{"sbom", false, 2},       // syft, grype
		{"all", false, 16},       // All tools (added grype)
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
	t.Parallel()
	config, err := LoadToolsDefaultsConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// v0.4.4+: The new scope-based structure doesn't use required_for_languages on individual tools.
	// Language filtering now works via scopes (e.g., "go" scope, "rust" scope).
	// This test verifies the scope-based filtering approach.
	tests := []struct {
		scope      string
		expectTool string // Tool we expect to be in this scope
		dontExpect string // Tool we expect NOT to be in this scope
	}{
		{
			scope:      "go",
			expectTool: "golangci-lint",
			dontExpect: "ruff", // Python tool
		},
		{
			scope:      "python",
			expectTool: "ruff",
			dontExpect: "golangci-lint", // Go tool
		},
		{
			scope:      "foundation",
			expectTool: "ripgrep", // Universal tool
			dontExpect: "go",      // Language-specific tool (in go scope)
		},
	}

	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			// Get tools for this scope
			scopeTools, err := config.GetToolsForScope(tt.scope)
			if err != nil {
				t.Fatalf("Failed to get scope %s: %v", tt.scope, err)
			}

			foundExpected := false
			foundUnexpected := false

			for _, tool := range scopeTools {
				if tool.Name == tt.expectTool {
					foundExpected = true
				}
				if tool.Name == tt.dontExpect {
					foundUnexpected = true
				}
			}

			if !foundExpected && tt.expectTool != "" {
				t.Errorf("Expected tool %s not found in scope %s", tt.expectTool, tt.scope)
			}

			if foundUnexpected {
				t.Errorf("Unexpected tool %s found in scope %s", tt.dontExpect, tt.scope)
			}

			t.Logf("Scope %s: %d tools", tt.scope, len(scopeTools))
		})
	}
}

func TestGetMinimalToolsForLanguage(t *testing.T) {
	t.Parallel()
	config, err := LoadToolsDefaultsConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// v0.4.4+: Minimal tools are now accessed via language-specific scopes.
	// This test verifies that each language scope has the expected tools.
	tests := []struct {
		scope    string
		minTools int
		maxTools int
	}{
		{"go", 5, 10},         // go, go-licenses, golangci-lint, goimports, gofmt, gosec, govulncheck
		{"python", 1, 5},      // ruff
		{"typescript", 1, 5},  // biome
		{"foundation", 5, 15}, // Language-agnostic tools
	}

	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			scopeTools, err := config.GetToolsForScope(tt.scope)
			if err != nil {
				t.Fatalf("Failed to get scope %s: %v", tt.scope, err)
			}

			if len(scopeTools) < tt.minTools {
				t.Errorf("Expected at least %d tools for scope %s, got %d",
					tt.minTools, tt.scope, len(scopeTools))
			}

			if tt.maxTools > 0 && len(scopeTools) > tt.maxTools {
				t.Errorf("Expected at most %d tools for scope %s, got %d",
					tt.maxTools, tt.scope, len(scopeTools))
			}

			t.Logf("Tools for scope %s: %d", tt.scope, len(scopeTools))
			for _, tool := range scopeTools {
				t.Logf("  - %s (kind: %s)", tool.Name, tool.Kind)
			}
		})
	}
}

func TestConvertToToolsConfig(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
