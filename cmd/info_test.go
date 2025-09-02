/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// execInfoLicenses executes the info licenses subcommand with given args and captures output
func execInfoLicenses(t *testing.T, args []string) (string, error) {
	t.Helper()

	// Execute via the real root command path: goneat info licenses ...
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootArgs := append([]string{"info", "licenses"}, args...)
	rootCmd.SetArgs(rootArgs)

	err := rootCmd.Execute()
	return buf.String(), err
}

// TestInfoLicenses_BasicExecution tests that the command executes without errors
func TestInfoLicenses_BasicExecution(t *testing.T) {
	out, err := execInfoLicenses(t, []string{})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify basic output structure
	if !strings.Contains(out, "Goneat License Information") {
		t.Errorf("expected license header in output, got: %s", out)
	}

	if !strings.Contains(out, "Main Project:") {
		t.Errorf("expected main project section, got: %s", out)
	}

	if !strings.Contains(out, "github.com/3leaps/goneat") {
		t.Errorf("expected main project module, got: %s", out)
	}
}

// TestInfoLicenses_SummaryFlag tests the --summary flag
func TestInfoLicenses_SummaryFlag(t *testing.T) {
	out, err := execInfoLicenses(t, []string{"--summary"})
	if err != nil {
		t.Fatalf("expected no error with --summary, got: %v", err)
	}

	// Verify summary output structure
	if !strings.Contains(out, "License Summary") {
		t.Errorf("expected summary header, got: %s", out)
	}

	if !strings.Contains(out, "Total modules:") {
		t.Errorf("expected total count, got: %s", out)
	}

	// Should contain license types
	expectedLicenses := []string{"MIT License", "Apache License 2.0", "BSD-3-Clause License"}
	for _, license := range expectedLicenses {
		if !strings.Contains(out, license) {
			t.Errorf("expected license type %s in summary, got: %s", license, out)
		}
	}
}

// TestInfoLicenses_FilterFlag tests the --filter flag
func TestInfoLicenses_FilterFlag(t *testing.T) {
	// Test MIT filter - this returns summary format when filtered
	out, err := execInfoLicenses(t, []string{"--filter", "mit"})
	if err != nil {
		t.Fatalf("expected no error with --filter mit, got: %v", err)
	}

	// Should contain MIT licenses but not others
	if !strings.Contains(out, "MIT License") {
		t.Errorf("expected MIT License in filtered output, got: %s", out)
	}

	// Should not contain other license types
	if strings.Contains(out, "Apache License 2.0") {
		t.Errorf("expected Apache License to be filtered out, got: %s", out)
	}

	if strings.Contains(out, "BSD-3-Clause License") {
		t.Errorf("expected BSD License to be filtered out, got: %s", out)
	}

	// Should show count of MIT modules
	if !strings.Contains(out, "3 modules") {
		t.Errorf("expected module count in filtered output, got: %s", out)
	}
}

// TestInfoLicenses_FilterApache tests Apache license filtering
func TestInfoLicenses_FilterApache(t *testing.T) {
	out, err := execInfoLicenses(t, []string{"--filter", "apache"})
	if err != nil {
		t.Fatalf("expected no error with --filter apache, got: %v", err)
	}

	// Should contain Apache licenses
	if !strings.Contains(out, "Apache License 2.0") {
		t.Errorf("expected Apache License in filtered output, got: %s", out)
	}

	// Should not contain other license types
	if strings.Contains(out, "MIT License") {
		t.Errorf("expected MIT License to be filtered out, got: %s", out)
	}

	// Should show count of Apache modules
	if !strings.Contains(out, "2 modules") {
		t.Errorf("expected module count in filtered output, got: %s", out)
	}
}

// TestInfoLicenses_JSONFlag tests the --json flag
func TestInfoLicenses_JSONFlag(t *testing.T) {
	out, err := execInfoLicenses(t, []string{"--json"})
	if err != nil {
		t.Fatalf("expected no error with --json, got: %v", err)
	}

	// Should be valid JSON (starts with [ for array)
	if !strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Errorf("expected JSON array output, got: %s", out)
	}

	// The JSON output is actually summary format, so check for summary fields
	if !strings.Contains(out, `"license_type"`) {
		t.Errorf("expected license_type field in JSON, got: %s", out)
	}

	if !strings.Contains(out, `"count"`) {
		t.Errorf("expected count field in JSON, got: %s", out)
	}

	if !strings.Contains(out, `"modules"`) {
		t.Errorf("expected modules field in JSON, got: %s", out)
	}
}

// TestInfoLicenses_JSONSummary tests --json with --summary
func TestInfoLicenses_JSONSummary(t *testing.T) {
	out, err := execInfoLicenses(t, []string{"--json", "--summary"})
	if err != nil {
		t.Fatalf("expected no error with --json --summary, got: %v", err)
	}

	// Should be valid JSON array
	if !strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Errorf("expected JSON array output, got: %s", out)
	}

	// Should contain summary structure
	if !strings.Contains(out, `"license_type"`) {
		t.Errorf("expected license_type field in JSON summary, got: %s", out)
	}

	if !strings.Contains(out, `"count"`) {
		t.Errorf("expected count field in JSON summary, got: %s", out)
	}

	if !strings.Contains(out, `"modules"`) {
		t.Errorf("expected modules field in JSON summary, got: %s", out)
	}
}

// TestInfoLicenses_CommandStructure tests that the command is properly registered
func TestInfoLicenses_CommandStructure(t *testing.T) {
	// Verify the info command exists and has the licenses subcommand
	if infoCmd == nil {
		t.Fatal("info command should not be nil")
	}

	if infoLicensesCmd == nil {
		t.Fatal("infoLicensesCmd should not be nil")
	}

	// Check command names
	if infoCmd.Use != "info" {
		t.Errorf("expected info command use to be 'info', got: %s", infoCmd.Use)
	}

	if infoLicensesCmd.Use != "licenses" {
		t.Errorf("expected licenses subcommand use to be 'licenses', got: %s", infoLicensesCmd.Use)
	}
}

// TestInfoLicenses_DataIntegrity tests that license data is consistent
func TestInfoLicenses_DataIntegrity(t *testing.T) {
	licenses := getLicenseInfo()

	// Should have at least one license
	if len(licenses) == 0 {
		t.Fatal("expected at least one license in data")
	}

	// Should have exactly one main project
	mainCount := 0
	for _, lic := range licenses {
		if lic.Main {
			mainCount++
		}
	}

	if mainCount != 1 {
		t.Errorf("expected exactly one main project, got: %d", mainCount)
	}

	// All licenses should have non-empty module and license fields
	for i, lic := range licenses {
		if lic.Module == "" {
			t.Errorf("license %d has empty module name", i)
		}
		if lic.License == "" {
			t.Errorf("license %d has empty license name", i)
		}
	}
}

// TestInfoLicenses_GroupByLicense tests the license grouping function
func TestInfoLicenses_GroupByLicense(t *testing.T) {
	licenses := getLicenseInfo()
	grouped := groupByLicense(licenses)

	// Should have multiple license types
	if len(grouped) < 2 {
		t.Errorf("expected at least 2 license types, got: %d", len(grouped))
	}

	// Each group should have at least one module
	for licenseType, modules := range grouped {
		if len(modules) == 0 {
			t.Errorf("license type %s has no modules", licenseType)
		}

		// All modules in a group should be non-empty
		for _, module := range modules {
			if module == "" {
				t.Errorf("license type %s has empty module name", licenseType)
			}
		}
	}

	// Should contain expected license types
	expectedTypes := []string{"MIT License", "Apache License 2.0", "BSD-3-Clause License"}
	for _, expectedType := range expectedTypes {
		if _, exists := grouped[expectedType]; !exists {
			t.Errorf("expected license type %s not found in grouped data", expectedType)
		}
	}
}
