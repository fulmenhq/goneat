/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package dates

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFixtures_MonotonicOrder tests all the fixtures we created
func TestFixtures_MonotonicOrder(t *testing.T) {
	fixtureDir := "../../tests/fixtures/dates"

	tests := []struct {
		name          string
		filename      string
		expectIssues  bool
		issueContains string
	}{
		{
			name:          "valid monotonic changelog",
			filename:      "valid_monotonic_changelog.md",
			expectIssues:  false,
			issueContains: "",
		},
		{
			name:          "invalid monotonic changelog",
			filename:      "invalid_monotonic_changelog.md",
			expectIssues:  true,
			issueContains: "monotonic",
		},
		{
			name:          "complex monotonic violation",
			filename:      "complex_monotonic_violation.md",
			expectIssues:  false, // File doesn't match default monotonic patterns (not CHANGELOG*.md, etc.)
			issueContains: "monotonic",
		},
		{
			name:          "single release changelog",
			filename:      "single_release_changelog.md",
			expectIssues:  false,
			issueContains: "",
		},
		{
			name:          "no dates changelog",
			filename:      "no_dates_changelog.md",
			expectIssues:  false,
			issueContains: "",
		},
		{
			name:          "alternative heading format",
			filename:      "alternative_heading_format.md",
			expectIssues:  false, // File doesn't match default monotonic patterns
			issueContains: "monotonic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with default config (monotonic disabled)
			t.Run("default_config", func(t *testing.T) {
				runner := NewDatesRunner()
				filePath := filepath.Join(fixtureDir, tt.filename)

				result, err := runner.Assess(context.Background(), filePath, nil)
				if err != nil {
					t.Fatalf("Assess failed: %v", err)
				}

				// With default config, check based on test expectations
				foundViolation := false
				for _, issue := range result.Issues {
					if strings.Contains(issue.Message, "not in descending date order") {
						foundViolation = true
					}
				}

				if tt.expectIssues && !foundViolation {
					t.Errorf("Expected to find monotonic violation but didn't")
				} else if !tt.expectIssues && foundViolation {
					t.Errorf("Did not expect monotonic violation but found one")
				}
			})

			// Test with enabled config
			t.Run("enabled_config", func(t *testing.T) {
				// Create temp dir with enabled config
				tempDir := t.TempDir()
				goneatDir := filepath.Join(tempDir, ".goneat")
				if err := os.MkdirAll(goneatDir, 0755); err != nil {
					t.Fatalf("Failed to create goneat dir: %v", err)
				}

				// Copy the enabled config
				configSrc := filepath.Join(fixtureDir, "enabled_dates_config.yaml")
				configDst := filepath.Join(goneatDir, "dates.yaml")

				configData, err := os.ReadFile(configSrc)
				if err != nil {
					t.Fatalf("Failed to read config: %v", err)
				}

				if err := os.WriteFile(configDst, configData, 0644); err != nil {
					t.Fatalf("Failed to write config: %v", err)
				}

				// Copy the test file
				srcFile := filepath.Join(fixtureDir, tt.filename)
				dstFile := filepath.Join(tempDir, tt.filename)

				fileData, err := os.ReadFile(srcFile)
				if err != nil {
					t.Fatalf("Failed to read fixture: %v", err)
				}

				if err := os.WriteFile(dstFile, fileData, 0644); err != nil {
					t.Fatalf("Failed to write fixture: %v", err)
				}

				// Run assessment on temp dir with loaded config
				config := LoadDatesConfig(tempDir)
				runner := NewDatesRunnerWithConfig(config)
				result, err := runner.Assess(context.Background(), tempDir, nil)
				if err != nil {
					t.Fatalf("Assess failed: %v", err)
				}

				// Check for monotonic violations (not just informational messages)
				foundViolation := false
				for _, issue := range result.Issues {
					if strings.Contains(issue.Message, "not in descending date order") {
						foundViolation = true
						t.Logf("Found monotonic violation: %s", issue.Message)
					}
				}

				if tt.expectIssues && !foundViolation {
					t.Errorf("Expected to find monotonic violation but didn't")
				} else if !tt.expectIssues && foundViolation {
					t.Errorf("Did not expect monotonic violation but found one")
				}
			})
		})
	}
}

// TestFixtures_ExtractHeadingDates tests heading extraction on fixtures
func TestFixtures_ExtractHeadingDates(t *testing.T) {
	fixtureDir := "../../tests/fixtures/dates"

	tests := []struct {
		name              string
		filename          string
		expectedDateCount int
		shouldBeMonotonic bool
	}{
		{
			name:              "valid monotonic changelog",
			filename:          "valid_monotonic_changelog.md",
			expectedDateCount: 4, // v1.2.3, v1.2.2, v1.2.1, v1.2.0
			shouldBeMonotonic: true,
		},
		{
			name:              "invalid monotonic changelog",
			filename:          "invalid_monotonic_changelog.md",
			expectedDateCount: 4,     // v1.2.3, v1.2.2, v1.2.1, v1.2.0
			shouldBeMonotonic: false, // v1.2.2 is newer than v1.2.3
		},
		{
			name:              "complex monotonic violation",
			filename:          "complex_monotonic_violation.md",
			expectedDateCount: 4,     // v2.0.0, v1.9.8, v1.9.7, v1.9.6
			shouldBeMonotonic: false, // multiple violations
		},
		{
			name:              "alternative heading format",
			filename:          "alternative_heading_format.md",
			expectedDateCount: 4,     // v3.1.0, v3.0.2, v3.0.1, v3.0.0
			shouldBeMonotonic: false, // v3.0.1 is newer than v3.0.2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(fixtureDir, tt.filename)
			content, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("Failed to read fixture: %v", err)
			}

			entries := extractChangelogEntries(string(content), time.UTC)
			var dates []time.Time
			var headers []string
			for _, entry := range entries {
				if entry.Date != nil {
					dates = append(dates, *entry.Date)
					headers = append(headers, entry.Line)
				}
			}

			if len(dates) != tt.expectedDateCount {
				t.Errorf("Expected %d dates, got %d", tt.expectedDateCount, len(dates))
			}

			if len(headers) != len(dates) {
				t.Errorf("Expected %d headers, got %d", len(dates), len(headers))
			}

			// Log the dates for debugging
			for i, date := range dates {
				t.Logf("Date %d: %s", i, date.Format("2006-01-02"))
			}

			// Test monotonic order
			if len(dates) > 1 {
				isMonotonic := isMonotonicDescending(dates)
				if isMonotonic != tt.shouldBeMonotonic {
					t.Errorf("Expected monotonic=%v, got %v", tt.shouldBeMonotonic, isMonotonic)
				}
			}
		})
	}
}

// TestFixtures_DefaultVsEnabledConfig tests the key difference between default and enabled configs
func TestFixtures_DefaultVsEnabledConfig(t *testing.T) {
	fixtureDir := "../../tests/fixtures/dates"
	testFile := "invalid_monotonic_changelog.md"

	// Test 1: Default config should NOW detect monotonic issues (after our fix)
	t.Run("default_config_now_finds_monotonic", func(t *testing.T) {
		cfg := DefaultDatesConfig()
		if !cfg.Rules.MonotonicOrder.Enabled {
			t.Error("Default config should now have MonotonicOrder.Enabled=true (after our fix)")
		}

		// Create temp dir and copy test file for directory-based assessment
		tempDir := t.TempDir()

		// Copy the test file
		srcFile := filepath.Join(fixtureDir, testFile)
		dstFile := filepath.Join(tempDir, "CHANGELOG.md") // Use standard name

		fileData, err := os.ReadFile(srcFile)
		if err != nil {
			t.Fatalf("Failed to read fixture: %v", err)
		}

		if err := os.WriteFile(dstFile, fileData, 0644); err != nil {
			t.Fatalf("Failed to write fixture: %v", err)
		}

		runner := NewDatesRunner()
		result, err := runner.Assess(context.Background(), tempDir, nil)
		if err != nil {
			t.Fatalf("Assess failed: %v", err)
		}

		// Should now find monotonic issues with default config
		foundViolation := false
		for _, issue := range result.Issues {
			t.Logf("Default config issue: Severity=%s, Message=%s", issue.Severity, issue.Message)
			if strings.Contains(issue.Message, "not in descending date order") {
				foundViolation = true
			}
		}

		if !foundViolation {
			t.Error("Expected to find monotonic violation with default config (after our fix)")
		}
	})

	// Test 2: Enabled config SHOULD detect monotonic issues
	t.Run("enabled_config_finds_monotonic", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create .goneat dir with enabled config
		goneatDir := filepath.Join(tempDir, ".goneat")
		if err := os.MkdirAll(goneatDir, 0755); err != nil {
			t.Fatalf("Failed to create goneat dir: %v", err)
		}

		// Copy enabled config
		enabledConfigPath := filepath.Join(fixtureDir, "enabled_dates_config.yaml")
		dstConfigPath := filepath.Join(goneatDir, "dates.yaml")

		configData, err := os.ReadFile(enabledConfigPath)
		if err != nil {
			t.Fatalf("Failed to read enabled config: %v", err)
		}

		if err := os.WriteFile(dstConfigPath, configData, 0644); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}

		// Copy test file
		srcFile := filepath.Join(fixtureDir, testFile)
		dstFile := filepath.Join(tempDir, testFile)

		fileData, err := os.ReadFile(srcFile)
		if err != nil {
			t.Fatalf("Failed to read fixture: %v", err)
		}

		if err := os.WriteFile(dstFile, fileData, 0644); err != nil {
			t.Fatalf("Failed to write fixture: %v", err)
		}

		// Test config loading
		loadedCfg := LoadDatesConfig(tempDir)
		if !loadedCfg.Rules.MonotonicOrder.Enabled {
			t.Error("Loaded config should have MonotonicOrder.Enabled=true")
		}

		// Run assessment
		runner := NewDatesRunner()
		result, err := runner.Assess(context.Background(), tempDir, nil)
		if err != nil {
			t.Fatalf("Assess failed: %v", err)
		}

		// Should find monotonic issues
		foundMonotonic := false
		foundViolation := false
		for _, issue := range result.Issues {
			t.Logf("Issue: Severity=%s, Message=%s", issue.Severity, issue.Message)
			if strings.Contains(strings.ToLower(issue.Message), "monotonic") {
				foundMonotonic = true
			}
			if strings.Contains(issue.Message, "not in descending date order") {
				foundViolation = true
			}
		}

		if !foundMonotonic {
			t.Error("Expected to find monotonic issue with enabled config")
		}
		if !foundViolation {
			t.Error("Expected to find actual monotonic violation (not just scan message)")
		}
	})
}
