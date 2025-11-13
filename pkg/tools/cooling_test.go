package tools

import (
	"os"
	"testing"
)

// TestMergeCoolingConfig tests the cooling configuration inheritance logic
func TestMergeCoolingConfig(t *testing.T) {
	tests := []struct {
		name         string
		global       *CoolingConfig
		toolSpecific *CoolingConfig
		expected     *CoolingConfig
	}{
		{
			name:         "nil_tool_uses_global",
			global:       &CoolingConfig{Enabled: true, MinAgeDays: 7, MinDownloads: 100},
			toolSpecific: nil,
			expected:     &CoolingConfig{Enabled: true, MinAgeDays: 7, MinDownloads: 100},
		},
		{
			name:         "nil_both_uses_defaults",
			global:       nil,
			toolSpecific: nil,
			expected:     &CoolingConfig{Enabled: true, MinAgeDays: 7, MinDownloads: 100, MinDownloadsRecent: 10, AlertOnly: false, GracePeriodDays: 3},
		},
		{
			name:         "tool_overrides_numeric_fields",
			global:       &CoolingConfig{Enabled: true, MinAgeDays: 7, MinDownloads: 100},
			toolSpecific: &CoolingConfig{Enabled: true, MinAgeDays: 14, MinDownloads: 5000},
			expected:     &CoolingConfig{Enabled: true, MinAgeDays: 14, MinDownloads: 5000},
		},
		{
			name:         "tool_disables_cooling",
			global:       &CoolingConfig{Enabled: true, MinAgeDays: 7},
			toolSpecific: &CoolingConfig{Enabled: false},
			expected:     &CoolingConfig{Enabled: false, MinAgeDays: 7},
		},
		{
			name:         "tool_zero_values_preserve_global",
			global:       &CoolingConfig{Enabled: true, MinAgeDays: 7, MinDownloads: 100, MinDownloadsRecent: 10},
			toolSpecific: &CoolingConfig{Enabled: true, MinAgeDays: 14}, // Only override min_age_days
			expected:     &CoolingConfig{Enabled: true, MinAgeDays: 14, MinDownloads: 100, MinDownloadsRecent: 10},
		},
		{
			name:   "tool_exceptions_override",
			global: &CoolingConfig{Enabled: true, Exceptions: []CoolingException{{Pattern: "github.com/fulmenhq/*", Reason: "Global trust"}}},
			toolSpecific: &CoolingConfig{
				Enabled:    true,
				Exceptions: []CoolingException{{Pattern: "github.com/anchore/*", Reason: "Tool-specific trust"}},
			},
			expected: &CoolingConfig{
				Enabled:    true,
				Exceptions: []CoolingException{{Pattern: "github.com/anchore/*", Reason: "Tool-specific trust"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeCoolingConfig(tt.global, tt.toolSpecific)

			if result.Enabled != tt.expected.Enabled {
				t.Errorf("Enabled: got %v, want %v", result.Enabled, tt.expected.Enabled)
			}
			if result.MinAgeDays != tt.expected.MinAgeDays {
				t.Errorf("MinAgeDays: got %d, want %d", result.MinAgeDays, tt.expected.MinAgeDays)
			}
			if result.MinDownloads != tt.expected.MinDownloads {
				t.Errorf("MinDownloads: got %d, want %d", result.MinDownloads, tt.expected.MinDownloads)
			}
			if result.MinDownloadsRecent != tt.expected.MinDownloadsRecent {
				t.Errorf("MinDownloadsRecent: got %d, want %d", result.MinDownloadsRecent, tt.expected.MinDownloadsRecent)
			}
			if result.AlertOnly != tt.expected.AlertOnly {
				t.Errorf("AlertOnly: got %v, want %v", result.AlertOnly, tt.expected.AlertOnly)
			}
			if result.GracePeriodDays != tt.expected.GracePeriodDays {
				t.Errorf("GracePeriodDays: got %d, want %d", result.GracePeriodDays, tt.expected.GracePeriodDays)
			}
			if len(result.Exceptions) != len(tt.expected.Exceptions) {
				t.Errorf("Exceptions length: got %d, want %d", len(result.Exceptions), len(tt.expected.Exceptions))
			}
		})
	}
}

// TestLoadGlobalCoolingConfig tests loading global cooling config from dependencies.yaml
func TestLoadGlobalCoolingConfig(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "goneat-cooling-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("failed to remove temp dir: %v", err)
		}
	}()

	// Change to temp directory for test
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// Test case 1: No dependencies.yaml exists
	t.Run("no_file_returns_nil", func(t *testing.T) {
		config, err := LoadGlobalCoolingConfig()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if config != nil {
			t.Errorf("expected nil config when file doesn't exist, got %v", config)
		}
	})

	// Test case 2: Valid dependencies.yaml with cooling section
	t.Run("valid_cooling_config", func(t *testing.T) {
		// Create .goneat directory
		if err := os.MkdirAll(".goneat", 0755); err != nil {
			t.Fatalf("failed to create .goneat: %v", err)
		}
		defer func() {
			if err := os.RemoveAll(".goneat"); err != nil {
				t.Errorf("failed to remove .goneat: %v", err)
			}
		}()

		// Write dependencies.yaml
		configYAML := `version: v1
cooling:
  enabled: true
  min_age_days: 14
  min_downloads: 1000
  min_downloads_recent: 50
  alert_only: false
  grace_period_days: 7
  exceptions:
    - pattern: "github.com/fulmenhq/*"
      reason: "Internal packages"
      approved_by: "@3leapsdave"
`
		if err := os.WriteFile(".goneat/dependencies.yaml", []byte(configYAML), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		config, err := LoadGlobalCoolingConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config == nil {
			t.Fatal("expected config, got nil")
		}

		if !config.Enabled {
			t.Error("expected Enabled=true")
		}
		if config.MinAgeDays != 14 {
			t.Errorf("MinAgeDays: got %d, want 14", config.MinAgeDays)
		}
		if config.MinDownloads != 1000 {
			t.Errorf("MinDownloads: got %d, want 1000", config.MinDownloads)
		}
		if config.MinDownloadsRecent != 50 {
			t.Errorf("MinDownloadsRecent: got %d, want 50", config.MinDownloadsRecent)
		}
		if config.GracePeriodDays != 7 {
			t.Errorf("GracePeriodDays: got %d, want 7", config.GracePeriodDays)
		}
		if len(config.Exceptions) != 1 {
			t.Errorf("expected 1 exception, got %d", len(config.Exceptions))
		} else {
			if config.Exceptions[0].Pattern != "github.com/fulmenhq/*" {
				t.Errorf("exception pattern: got %s, want github.com/fulmenhq/*", config.Exceptions[0].Pattern)
			}
		}
	})
}

// TestToolGetEffectiveCoolingConfig tests the Tool.GetEffectiveCoolingConfig method
func TestToolGetEffectiveCoolingConfig(t *testing.T) {
	// Create temp dir for test
	tmpDir, err := os.MkdirTemp("", "goneat-effective-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("failed to remove temp dir: %v", err)
		}
	}()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	// Setup global config
	if err := os.MkdirAll(".goneat", 0755); err != nil {
		t.Fatalf("failed to create .goneat: %v", err)
	}

	globalCooling := `version: v1
cooling:
  enabled: true
  min_age_days: 7
  min_downloads: 100
`
	if err := os.WriteFile(".goneat/dependencies.yaml", []byte(globalCooling), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	tests := []struct {
		name            string
		tool            Tool
		disableCooling  bool
		expectEnabled   bool
		expectMinAge    int
		expectDownloads int
	}{
		{
			name:            "no_override_uses_global",
			tool:            Tool{Name: "syft", Cooling: nil},
			disableCooling:  false,
			expectEnabled:   true,
			expectMinAge:    7,
			expectDownloads: 100,
		},
		{
			name: "tool_override_stricter",
			tool: Tool{
				Name:    "syft",
				Cooling: &CoolingConfig{Enabled: true, MinAgeDays: 14, MinDownloads: 5000},
			},
			disableCooling:  false,
			expectEnabled:   true,
			expectMinAge:    14,
			expectDownloads: 5000,
		},
		{
			name:            "disable_flag_overrides_all",
			tool:            Tool{Name: "syft", Cooling: &CoolingConfig{Enabled: true, MinAgeDays: 14}},
			disableCooling:  true,
			expectEnabled:   false,
			expectMinAge:    0,
			expectDownloads: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := tt.tool.GetEffectiveCoolingConfig(tt.disableCooling)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if config.Enabled != tt.expectEnabled {
				t.Errorf("Enabled: got %v, want %v", config.Enabled, tt.expectEnabled)
			}
			if tt.expectEnabled { // Only check values if enabled
				if config.MinAgeDays != tt.expectMinAge {
					t.Errorf("MinAgeDays: got %d, want %d", config.MinAgeDays, tt.expectMinAge)
				}
				if config.MinDownloads != tt.expectDownloads {
					t.Errorf("MinDownloads: got %d, want %d", config.MinDownloads, tt.expectDownloads)
				}
			}
		})
	}
}

// TestCoolingConfigParsing tests that cooling config can be parsed from YAML
func TestCoolingConfigParsing(t *testing.T) {
	configYAML := `scopes:
  test:
    description: "Test scope"
    tools: ["syft"]
tools:
  syft:
    name: "syft"
    description: "SBOM tool"
    kind: "system"
    detect_command: "syft version"
    cooling:
      enabled: true
      min_age_days: 14
      min_downloads: 5000
      min_downloads_recent: 100
      alert_only: false
      grace_period_days: 7
      exceptions:
        - pattern: "github.com/anchore/*"
          reason: "Trusted SBOM vendor"
          approved_by: "@security-team"
`

	cfg, err := ParseConfig([]byte(configYAML))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	tool, ok := cfg.Tools["syft"]
	if !ok {
		t.Fatal("syft tool not found")
	}

	if tool.Cooling == nil {
		t.Fatal("expected cooling config, got nil")
	}

	c := tool.Cooling
	if !c.Enabled {
		t.Error("expected Enabled=true")
	}
	if c.MinAgeDays != 14 {
		t.Errorf("MinAgeDays: got %d, want 14", c.MinAgeDays)
	}
	if c.MinDownloads != 5000 {
		t.Errorf("MinDownloads: got %d, want 5000", c.MinDownloads)
	}
	if c.MinDownloadsRecent != 100 {
		t.Errorf("MinDownloadsRecent: got %d, want 100", c.MinDownloadsRecent)
	}
	if c.GracePeriodDays != 7 {
		t.Errorf("GracePeriodDays: got %d, want 7", c.GracePeriodDays)
	}
	if len(c.Exceptions) != 1 {
		t.Fatalf("expected 1 exception, got %d", len(c.Exceptions))
	}
	if c.Exceptions[0].Pattern != "github.com/anchore/*" {
		t.Errorf("exception pattern: got %s, want github.com/anchore/*", c.Exceptions[0].Pattern)
	}
}

// TestCoolingConfigSchemaValidation tests that cooling config validates correctly
func TestCoolingConfigSchemaValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_cooling_config",
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
    cooling:
      enabled: true
      min_age_days: 14
      min_downloads: 5000`,
			wantErr: false,
		},
		{
			name: "cooling_config_all_fields",
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
    cooling:
      enabled: true
      min_age_days: 14
      min_downloads: 5000
      min_downloads_recent: 100
      alert_only: false
      grace_period_days: 7
      exceptions:
        - pattern: "github.com/org/*"
          reason: "Trusted org"
          approved_by: "@team"`,
			wantErr: false,
		},
		{
			name: "cooling_disabled",
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
    cooling:
      enabled: false`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBytes([]byte(tt.config))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected validation error, got nil")
				}
				// Error validation would check err.Error() contains tt.errMsg
				// but we're keeping tests simple here
			} else {
				if err != nil {
					t.Fatalf("unexpected validation error: %v", err)
				}
			}
		})
	}
}

// Note: Using simple string contains check to avoid redeclaration with locator_test.go
