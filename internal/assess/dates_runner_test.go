/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/internal/dates"
)

func TestDatesConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  dates.DatesConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: dates.DatesConfig{
				Enabled: true,
				Files: dates.Files{
					Include:          []string{"CHANGELOG.md", "docs/"},
					TextExtensions:   []string{".md", ".txt"},
					MaxFileSizeBytes: 4194304,
				},
				DatePatterns: []dates.DatePattern{
					{Regex: `(\d{4})-(\d{2})-(\d{2})`, Order: "YMD"},
				},
				Rules: dates.Rules{
					FutureDates: dates.FutureDates{
						Enabled:  true,
						MaxSkew:  "0h",
						Severity: "error",
						AutoFix:  false,
					},
					StaleEntries: dates.StaleEntries{
						Enabled:  true,
						WarnDays: 180,
						Severity: "warning",
					},
				},
				Output: dates.Output{
					FailOn: "error",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid severity",
			config: dates.DatesConfig{
				Enabled: true,
				Files: dates.Files{
					Include: []string{"CHANGELOG.md"},
				},
				DatePatterns: []dates.DatePattern{
					{Regex: `(\d{4})-(\d{2})-(\d{2})`, Order: "YMD"},
				},
				Rules: dates.Rules{
					FutureDates: dates.FutureDates{
						Severity: "invalid",
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid future_dates severity",
		},
		{
			name: "invalid max_skew",
			config: dates.DatesConfig{
				Enabled: true,
				Files: dates.Files{
					Include: []string{"CHANGELOG.md"},
				},
				DatePatterns: []dates.DatePattern{
					{Regex: `(\d{4})-(\d{2})-(\d{2})`, Order: "YMD"},
				},
				Rules: dates.Rules{
					FutureDates: dates.FutureDates{
						MaxSkew: "invalid",
					},
				},
			},
			wantErr: true,
			errMsg:  "max_skew",
		},
		{
			name: "invalid date pattern - no capture groups",
			config: dates.DatesConfig{
				Enabled: true,
				Files: dates.Files{
					Include: []string{"CHANGELOG.md"},
				},
				DatePatterns: []dates.DatePattern{
					{Regex: `\d{4}-\d{2}-\d{2}`}, // No capture groups
				},
			},
			wantErr: true,
			errMsg:  "3 capture groups",
		},
		{
			name: "empty include pattern",
			config: dates.DatesConfig{
				Enabled: true,
				Files: dates.Files{
					Include: []string{""},
				},
				DatePatterns: []dates.DatePattern{
					{Regex: `(\d{4})-(\d{2})-(\d{2})`, Order: "YMD"},
				},
			},
			wantErr: true,
			errMsg:  "include glob",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip - ValidateDatesConfig removed in favor of schema validation
			t.Skip("ValidateDatesConfig removed - now using schema validation in LoadDatesConfig")
		})
	}
}

func TestDefaultDatesConfig(t *testing.T) {
	config := dates.DefaultDatesConfig()

	if !config.Enabled {
		t.Error("Expected default config to be enabled")
	}

	if len(config.Files.Include) == 0 {
		t.Error("Expected default config to have include patterns")
	}

	if len(config.DatePatterns) == 0 {
		t.Error("Expected default config to have date patterns")
	}

	if config.Rules.FutureDates.MaxSkew != "24h" {
		t.Errorf("Expected default max_skew to be '24h', got %s", config.Rules.FutureDates.MaxSkew)
	}

	if config.Rules.FutureDates.Severity != "error" {
		t.Errorf("Expected default severity to be 'error', got %s", config.Rules.FutureDates.Severity)
	}

	if config.Rules.FutureDates.AutoFix {
		t.Error("Expected default auto_fix to be false for future dates")
	}
}

func TestDatesRunner_Assess(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"CHANGELOG.md":           "## [v1.0.0] - 2025-12-31\n## [v0.9.0] - 2025-09-09",
		"README.md":              "Updated: 2025-12-31",
		"docs/releases/1.0.0.md": "Release date: 2025-12-31",
		"ignored.txt":            "Date: 2025-12-31", // Should be ignored if excluded
	}

	for file, content := range testFiles {
		filePath := filepath.Join(tempDir, file)
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}
	}

	runner := NewDatesAssessmentRunner()
	ctx := context.Background()
	config := AssessmentConfig{Mode: AssessmentModeCheck}

	result, err := runner.Assess(ctx, tempDir, config)
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}

	// The dates runner returns Success=true even when issues are found (issues are expected output)
	if !result.Success {
		t.Errorf("Expected assessment to succeed even with issues, got success = %v", result.Success)
	}

	if len(result.Issues) == 0 {
		t.Error("Expected to find future date issues")
	}

	// Check ignored files not processed (if config excludes)
}

func TestDatesRunner_Assess_WithConfig(t *testing.T) {
	// Similar to original, but using new config structure
	tempDir := t.TempDir()

	// Create .goneat/dates.yaml with new schema
	goneatDir := filepath.Join(tempDir, ".goneat")
	if err := os.MkdirAll(goneatDir, 0755); err != nil {
		t.Fatalf("Failed to create goneat directory: %v", err)
	}

	configContent := `# New schema example
enabled: true
files:
  include:
    - "CUSTOM_CHANGELOG.md"
    - "docs/custom/"
  exclude:
    - "**/ignore/**"
date_patterns:
  - regex: "(\\d{4})-(\\d{2})-(\\d{2})"
    order: "YMD"
rules:
  future_dates:
    enabled: true
    max_skew: "0h"
    severity: "high"
`

	configPath := filepath.Join(goneatDir, "dates.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create test files
	testFiles := map[string]string{
		"CUSTOM_CHANGELOG.md":    "## [v1.0.0] - 2025-12-31",
		"docs/custom/release.md": "Release: 2025-12-31",
		"ignore/file.md":         "Date: 2025-12-31",
		"README.md":              "Date: 2025-12-31",
	}

	for file, content := range testFiles {
		filePath := filepath.Join(tempDir, file)
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}
	}

	runner := NewDatesAssessmentRunner()
	ctx := context.Background()
	config := AssessmentConfig{Mode: AssessmentModeCheck}

	result, err := runner.Assess(ctx, tempDir, config)
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}

	// Should find issues in custom files
	foundCustom := false
	for _, issue := range result.Issues {
		if strings.Contains(issue.File, "CUSTOM") || strings.Contains(issue.File, "custom/release") {
			foundCustom = true
		}
	}
	if !foundCustom {
		t.Error("Expected issues in custom files")
	}
}

func TestDatesRunner_Assess_Disabled(t *testing.T) {
	tempDir := t.TempDir()

	goneatDir := filepath.Join(tempDir, ".goneat")
	if err := os.MkdirAll(goneatDir, 0755); err != nil {
		t.Fatalf("Failed to create goneat directory: %v", err)
	}

	configContent := `enabled: false
files:
	 include:
	   - "CHANGELOG.md"
`

	configPath := filepath.Join(goneatDir, "dates.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	runner := NewDatesAssessmentRunner()
	ctx := context.Background()
	config := AssessmentConfig{Mode: AssessmentModeCheck}

	result, err := runner.Assess(ctx, tempDir, config)
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success when disabled, got %v", result.Success)
	}

	if len(result.Issues) != 0 {
		t.Errorf("Expected no issues when disabled, got %d", len(result.Issues))
	}
}

// TestDatesRunner_IsTextFile removed - function not implemented
// TODO: Implement dates.IsTextFile() utility function if needed

func TestLoadDatesConfig(t *testing.T) {
	tempDir := t.TempDir()

	// No config - default
	config := dates.LoadDatesConfig(tempDir)
	if !config.Enabled {
		t.Error("Expected default enabled")
	}

	// YAML config
	goneatDir := filepath.Join(tempDir, ".goneat")
	if err := os.MkdirAll(goneatDir, 0755); err != nil {
		t.Fatalf("Failed to create goneat directory: %v", err)
	}

	yamlConfig := `enabled: false
files:
  include:
    - "CUSTOM.md"
date_patterns:
  - regex: "(\\d{4})-(\\d{2})-(\\d{2})"
    order: "YMD"
rules:
  future_dates:
    max_skew: "5d"
    severity: "high"
`

	yamlPath := filepath.Join(goneatDir, "dates.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlConfig), 0644); err != nil {
		t.Fatalf("Failed to write yaml config file: %v", err)
	}

	config = dates.LoadDatesConfig(tempDir)
	if config.Enabled {
		t.Error("Expected disabled")
	}
	if len(config.Files.Include) != 1 || config.Files.Include[0] != "CUSTOM.md" {
		t.Errorf("Expected include [CUSTOM.md], got %v", config.Files.Include)
	}
	if config.Rules.FutureDates.MaxSkew != "5d" {
		t.Errorf("Expected max_skew '5d', got %s", config.Rules.FutureDates.MaxSkew)
	}
	if config.Rules.FutureDates.Severity != "high" {
		t.Errorf("Expected severity 'high', got %s", config.Rules.FutureDates.Severity)
	}
}
