package tools

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/pkg/versioning"
)

const sampleConfig = `scopes:
  foundation:
    description: "Foundation tools"
    tools: ["rg", "jq"]
  lint:
    description: "Linting"
    tools: ["golangci"]
tools:
  rg:
    name: "rg"
    description: "ripgrep"
    kind: "system"
    detect_command: "rg --version"
  jq:
    name: "jq"
    description: "jq"
    kind: "system"
    detect_command: "jq --version"
  golangci:
    name: "golangci"
    description: "golangci-lint"
    kind: "go"
    detect_command: "golangci-lint --version"
    version_scheme: "semver"
    minimum_version: "1.60.0"
    recommended_version: "1.61.0"
    disallowed_versions: ["1.59.0"]
    installer_priority:
      linux: ["mise", "pacman", "apt-get", "dnf", "yum"]
      darwin: ["mise", "brew"]
`

// v1.1.0 config with package_manager install
const sampleConfigV110Brew = `scopes:
  cli:
    description: "CLI tools via brew"
    tools: ["goneat", "docker"]
tools:
  goneat:
    name: "goneat"
    description: "Fulmen CLI"
    kind: "system"
    detect_command: "goneat --version"
    install:
      type: package_manager
      package_manager:
        manager: brew
        tap: fulmenhq/homebrew-tap
        package: fulmenhq/tap/goneat
        package_type: formula
        flags: ["--quiet"]
  docker:
    name: "docker"
    description: "Docker Desktop"
    kind: "system"
    detect_command: "docker --version"
    install:
      type: package_manager
      package_manager:
        manager: brew
        package: docker
        package_type: cask
`

const sampleConfigV110Scoop = `scopes:
  windows-tools:
    description: "Windows tools via scoop"
    tools: ["ripgrep"]
tools:
  ripgrep:
    name: "ripgrep"
    description: "Fast grep"
    kind: "system"
    detect_command: "rg --version"
    install:
      type: package_manager
      package_manager:
        manager: scoop
        bucket: main
        package: ripgrep
        flags: ["--no-cache"]
`

// v1.0.0 legacy config with install_commands (backward compatibility test)
const sampleConfigV100Legacy = `scopes:
  legacy:
    description: "Legacy install commands"
    tools: ["ripgrep"]
tools:
  ripgrep:
    name: "ripgrep"
    description: "Fast grep"
    kind: "system"
    detect_command: "rg --version"
    install_commands:
      darwin: "brew install ripgrep"
      linux: "brew install ripgrep"
`

func TestParseConfig(t *testing.T) {
	cfg, err := ParseConfig([]byte(sampleConfig))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(cfg.Scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(cfg.Scopes))
	}
	if len(cfg.Tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(cfg.Tools))
	}
	golangci := cfg.Tools["golangci"]
	if len(golangci.InstallerPriority["linux"]) == 0 {
		t.Fatalf("expected installer priority for linux")
	}
}

func TestGetToolsForScope(t *testing.T) {
	cfg, err := ParseConfig([]byte(sampleConfig))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	tools, err := cfg.GetToolsForScope("foundation")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
}

func TestGetToolsForScope_SynthesizedAll(t *testing.T) {
	cfg, err := ParseConfig([]byte(sampleConfig))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	// "all" is not defined in sampleConfig — should be synthesized
	tools, err := cfg.GetToolsForScope("all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// sampleConfig has 3 unique tools across 2 scopes
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools from synthesized all scope, got %d", len(tools))
	}
	// Verify deduplication: tool names should be unique
	seen := make(map[string]bool)
	for _, tool := range tools {
		if seen[tool.Name] {
			t.Errorf("duplicate tool in synthesized all scope: %s", tool.Name)
		}
		seen[tool.Name] = true
	}
}

func TestGetAllScopes_IncludesAll(t *testing.T) {
	cfg, err := ParseConfig([]byte(sampleConfig))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	scopes := cfg.GetAllScopes()
	found := false
	for _, s := range scopes {
		if s == "all" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GetAllScopes() should include synthesized 'all', got: %v", scopes)
	}
}

func TestVersionPolicy(t *testing.T) {
	tests := []struct {
		name       string
		tool       Tool
		wantScheme versioning.Scheme
		wantMin    string
		wantRec    string
		wantDis    []string
		wantErr    bool
		errMsg     string
	}{
		{
			name: "full_policy_semver",
			tool: Tool{
				VersionScheme:      "semver",
				MinimumVersion:     "1.60.0",
				RecommendedVersion: "1.61.0",
				DisallowedVersions: []string{"1.59.0"},
			},
			wantScheme: versioning.SchemeSemverFull,
			wantMin:    "1.60.0",
			wantRec:    "1.61.0",
			wantDis:    []string{"1.59.0"},
			wantErr:    false,
		},
		{
			name: "full_policy_calver",
			tool: Tool{
				VersionScheme:      "calver",
				MinimumVersion:     "2024.09.01",
				RecommendedVersion: "2024.10.01",
				DisallowedVersions: []string{"2024.08.01"},
			},
			wantScheme: versioning.SchemeCalver,
			wantMin:    "2024.09.01",
			wantRec:    "2024.10.01",
			wantDis:    []string{"2024.08.01"},
			wantErr:    false,
		},
		{
			name: "minimum_only",
			tool: Tool{
				VersionScheme:  "semver",
				MinimumVersion: "1.0.0",
			},
			wantScheme: versioning.SchemeSemverFull,
			wantMin:    "1.0.0",
			wantRec:    "",
			wantDis:    nil,
			wantErr:    false,
		},
		{
			name: "recommended_only",
			tool: Tool{
				VersionScheme:      "semver",
				RecommendedVersion: "2.0.0",
			},
			wantScheme: versioning.SchemeSemverFull,
			wantMin:    "",
			wantRec:    "2.0.0",
			wantDis:    nil,
			wantErr:    false,
		},
		{
			name: "disallowed_only",
			tool: Tool{
				VersionScheme:      "semver",
				DisallowedVersions: []string{"1.0.0", "2.0.0"},
			},
			wantScheme: versioning.SchemeSemverFull,
			wantMin:    "",
			wantRec:    "",
			wantDis:    []string{"1.0.0", "2.0.0"},
			wantErr:    false,
		},
		{
			name: "zero_policy_empty_scheme",
			tool: Tool{
				VersionScheme: "",
			},
			wantScheme: versioning.SchemeLexical,
			wantMin:    "",
			wantRec:    "",
			wantDis:    nil,
			wantErr:    false,
		},
		{
			name:       "zero_policy_no_scheme",
			tool:       Tool{},
			wantScheme: versioning.SchemeLexical,
			wantMin:    "",
			wantRec:    "",
			wantDis:    nil,
			wantErr:    false,
		},
		{
			name: "invalid_scheme",
			tool: Tool{
				VersionScheme:  "invalid",
				MinimumVersion: "1.0.0",
			},
			wantErr: true,
			errMsg:  "unsupported version scheme",
		},
		{
			name: "scheme_clone_disallowed",
			tool: Tool{
				VersionScheme:      "semver",
				DisallowedVersions: []string{"1.0.0", "2.0.0"},
			},
			wantScheme: versioning.SchemeSemverFull,
			wantDis:    []string{"1.0.0", "2.0.0"},
			wantErr:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy, err := tc.tool.VersionPolicy()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing '%s', got nil", tc.errMsg)
				}
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("expected error containing '%s', got: %v", tc.errMsg, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if policy.Scheme != tc.wantScheme {
					t.Fatalf("expected scheme %v, got %v", tc.wantScheme, policy.Scheme)
				}
				if policy.MinimumVersion != tc.wantMin {
					t.Fatalf("expected minimum %v, got %v", tc.wantMin, policy.MinimumVersion)
				}
				if policy.RecommendedVersion != tc.wantRec {
					t.Fatalf("expected recommended %v, got %v", tc.wantRec, policy.RecommendedVersion)
				}
				if len(policy.DisallowedVersions) != len(tc.wantDis) {
					t.Fatalf("expected disallowed length %d, got %d", len(tc.wantDis), len(policy.DisallowedVersions))
				}
				for i, v := range tc.wantDis {
					if policy.DisallowedVersions[i] != v {
						t.Fatalf("expected disallowed[%d] %v, got %v", i, v, policy.DisallowedVersions[i])
					}
				}
				// Verify disallowed slice is cloned (not the same reference)
				if len(tc.wantDis) > 0 && &policy.DisallowedVersions[0] == &tc.tool.DisallowedVersions[0] {
					t.Fatal("disallowed slice should be cloned, not reference the original")
				}
			}
		})
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		override string
		check    func(*Config) error
	}{
		{
			name: "basic_merge",
			base: sampleConfig,
			override: `scopes:
  custom:
    description: "Custom"
    tools: ["yamlfmt"]
tools:
  yamlfmt:
    name: "yamlfmt"
    description: "YAML formatter"
    kind: "go"
    detect_command: "yamlfmt --version"`,
			check: func(cfg *Config) error {
				if _, ok := cfg.Scopes["custom"]; !ok {
					return fmt.Errorf("expected custom scope to be merged")
				}
				if _, ok := cfg.Tools["yamlfmt"]; !ok {
					return fmt.Errorf("expected yamlfmt tool to be merged")
				}
				return nil
			},
		},
		{
			name: "override_existing_tool",
			base: sampleConfig,
			override: `tools:
   golangci:
     name: "golangci"
     description: "Updated golangci-lint"
     kind: "go"
     detect_command: "golangci-lint --version"
     version_scheme: "semver"
     minimum_version: "1.70.0"`, // Override minimum version
			check: func(cfg *Config) error {
				tool, ok := cfg.Tools["golangci"]
				if !ok {
					return fmt.Errorf("expected golangci tool to exist")
				}
				if tool.MinimumVersion != "1.70.0" {
					return fmt.Errorf("expected minimum version to be overridden to 1.70.0, got %s", tool.MinimumVersion)
				}
				if tool.Description != "Updated golangci-lint" {
					return fmt.Errorf("expected description to be overridden")
				}
				return nil
			},
		},
		{
			name:     "merge_nil_config",
			base:     sampleConfig,
			override: "",
			check: func(cfg *Config) error {
				// Should not panic or modify when merging nil
				cfg.Merge(nil)
				if len(cfg.Scopes) != 2 {
					return fmt.Errorf("expected 2 scopes after nil merge, got %d", len(cfg.Scopes))
				}
				return nil
			},
		},
		{
			name: "merge_empty_config",
			base: sampleConfig,
			override: `scopes: {}
tools: {}`,
			check: func(cfg *Config) error {
				if len(cfg.Scopes) != 2 {
					return fmt.Errorf("expected 2 scopes after empty merge, got %d", len(cfg.Scopes))
				}
				return nil
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseConfig([]byte(tc.base))
			if err != nil {
				t.Fatalf("unexpected base parse error: %v", err)
			}

			if tc.override != "" {
				override, err := ParseConfig([]byte(tc.override))
				if err != nil {
					t.Fatalf("unexpected override parse error: %v", err)
				}
				cfg.Merge(override)
			}

			if err := tc.check(cfg); err != nil {
				t.Fatalf("check failed: %v", err)
			}
		})
	}
}

// TestRoundtripValidation tests parsing, merging, and policy evaluation
func TestRoundtripValidation(t *testing.T) {
	configYAML := `scopes:
  foundation:
    description: "Foundation tools"
    tools: ["golangci", "rg"]
  lint:
    description: "Linting"
    tools: ["golangci"]
tools:
  golangci:
    name: "golangci"
    description: "golangci-lint"
    kind: "go"
    detect_command: "golangci-lint --version"
    version_scheme: "semver"
    minimum_version: "1.60.0"
    recommended_version: "1.61.0"
    disallowed_versions: ["1.59.0"]
  rg:
    name: "rg"
    description: "ripgrep"
    kind: "system"
    detect_command: "rg --version"
    version_scheme: "semver"
    minimum_version: "13.0.0"`

	// Parse config
	cfg, err := ParseConfig([]byte(configYAML))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	// Validate structure
	if len(cfg.Scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(cfg.Scopes))
	}
	if len(cfg.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(cfg.Tools))
	}

	// Test policy extraction for each tool
	for toolName, tool := range cfg.Tools {
		t.Run("policy_"+toolName, func(t *testing.T) {
			policy, err := tool.VersionPolicy()
			if err != nil {
				t.Fatalf("unexpected policy error for %s: %v", toolName, err)
			}

			if policy.IsZero() {
				t.Fatalf("expected non-zero policy for %s", toolName)
			}

			if tool.MinimumVersion != "" {
				eval, err := versioning.Evaluate(policy, tool.MinimumVersion)
				if err != nil {
					t.Fatalf("unexpected evaluation error: %v", err)
				}
				if !eval.MeetsMinimum {
					t.Fatalf("minimum version should meet minimum for %s", toolName)
				}
			}
		})
	}

	// Test scope resolution
	tools, err := cfg.GetToolsForScope("foundation")
	if err != nil {
		t.Fatalf("unexpected error getting foundation tools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 foundation tools, got %d", len(tools))
	}

	// Verify tools are correctly resolved
	found := make(map[string]bool)
	for _, tool := range tools {
		found[tool.Name] = true
	}
	if !found["golangci"] || !found["rg"] {
		t.Fatalf("expected golangci and rg in foundation scope, found: %v", found)
	}
}

// TestSchemaValidation tests that configs pass schema validation
func TestSchemaValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr bool
	}{
		{
			name:    "valid_config",
			config:  sampleConfig,
			wantErr: false,
		},
		{
			name: "invalid_tool_missing_name",
			config: `tools:
   badtool:
     description: "Missing name"
     kind: "go"`,
			wantErr: true,
		},
		{
			name: "invalid_scope_missing_description",
			config: `scopes:
   badscope:
     tools: ["tool1"]
 tools:
   tool1:
     name: "tool1"
     kind: "go"`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateBytes([]byte(tc.config))
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected validation error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected validation error: %v", err)
				}
			}
		})
	}
}

func TestMergeScopeExtend(t *testing.T) {
	base, err := ParseConfig([]byte(sampleConfig))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	override, err := ParseConfig([]byte(`scopes:
  foundation:
    description: "Foundation tools (extended)"
    tools: ["yamlfmt"]
`))
	if err != nil {
		t.Fatalf("unexpected override parse error: %v", err)
	}
	base.Merge(override)
	scope := base.Scopes["foundation"]
	if scope.Description != "Foundation tools (extended)" {
		t.Fatalf("expected description override, got %s", scope.Description)
	}
	if len(scope.Tools) != 3 {
		t.Fatalf("expected 3 tools after merge, got %d", len(scope.Tools))
	}
	if scope.Replace {
		t.Fatal("replace flag should be cleared after merge")
	}
	if scope.Tools[2] != "yamlfmt" {
		t.Fatalf("expected yamlfmt appended, got %v", scope.Tools)
	}
}

func TestMergeScopeReplace(t *testing.T) {
	base, err := ParseConfig([]byte(sampleConfig))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	override, err := ParseConfig([]byte(`scopes:
  foundation:
    description: "Custom foundation"
    tools: ["customtool"]
    replace: true
`))
	if err != nil {
		t.Fatalf("unexpected override parse error: %v", err)
	}
	base.Merge(override)
	scope := base.Scopes["foundation"]
	if len(scope.Tools) != 1 || scope.Tools[0] != "customtool" {
		t.Fatalf("expected replacement tools, got %v", scope.Tools)
	}
	if scope.Replace {
		t.Fatal("replace flag should be cleared after merge")
	}
}

// TestPackageManagerConfigV110 tests v1.1.0 package manager configurations
func TestPackageManagerConfigV110(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		toolName string
		wantMgr  string
		wantPkg  string
		wantType string
	}{
		{
			name:     "brew_formula",
			config:   sampleConfigV110Brew,
			toolName: "goneat",
			wantMgr:  "brew",
			wantPkg:  "fulmenhq/tap/goneat",
			wantType: "formula",
		},
		{
			name:     "brew_cask",
			config:   sampleConfigV110Brew,
			toolName: "docker",
			wantMgr:  "brew",
			wantPkg:  "docker",
			wantType: "cask",
		},
		{
			name:     "scoop",
			config:   sampleConfigV110Scoop,
			toolName: "ripgrep",
			wantMgr:  "scoop",
			wantPkg:  "ripgrep",
			wantType: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseConfig([]byte(tc.config))
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}

			tool, ok := cfg.Tools[tc.toolName]
			if !ok {
				t.Fatalf("tool %s not found", tc.toolName)
			}

			if tool.Install == nil {
				t.Fatal("expected install config, got nil")
			}

			if tool.Install.Type != "package_manager" {
				t.Fatalf("expected type package_manager, got %s", tool.Install.Type)
			}

			if tool.Install.PackageManager == nil {
				t.Fatal("expected package_manager config, got nil")
			}

			pm := tool.Install.PackageManager
			if pm.Manager != tc.wantMgr {
				t.Fatalf("expected manager %s, got %s", tc.wantMgr, pm.Manager)
			}

			if pm.Package != tc.wantPkg {
				t.Fatalf("expected package %s, got %s", tc.wantPkg, pm.Package)
			}

			if tc.wantType != "" && pm.PackageType != tc.wantType {
				t.Fatalf("expected package_type %s, got %s", tc.wantType, pm.PackageType)
			}
		})
	}
}

// TestPackageManagerFlags tests flag parsing
func TestPackageManagerFlags(t *testing.T) {
	cfg, err := ParseConfig([]byte(sampleConfigV110Brew))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	goneat := cfg.Tools["goneat"]
	if goneat.Install == nil || goneat.Install.PackageManager == nil {
		t.Fatal("expected package manager config")
	}

	flags := goneat.Install.PackageManager.Flags
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(flags))
	}

	if flags[0] != "--quiet" {
		t.Fatalf("expected --quiet flag, got %s", flags[0])
	}
}

// TestBackwardCompatibilityV100 tests that v1.0.0 install_commands still work
func TestBackwardCompatibilityV100(t *testing.T) {
	cfg, err := ParseConfig([]byte(sampleConfigV100Legacy))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	tool, ok := cfg.Tools["ripgrep"]
	if !ok {
		t.Fatal("ripgrep tool not found")
	}

	// v1.0.0 uses install_commands, not install
	if tool.Install != nil {
		t.Fatal("expected nil install for v1.0.0 config")
	}

	if len(tool.InstallCommands) == 0 {
		t.Fatal("expected install_commands to be populated")
	}

	if tool.InstallCommands["darwin"] != "brew install ripgrep" {
		t.Fatalf("unexpected darwin install command: %s", tool.InstallCommands["darwin"])
	}
}

// TestSchemaValidationV110 tests schema validation for v1.1.0 configs
func TestSchemaValidationV110(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid_brew_formula",
			config:  sampleConfigV110Brew,
			wantErr: false,
		},
		{
			name:    "valid_scoop",
			config:  sampleConfigV110Scoop,
			wantErr: false,
		},
		{
			name:    "valid_v1.0.0_legacy",
			config:  sampleConfigV100Legacy,
			wantErr: false,
		},
		{
			name: "invalid_missing_manager",
			config: `scopes:
  test:
    description: "Test"
    tools: ["tool1"]
tools:
  tool1:
    name: "tool1"
    description: "Test tool"
    kind: "system"
    detect_command: "tool1 --version"
    install:
      type: package_manager
      package_manager:
        package: "tool1"`,
			wantErr: true,
			errMsg:  "manager",
		},
		{
			name: "invalid_missing_package",
			config: `scopes:
  test:
    description: "Test"
    tools: ["tool1"]
tools:
  tool1:
    name: "tool1"
    description: "Test tool"
    kind: "system"
    detect_command: "tool1 --version"
    install:
      type: package_manager
      package_manager:
        manager: brew`,
			wantErr: true,
			errMsg:  "package",
		},
		{
			name: "invalid_scoop_with_tap",
			config: `scopes:
  test:
    description: "Test"
    tools: ["tool1"]
tools:
  tool1:
    name: "tool1"
    description: "Test tool"
    kind: "system"
    detect_command: "tool1 --version"
    install:
      type: package_manager
      package_manager:
        manager: scoop
        tap: fulmenhq/tap
        package: tool1`,
			wantErr: true,
			errMsg:  "tap",
		},
		{
			name: "invalid_both_install_methods",
			config: `scopes:
  test:
    description: "Test"
    tools: ["badtool"]
tools:
  badtool:
    name: "badtool"
    description: "Tool with both install methods (should fail)"
    kind: "system"
    detect_command: "badtool --version"
    install:
      type: package_manager
      package_manager:
        manager: brew
        package: badtool
    install_commands:
      darwin: "brew install badtool"`,
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateBytes([]byte(tc.config))
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected validation error, got nil")
				}
				if tc.errMsg != "" && !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("expected error containing %q, got: %v", tc.errMsg, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected validation error: %v", err)
				}
			}
		})
	}
}

// TestBrewTapParsing tests that brew tap field is correctly parsed
func TestBrewTapParsing(t *testing.T) {
	cfg, err := ParseConfig([]byte(sampleConfigV110Brew))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	goneat := cfg.Tools["goneat"]
	if goneat.Install == nil || goneat.Install.PackageManager == nil {
		t.Fatal("expected package manager config")
	}

	if goneat.Install.PackageManager.Tap != "fulmenhq/homebrew-tap" {
		t.Fatalf("expected tap fulmenhq/homebrew-tap, got %s", goneat.Install.PackageManager.Tap)
	}
}

// TestScoopBucketParsing tests that scoop bucket field is correctly parsed
func TestScoopBucketParsing(t *testing.T) {
	cfg, err := ParseConfig([]byte(sampleConfigV110Scoop))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	rg := cfg.Tools["ripgrep"]
	if rg.Install == nil || rg.Install.PackageManager == nil {
		t.Fatal("expected package manager config")
	}

	if rg.Install.PackageManager.Bucket != "main" {
		t.Fatalf("expected bucket main, got %s", rg.Install.PackageManager.Bucket)
	}
}

// TestBrewWithoutTap tests brew installation without tap (tap is optional)
func TestBrewWithoutTap(t *testing.T) {
	config := `scopes:
  test:
    description: "Test"
    tools: ["jq"]
tools:
  jq:
    name: "jq"
    description: "JSON processor"
    kind: "system"
    detect_command: "jq --version"
    install:
      type: package_manager
      package_manager:
        manager: brew
        package: jq`

	cfg, err := ParseConfig([]byte(config))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	// Validate schema
	if err := ValidateBytes([]byte(config)); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	jq := cfg.Tools["jq"]
	if jq.Install == nil || jq.Install.PackageManager == nil {
		t.Fatal("expected package manager config")
	}

	if jq.Install.PackageManager.Tap != "" {
		t.Fatalf("expected empty tap, got %s", jq.Install.PackageManager.Tap)
	}

	if jq.Install.PackageManager.Manager != "brew" {
		t.Fatalf("expected manager brew, got %s", jq.Install.PackageManager.Manager)
	}
}

// TestMultipleFlags tests parsing of multiple CLI flags
func TestMultipleFlags(t *testing.T) {
	config := `scopes:
  test:
    description: "Test"
    tools: ["tool1"]
tools:
  tool1:
    name: "tool1"
    description: "Tool with multiple flags"
    kind: "system"
    detect_command: "tool1 --version"
    install:
      type: package_manager
      package_manager:
        manager: brew
        package: tool1
        flags: ["--quiet", "--force", "--no-cache"]`

	cfg, err := ParseConfig([]byte(config))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if err := ValidateBytes([]byte(config)); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	tool := cfg.Tools["tool1"]
	pm := tool.Install.PackageManager
	if len(pm.Flags) != 3 {
		t.Fatalf("expected 3 flags, got %d", len(pm.Flags))
	}

	expectedFlags := []string{"--quiet", "--force", "--no-cache"}
	for i, expected := range expectedFlags {
		if pm.Flags[i] != expected {
			t.Fatalf("expected flag[%d] = %s, got %s", i, expected, pm.Flags[i])
		}
	}
}

// TestDestinationAndBinName tests destination and bin_name fields
func TestDestinationAndBinName(t *testing.T) {
	config := `scopes:
  test:
    description: "Test"
    tools: ["goneat"]
tools:
  goneat:
    name: "goneat"
    description: "Fulmen CLI"
    kind: "system"
    detect_command: "goneat --version"
    install:
      type: package_manager
      package_manager:
        manager: brew
        tap: fulmenhq/homebrew-tap
        package: fulmenhq/tap/goneat
        destination: ./bin
        bin_name: neat`

	cfg, err := ParseConfig([]byte(config))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if err := ValidateBytes([]byte(config)); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	goneat := cfg.Tools["goneat"]
	pm := goneat.Install.PackageManager

	if pm.Destination != "./bin" {
		t.Fatalf("expected destination ./bin, got %s", pm.Destination)
	}

	if pm.BinName != "neat" {
		t.Fatalf("expected bin_name neat, got %s", pm.BinName)
	}
}

// TestEmptyFlags tests that empty flags array is handled correctly
func TestEmptyFlags(t *testing.T) {
	config := `scopes:
  test:
    description: "Test"
    tools: ["tool1"]
tools:
  tool1:
    name: "tool1"
    description: "Tool with empty flags"
    kind: "system"
    detect_command: "tool1 --version"
    install:
      type: package_manager
      package_manager:
        manager: brew
        package: tool1
        flags: []`

	cfg, err := ParseConfig([]byte(config))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if err := ValidateBytes([]byte(config)); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	tool := cfg.Tools["tool1"]
	pm := tool.Install.PackageManager

	// Empty array should be valid
	if pm.Flags == nil {
		t.Fatal("expected empty flags array, got nil")
	}

	if len(pm.Flags) != 0 {
		t.Fatalf("expected 0 flags, got %d", len(pm.Flags))
	}
}

// TestNoFlags tests that omitting flags field is valid
func TestNoFlags(t *testing.T) {
	config := `scopes:
  test:
    description: "Test"
    tools: ["tool1"]
tools:
  tool1:
    name: "tool1"
    description: "Tool without flags field"
    kind: "system"
    detect_command: "tool1 --version"
    install:
      type: package_manager
      package_manager:
        manager: brew
        package: tool1`

	cfg, err := ParseConfig([]byte(config))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if err := ValidateBytes([]byte(config)); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	tool := cfg.Tools["tool1"]
	pm := tool.Install.PackageManager

	// No flags field should result in nil or empty slice
	if len(pm.Flags) > 0 {
		t.Fatalf("expected nil or empty flags, got %d flags", len(pm.Flags))
	}
}
