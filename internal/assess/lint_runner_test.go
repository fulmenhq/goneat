/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLintAssessmentRunner_verifyGolangciConfig(t *testing.T) {
	runner := NewLintAssessmentRunner()

	tests := []struct {
		name        string
		configFile  string
		expectError bool
	}{
		{
			name:        "no config file",
			configFile:  "",
			expectError: false, // No config file is OK
		},
		{
			name:        "valid config",
			configFile:  "../../tests/fixtures/golangci-config/valid/basic.yml",
			expectError: false,
		},
		{
			name:        "invalid config - version issue",
			configFile:  "../../tests/fixtures/golangci-config/invalid/version-issue.yml",
			expectError: true,
		},
		{
			name:        "invalid config - schema violation",
			configFile:  "../../tests/fixtures/golangci-config/invalid/schema-violation.yml",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory for testing
			tempDir, err := os.MkdirTemp("", "golangci-config-test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer func() {
				if err := os.RemoveAll(tempDir); err != nil {
					t.Logf("Failed to clean up temp dir %s: %v", tempDir, err)
				}
			}()

			// Copy the config file if specified
			if tt.configFile != "" {
				srcPath := tt.configFile
				destPath := filepath.Join(tempDir, ".golangci.yml")

				if _, err := os.Stat(srcPath); os.IsNotExist(err) {
					t.Skipf("Test fixture not found: %s", srcPath)
					return
				}

				srcContent, err := os.ReadFile(srcPath)
				if err != nil {
					t.Fatalf("Failed to read test fixture: %v", err)
				}

				if err := os.WriteFile(destPath, srcContent, 0644); err != nil {
					t.Fatalf("Failed to write test config: %v", err)
				}
			}

			// Run config verification
			err = runner.verifyGolangciConfig(tempDir)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestLintAssessmentRunner_Assess_WithInvalidConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	runner := NewLintAssessmentRunner()
	if !runner.IsAvailable() {
		t.Skip("golangci-lint not available in PATH")
	}

	// Create a temporary directory with invalid config
	tempDir, err := os.MkdirTemp("", "golangci-invalid-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to clean up temp dir %s: %v", tempDir, err)
		}
	}()

	// Copy invalid config
	invalidConfigPath := "../../tests/fixtures/golangci-config/invalid/version-issue.yml"
	if _, err := os.Stat(invalidConfigPath); os.IsNotExist(err) {
		t.Skipf("Test fixture not found: %s", invalidConfigPath)
		return
	}

	srcContent, err := os.ReadFile(invalidConfigPath)
	if err != nil {
		t.Fatalf("Failed to read test fixture: %v", err)
	}

	configPath := filepath.Join(tempDir, ".golangci.yml")
	if err := os.WriteFile(configPath, srcContent, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Create a simple Go file to lint
	goFile := filepath.Join(tempDir, "main.go")
	goContent := `package main

func main() {
	println("hello")
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test Go file: %v", err)
	}

	// Run assessment - should fail due to invalid config
	config := DefaultAssessmentConfig()
	config.Mode = AssessmentModeCheck

	result, err := runner.Assess(context.Background(), tempDir, config)
	if err != nil {
		t.Fatalf("Assess returned unexpected error: %v", err)
	}

	if result.Success {
		t.Errorf("Expected assessment to fail due to invalid config, but it succeeded")
	}

	if result.Error == "" {
		t.Errorf("Expected error message for invalid config, but got empty error")
	}

	// Check that the error mentions config validation
	if !containsString(result.Error, "config validation") {
		t.Errorf("Expected error to mention config validation, got: %s", result.Error)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsString(s[1:], substr) || (len(s) > 0 && s[:len(substr)] == substr))
}
