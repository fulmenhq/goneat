/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package dates

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultDatesConfig(t *testing.T) {
	config := DefaultDatesConfig()

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
		t.Errorf("Expected default max_skew '24h', got %s", config.Rules.FutureDates.MaxSkew)
	}

	if config.Rules.FutureDates.Severity != "error" {
		t.Errorf("Expected default severity 'error', got %s", config.Rules.FutureDates.Severity)
	}

	if config.Rules.FutureDates.AutoFix {
		t.Error("Expected default auto_fix false for future dates? Wait, it's true in default")
	}
}

func TestDatesRunner_Assess(t *testing.T) {
	tempDir := t.TempDir()

	testFiles := map[string]string{
		"CHANGELOG.md":           "## [v1.0.0] - 2025-12-31\n## [v0.9.0] - 2025-09-09",
		"README.md":              "Updated: 2025-12-31",
		"docs/releases/1.0.0.md": "Release date: 2025-12-31",
		"ignored.txt":            "Date: 2025-12-31",
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

	// Load the config from the temp directory (where we placed .goneat/dates.yaml)
	config := LoadDatesConfig(tempDir)
	runner := NewDatesRunnerWithConfig(config)
	ctx := context.Background()

	result, err := runner.Assess(ctx, tempDir, nil)
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}

	// The runner returns Success=true even when issues are found (issues are expected behavior)
	// Check that we found the expected future date issues
	if len(result.Issues) == 0 {
		t.Error("Expected to find future date issues")
	}

	// Verify we found future date issues for the test files
	futureIssuesFound := 0
	for _, issue := range result.Issues {
		if strings.Contains(issue.Message, "Future date found") {
			futureIssuesFound++
		}
	}

	if futureIssuesFound == 0 {
		t.Error("Expected to find future date issues in the test files")
	}
}

func TestDatesRunner_Assess_WithConfig(t *testing.T) {
	tempDir := t.TempDir()

	// Create .goneat/dates.yaml
	goneatDir := filepath.Join(tempDir, ".goneat")
	if err := os.MkdirAll(goneatDir, 0755); err != nil {
		t.Fatalf("Failed to create goneat directory: %v", err)
	}

	configContent := `# New schema example
enabled: true
files:
  include:
    - "CUSTOM_CHANGELOG.md"
    - "docs/custom/**"
  exclude:
    - "**/ignore/**"
date_patterns:
  - regex: "(\\d{4})-(\\d{2})-(\\d{2})"
    order: "YMD"
    description: "ISO format"
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
			t.Fatalf("Failed to write file %s: %v", filePath, err)
		}
	}

	// Load the config from the temp directory (where we placed .goneat/dates.yaml)
	config := LoadDatesConfig(tempDir)
	runner := NewDatesRunnerWithConfig(config)
	ctx := context.Background()

	result, err := runner.Assess(ctx, tempDir, nil)
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}

	foundCustom := false
	foundIgnored := false
	foundReadme := false

	t.Logf("Found %d issues", len(result.Issues))
	for _, issue := range result.Issues {
		t.Logf("Issue in file: %s", issue.File)
		if strings.Contains(issue.File, "CUSTOM") || strings.Contains(issue.File, "custom/release") {
			foundCustom = true
		}
		if strings.Contains(issue.File, "ignore") {
			foundIgnored = true
		}
		if strings.Contains(issue.File, "README") {
			foundReadme = true
		}
	}

	if !foundCustom {
		t.Error("Expected issues in custom files matching include patterns")
	}
	if foundIgnored {
		t.Error("Found issue in excluded file (ignore/file.md)")
	}
	if foundReadme {
		t.Error("Found issue in README.md which is not in include patterns")
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
rules:
  future_dates:
    enabled: false
  stale_entries:
    enabled: false
  monotonic_order:
    enabled: false
`

	configPath := filepath.Join(goneatDir, "dates.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create a test file with dates (should be ignored when disabled)
	testFile := filepath.Join(tempDir, "CHANGELOG.md")
	if err := os.WriteFile(testFile, []byte("## [v1.0.0] - 2025-12-31"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load the config from the temp directory (where we placed .goneat/dates.yaml)
	config := LoadDatesConfig(tempDir)
	runner := NewDatesRunnerWithConfig(config)
	ctx := context.Background()

	result, err := runner.Assess(ctx, tempDir, nil)
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success when disabled, got %v", result.Success)
	}

	if len(result.Issues) != 0 {
		t.Errorf("Expected no issues when disabled, got %d", len(result.Issues))
	}

	if enabled, ok := result.Metrics["enabled"].(bool); ok && enabled {
		t.Error("Expected enabled metric false")
	}
}

func TestParseDateParts(t *testing.T) {
	tests := []struct {
		yearStr, monthStr, dayStr, order         string
		expectedYear, expectedMonth, expectedDay int
	}{
		{"2025", "12", "31", "YMD", 2025, 12, 31},
		{"12", "31", "2025", "MDY", 2025, 12, 31},
		{"31", "12", "2025", "DMY", 2025, 12, 31},
	}

	for _, tt := range tests {
		t.Run(tt.order, func(t *testing.T) {
			y, m, d := ParseDateParts(tt.yearStr, tt.monthStr, tt.dayStr, tt.order)
			if y != tt.expectedYear || m != tt.expectedMonth || d != tt.expectedDay {
				t.Errorf("ParseDateParts(%q, %q, %q, %q) = %d,%d,%d; want %d,%d,%d", tt.yearStr, tt.monthStr, tt.dayStr, tt.order, y, m, d, tt.expectedYear, tt.expectedMonth, tt.expectedDay)
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"123", 123},
		{"0", 0},
		{"999", 999},
		{"abc", 0},
		{"12a34", 1234},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseInt(tt.input)
			if result != tt.expected {
				t.Errorf("parseInt(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoadDatesConfig(t *testing.T) {
	tempDir := t.TempDir()

	// No config - default
	config := LoadDatesConfig(tempDir)
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
    description: "ISO format"
rules:
  future_dates:
    enabled: false
    max_skew: "5d"
    severity: "high"
`

	yamlPath := filepath.Join(goneatDir, "dates.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlConfig), 0644); err != nil {
		t.Fatalf("Failed to write YAML config file: %v", err)
	}

	config = LoadDatesConfig(tempDir)
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

// TestExtractHeadingDates tests the heading date extraction
func TestExtractHeadingDates(t *testing.T) {
	content := `# Changelog

## [v0.2.3] - 2025-09-09
Some content

## [v0.2.2] - 2025-09-09
More content

## [v0.2.2-rc.4] - 2026-01-25
Out of order date

## [0.2.2-rc.4] - 2025-09-09
Another date

## [v0.2.1] - 2025-09-08
Older date
`

	entries := extractChangelogEntries(content, time.UTC)
	var dates []time.Time
	var headers []string
	for _, entry := range entries {
		if entry.Date != nil {
			dates = append(dates, *entry.Date)
			headers = append(headers, entry.Line)
		}
	}

	// Should extract 5 dates
	if len(dates) != 5 {
		t.Errorf("Expected 5 dates, got %d", len(dates))
	}

	// Check that dates are in the order they appear in the file
	expectedDates := []string{
		"2025-09-09", // v0.2.3
		"2025-09-09", // v0.2.2
		"2026-01-25", // v0.2.2-rc.4 (out of order)
		"2025-09-09", // 0.2.2-rc.4
		"2025-09-08", // v0.2.1
	}

	for i, expected := range expectedDates {
		if i < len(dates) {
			actual := dates[i].Format("2006-01-02")
			if actual != expected {
				t.Errorf("Date %d: expected %s, got %s", i, expected, actual)
			}
		}
	}

	// Check that headers are extracted
	if len(headers) != 5 {
		t.Errorf("Expected 5 headers, got %d", len(headers))
	}
}

// TestIsMonotonicDescending tests the monotonic order detection
func TestIsMonotonicDescending(t *testing.T) {
	tests := []struct {
		name     string
		dates    []time.Time
		expected bool
	}{
		{
			name:     "empty slice",
			dates:    []time.Time{},
			expected: true,
		},
		{
			name:     "single date",
			dates:    []time.Time{time.Date(2025, 9, 9, 0, 0, 0, 0, time.UTC)},
			expected: true,
		},
		{
			name: "descending order",
			dates: []time.Time{
				time.Date(2025, 9, 9, 0, 0, 0, 0, time.UTC),
				time.Date(2025, 9, 8, 0, 0, 0, 0, time.UTC),
				time.Date(2025, 9, 7, 0, 0, 0, 0, time.UTC),
			},
			expected: true,
		},
		{
			name: "ascending order (should fail)",
			dates: []time.Time{
				time.Date(2025, 9, 7, 0, 0, 0, 0, time.UTC),
				time.Date(2025, 9, 8, 0, 0, 0, 0, time.UTC),
				time.Date(2025, 9, 9, 0, 0, 0, 0, time.UTC),
			},
			expected: false,
		},
		{
			name: "mixed order (should fail)",
			dates: []time.Time{
				time.Date(2025, 9, 9, 0, 0, 0, 0, time.UTC),
				time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC), // Future date out of order
				time.Date(2025, 9, 8, 0, 0, 0, 0, time.UTC),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMonotonicDescending(tt.dates)
			if result != tt.expected {
				t.Errorf("isMonotonicDescending() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestExtractHeadingDatesFromFixture tests date extraction from changelog entries using test fixtures.
// This ensures the extractChangelogEntries function correctly parses H2 headings with dates.
func TestExtractHeadingDatesFromFixture(t *testing.T) {
	// Use a fixture instead of the real CHANGELOG.md for reliable testing
	fixturePath := "../../tests/fixtures/dates/valid_monotonic_changelog.md"
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("Could not read fixture file: %v", err)
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

	// Should extract 4 dates from the fixture
	if len(dates) != 4 {
		t.Errorf("Expected 4 dates, got %d", len(dates))
	}

	// Check that dates are in descending order
	expectedDates := []string{"2025-03-15", "2025-02-28", "2025-01-10", "2024-12-05"}
	for i, expected := range expectedDates {
		if i < len(dates) {
			actual := dates[i].Format("2006-01-02")
			if actual != expected {
				t.Errorf("Date %d: expected %s, got %s", i, expected, actual)
			}
		}
	}

	// Test monotonic order detection
	if len(dates) > 1 {
		isMonotonic := isMonotonicDescending(dates)
		if !isMonotonic {
			t.Error("Expected fixture dates to be in monotonic descending order")
		}
	}
}

// TestDatesValidationWithFixture tests file path matching and monotonic order validation using fixtures.
// Verifies that the MonotonicOrder.Files patterns work correctly and changelog parsing is reliable.
func TestDatesValidationWithFixture(t *testing.T) {
	// Use fixture instead of real CHANGELOG.md for reliable testing
	fixturePath := "../../tests/fixtures/dates/valid_monotonic_changelog.md"
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("Could not read fixture file: %v", err)
	}

	// Create a dates runner with the default config
	runner := NewDatesRunner()

	// Test the file path matching
	rel := "CHANGELOG.md"
	cfg := runner.config

	// Test matchesAny function
	matches := matchesAny(rel, cfg.Rules.MonotonicOrder.Files)
	if !matches {
		t.Error("File path matching failed - CHANGELOG.md should match the pattern")
	}

	// Test various file patterns against MonotonicOrder.Files patterns
	testCases := []struct {
		file     string
		expected bool
	}{
		{"CHANGELOG.md", true},
		{"changelog.md", true},
		{"CHANGELOG.txt", false}, // .txt doesn't match .md pattern
		{"HISTORY.md", true},
		{"NEWS.md", true},
		{"README.md", false},
		{"VERSION", false}, // VERSION doesn't match the changelog patterns
		{"some/deep/path/CHANGELOG.md", true},
		{"some/deep/path/changelog.md", true},
	}

	for _, tc := range testCases {
		result := matchesAny(tc.file, cfg.Rules.MonotonicOrder.Files)
		if result != tc.expected {
			t.Errorf("matchesAny(%q, monotonicFiles) = %v, expected %v", tc.file, result, tc.expected)
		}
	}

	// Test the monotonic order detection directly with fixture
	entries := extractChangelogEntries(string(content), time.UTC)
	var hd []time.Time
	for _, entry := range entries {
		if entry.Date != nil {
			hd = append(hd, *entry.Date)
		}
	}

	// Should extract 4 dates from the fixture
	if len(hd) != 4 {
		t.Errorf("Expected 4 dates from fixture, got %d", len(hd))
	}

	if len(hd) > 1 {
		isMonotonic := isMonotonicDescending(hd)
		if !isMonotonic {
			t.Error("Expected fixture dates to be in monotonic descending order")
		}
	}
}

// TestFileDiscovery tests the file discovery logic for dates validation.
// Ensures that include/exclude patterns work correctly and irrelevant directories are skipped.
func TestFileDiscovery(t *testing.T) {
	// Create a dates runner
	runner := NewDatesRunner()
	cfg := runner.config

	t.Logf("Include patterns: %v", cfg.Files.Include)
	t.Logf("Exclude patterns: %v", cfg.Files.Exclude)

	// Test the file discovery logic
	target := "../.."
	var files []string

	// Simulate the file discovery logic
	_ = filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			// Skip irrelevant directories to avoid excessive logging
			if d.Name() == ".gocache" || d.Name() == "node_modules" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(target, path)
		rel = filepath.ToSlash(rel)

		// Test includeFile logic
		if rel == "." || strings.HasPrefix(rel, ".git/") {
			return nil
		}
		if !matchInclude(rel, cfg.Files.Include) {
			return nil
		}
		if matchExclude(rel, cfg.Files.Exclude) {
			return nil
		}

		files = append(files, rel)
		return nil
	})

	t.Logf("Found %d files total", len(files))

	// Check if CHANGELOG.md was found
	foundChangelog := false
	for _, f := range files {
		if f == "CHANGELOG.md" {
			foundChangelog = true
			break
		}
	}

	if !foundChangelog {
		t.Error("CHANGELOG.md was not found in file discovery")
	}
}

// Helper
