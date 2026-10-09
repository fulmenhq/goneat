package doctor

import (
	"strings"
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

func TestGoLicensesDefaultsPinnedToV2(t *testing.T) {
	t.Parallel()

	config, err := LoadToolsDefaultsConfig()
	if err != nil {
		t.Fatalf("Failed to load tools defaults config: %v", err)
	}

	const (
		wantPackage = "github.com/google/go-licenses/v2@v2.0.1"
		wantVersion = "2.0.1"
	)
	for _, tool := range config.GetAllTools() {
		if tool.Name == "go-licenses" {
			if tool.InstallPackage != wantPackage {
				t.Fatalf("go-licenses install package = %q, want %q", tool.InstallPackage, wantPackage)
			}
			if tool.VersionScheme != "semver" {
				t.Fatalf("go-licenses version scheme = %q, want semver", tool.VersionScheme)
			}
			if tool.MinimumVersion != wantVersion {
				t.Fatalf("go-licenses minimum version = %q, want %q", tool.MinimumVersion, wantVersion)
			}
			if tool.RecommendedVersion != wantVersion {
				t.Fatalf("go-licenses recommended version = %q, want %q", tool.RecommendedVersion, wantVersion)
			}
			return
		}
	}

	t.Fatal("go-licenses tool definition not found")
}

func TestRecommendedToolVersionLock(t *testing.T) {
	t.Parallel()

	defaults, err := LoadToolsDefaultsConfig()
	if err != nil {
		t.Fatalf("Failed to load tools defaults config: %v", err)
	}
	repository, err := LoadToolsConfig()
	if err != nil {
		t.Fatalf("Failed to load repository tools config: %v", err)
	}

	type expectedVersion struct {
		recommended       string
		defaultMinimum    string
		repositoryMinimum string
	}
	expected := map[string]expectedVersion{
		"actionlint":    {recommended: "1.7.12", defaultMinimum: "1.7.0", repositoryMinimum: "1.7.0"},
		"biome":         {recommended: "2.5.5", defaultMinimum: "2.0.0", repositoryMinimum: "2.0.0"},
		"gitleaks":      {recommended: "8.30.1"},
		"go":            {recommended: "1.26.9", defaultMinimum: "1.21.0", repositoryMinimum: "1.26.0"},
		"go-licenses":   {recommended: "2.0.1", defaultMinimum: "2.0.1", repositoryMinimum: "2.0.1"},
		"golangci-lint": {recommended: "2.12.2", defaultMinimum: "2.0.0", repositoryMinimum: "2.0.0"},
		"gosec":         {recommended: "2.28.0", defaultMinimum: "2.18.0", repositoryMinimum: "2.18.0"},
		"govulncheck":   {recommended: "1.6.0", defaultMinimum: "1.0.0", repositoryMinimum: "1.0.0"},
		"grype":         {recommended: "0.120.0", defaultMinimum: "0.120.0", repositoryMinimum: "0.120.0"},
		"jq":            {recommended: "1.8.2", defaultMinimum: "1.7.0", repositoryMinimum: "1.7.0"},
		"prettier":      {recommended: "3.9.6", defaultMinimum: "3.0.0", repositoryMinimum: "3.0.0"},
		"ruff":          {recommended: "0.15.22", defaultMinimum: "0.8.0", repositoryMinimum: "0.8.0"},
		"shfmt":         {recommended: "3.13.1"},
		"syft":          {recommended: "1.54.0", defaultMinimum: "1.54.0", repositoryMinimum: "1.54.0"},
		"yamlfmt":       {recommended: "0.21.0", defaultMinimum: "0.16.0", repositoryMinimum: "0.16.0"},
		"yamllint":      {recommended: "1.38.0", defaultMinimum: "1.33.0", repositoryMinimum: "1.33.0"},
		"yq":            {recommended: "4.53.3", defaultMinimum: "4.40.0", repositoryMinimum: "4.40.0"},
	}

	defaultTools := make(map[string]ToolDefinition)
	for _, tool := range defaults.GetAllTools() {
		defaultTools[tool.Name] = tool
	}

	for name, want := range expected {
		t.Run(name, func(t *testing.T) {
			defaultTool, ok := defaultTools[name]
			if !ok {
				t.Fatalf("default tool %q not found", name)
			}
			assertToolVersionLock(t, "defaults", defaultTool.VersionScheme, defaultTool.MinimumVersion,
				defaultTool.RecommendedVersion, want.defaultMinimum, want.recommended)

			repositoryTool, ok := repository.Tools[name]
			if !ok {
				t.Fatalf("repository tool %q not found", name)
			}
			assertToolVersionLock(t, "repository", repositoryTool.VersionScheme, repositoryTool.MinimumVersion,
				repositoryTool.RecommendedVersion, want.repositoryMinimum, want.recommended)
		})
	}
}

// Equal scanner minimum/recommendation pins have no warning-only band:
// every older version fails both comparisons and upgrade validation.
func TestScannerVersionPolicyBoundaries(t *testing.T) {
	t.Parallel()
	defaults, err := LoadToolsDefaultsConfig()
	if err != nil {
		t.Fatal(err)
	}
	repository, err := LoadToolsConfig()
	if err != nil {
		t.Fatal(err)
	}
	configs := map[string]*ToolsConfig{
		"defaults":   ConvertToToolsConfig(defaults.GetAllTools(), "all", "All tools"),
		"repository": repository,
	}
	for surface, config := range configs {
		for _, scanner := range []struct {
			name, pinned, older, previous, newer string
		}{
			{"syft", "1.54.0", "1.53.0", "1.52.0", "1.54.1"},
			{"grype", "0.120.0", "0.119.1", "0.119.0", "0.120.1"},
		} {
			t.Run(surface+"/"+scanner.name, func(t *testing.T) {
				declared, ok := config.GetTool(scanner.name)
				if !ok {
					t.Fatalf("missing scanner %s", scanner.name)
				}
				policy, err := declared.VersionPolicy()
				if err != nil {
					t.Fatal(err)
				}
				if policy.MinimumVersion != scanner.pinned || policy.RecommendedVersion != scanner.pinned {
					t.Fatalf("scanner policy is not pinned to %s: %+v", scanner.pinned, policy)
				}
				tool := Tool{Name: declared.Name, Kind: declared.Kind, VersionPolicy: policy}
				for _, fixture := range []struct {
					version  string
					wantPass bool
				}{
					{scanner.older, false},
					{scanner.previous, false},
					{scanner.pinned + "-rc.1", false},
					{scanner.pinned, true},
					{"v" + scanner.pinned, true},
					{scanner.newer, true},
				} {
					t.Run(fixture.version, func(t *testing.T) {
						status := Status{Name: scanner.name, Present: true, Version: fixture.version}
						applyVersionPolicy(tool, &status)
						if status.PolicyError != nil || status.PolicyEvaluation == nil {
							t.Fatalf("missing policy evaluation: %+v", status)
						}
						if status.PolicyEvaluation.MeetsMinimum != fixture.wantPass || status.PolicyEvaluation.MeetsRecommended != fixture.wantPass {
							t.Fatalf("minimum/recommendation boundary mismatch: %+v", status.PolicyEvaluation)
						}
						err := ValidateUpgradeResult(tool, status)
						if fixture.wantPass {
							if err != nil {
								t.Fatalf("compliant scanner rejected: %v", err)
							}
						} else if err == nil || !strings.Contains(err.Error(), "below minimum") {
							t.Fatalf("older scanner must fail minimum, not only recommendation: %v", err)
						}
					})
				}
			})
		}
	}
}

func assertToolVersionLock(
	t *testing.T,
	surface string,
	versionScheme string,
	minimumVersion string,
	recommendedVersion string,
	wantMinimum string,
	wantRecommended string,
) {
	t.Helper()

	if versionScheme != "semver" {
		t.Errorf("%s version scheme = %q, want semver", surface, versionScheme)
	}
	if minimumVersion != wantMinimum {
		t.Errorf("%s minimum version = %q, want %q", surface, minimumVersion, wantMinimum)
	}
	if recommendedVersion != wantRecommended {
		t.Errorf("%s recommended version = %q, want %q", surface, recommendedVersion, wantRecommended)
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
