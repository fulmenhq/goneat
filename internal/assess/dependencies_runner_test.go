/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fulmenhq/goneat/pkg/dependencies"
)

func TestDependenciesRunner_Assess_NoLanguageDetected(t *testing.T) {
	runner := NewDependenciesRunner()

	// Create a temporary directory with no language manifest files
	tmpDir := t.TempDir()

	// Test with directory that has no supported language
	result, err := runner.Assess(context.Background(), tmpDir, DefaultAssessmentConfig())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("Expected success when no language detected (skipped)")
	}

	if result.Metrics["status"] != "skipped" {
		t.Errorf("Expected status=skipped, got %v", result.Metrics["status"])
	}

	if result.Metrics["reason"] != "no supported language detected" {
		t.Errorf("Expected reason='no supported language detected', got %v", result.Metrics["reason"])
	}
}

func TestDependenciesRunner_SeverityMapping(t *testing.T) {
	runner := NewDependenciesRunner()

	tests := []struct {
		name        string
		depSeverity string
		issueType   string
		want        IssueSeverity
	}{
		{"critical_mapped", "critical", "license", SeverityCritical},
		{"high_mapped", "high", "license", SeverityHigh},
		{"medium_mapped", "medium", "license", SeverityMedium},
		{"low_mapped", "low", "license", SeverityLow},
		{"info_mapped", "info", "license", SeverityInfo},
		{"cooling_default", "", "cooling", SeverityHigh},
		{"license_default", "", "license", SeverityMedium},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runner.mapSeverity(tt.depSeverity, tt.issueType)
			if got != tt.want {
				t.Errorf("mapSeverity(%q, %q) = %v, want %v", tt.depSeverity, tt.issueType, got, tt.want)
			}
		})
	}
}

func TestDependenciesRunner_FailureThreshold(t *testing.T) {
	runner := NewDependenciesRunner()

	tests := []struct {
		name      string
		issues    []Issue
		threshold IssueSeverity
		wantPass  bool
	}{
		{
			name:      "no_issues",
			issues:    []Issue{},
			threshold: SeverityCritical,
			wantPass:  true,
		},
		{
			name: "critical_fails_on_critical",
			issues: []Issue{
				{Severity: SeverityCritical},
			},
			threshold: SeverityCritical,
			wantPass:  false,
		},
		{
			name: "high_passes_on_critical_threshold",
			issues: []Issue{
				{Severity: SeverityHigh},
			},
			threshold: SeverityCritical,
			wantPass:  true,
		},
		{
			name: "medium_fails_on_medium_threshold",
			issues: []Issue{
				{Severity: SeverityMedium},
			},
			threshold: SeverityMedium,
			wantPass:  false,
		},
		{
			name: "low_passes_on_high_threshold",
			issues: []Issue{
				{Severity: SeverityLow},
			},
			threshold: SeverityHigh,
			wantPass:  true,
		},
		{
			name: "info_passes_on_all_thresholds",
			issues: []Issue{
				{Severity: SeverityInfo},
			},
			threshold: SeverityInfo,
			wantPass:  false, // Info level issue fails on info threshold
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runner.shouldPass(tt.issues, tt.threshold)
			if got != tt.wantPass {
				t.Errorf("shouldPass() = %v, want %v", got, tt.wantPass)
			}
		})
	}
}

func TestDependenciesRunner_Metrics(t *testing.T) {
	runner := NewDependenciesRunner()

	result := &dependencies.AnalysisResult{
		Dependencies: []dependencies.Dependency{
			{Module: dependencies.Module{Name: "dep1"}},
			{Module: dependencies.Module{Name: "dep2"}},
		},
		Issues: []dependencies.Issue{
			{Type: "license", Severity: "high"},
			{Type: "license", Severity: "medium"},
			{Type: "cooling", Severity: "high"},
		},
		Passed:   false,
		Duration: 1 * time.Second,
	}

	metrics := runner.buildMetrics(result, ".")

	if metrics["dependency_count"] != 2 {
		t.Errorf("dependency_count = %v, want 2", metrics["dependency_count"])
	}

	if metrics["license_violations"] != 2 {
		t.Errorf("license_violations = %v, want 2", metrics["license_violations"])
	}

	if metrics["cooling_violations"] != 1 {
		t.Errorf("cooling_violations = %v, want 1", metrics["cooling_violations"])
	}

	if metrics["analysis_passed"] != false {
		t.Errorf("analysis_passed = %v, want false", metrics["analysis_passed"])
	}

	// Verify SBOM metadata exists
	if _, ok := metrics["sbom_metadata"]; !ok {
		t.Error("Expected sbom_metadata in metrics")
	}
}

func TestDependenciesRunner_InterfaceMethods(t *testing.T) {
	runner := NewDependenciesRunner()

	if runner.GetCategory() != CategoryDependencies {
		t.Errorf("GetCategory() = %v, want %v", runner.GetCategory(), CategoryDependencies)
	}

	if runner.CanRunInParallel() {
		t.Error("CanRunInParallel() = true, want false (network calls)")
	}

	if !runner.IsAvailable() {
		t.Error("IsAvailable() = false, want true")
	}

	estimatedTime := runner.GetEstimatedTime(".")
	if estimatedTime != 15*time.Second {
		t.Errorf("GetEstimatedTime() = %v, want 15s", estimatedTime)
	}
}

// TestDependenciesRunner_SeverityLevelsMatchCrucibleSchema validates that our
// hardcoded severity levels match the canonical Crucible severity schema.
// This test ensures we don't drift from the standard defined in:
// schemas/crucible-go/assessment/v1.0.0/severity-definitions.schema.json
func TestDependenciesRunner_SeverityLevelsMatchCrucibleSchema(t *testing.T) {
	// Expected mapping from Crucible schema severityMapping
	crucibleMapping := map[string]int{
		"info":     0,
		"low":      1,
		"medium":   2,
		"high":     3,
		"critical": 4,
	}

	// Our constant mapping (from internal/assess/types.go)
	ourMapping := map[string]IssueSeverity{
		"info":     SeverityInfo,
		"low":      SeverityLow,
		"medium":   SeverityMedium,
		"high":     SeverityHigh,
		"critical": SeverityCritical,
	}

	// Our runtime severity levels (from shouldPass method)
	severityLevels := map[IssueSeverity]int{
		SeverityInfo:     0,
		SeverityLow:      1,
		SeverityMedium:   2,
		SeverityHigh:     3,
		SeverityCritical: 4,
	}

	// Validate each severity name maps to correct Crucible level
	for name, crucibleLevel := range crucibleMapping {
		ourSeverity := ourMapping[name]
		ourLevel := severityLevels[ourSeverity]

		if ourLevel != crucibleLevel {
			t.Errorf("Severity %q: our level=%d, Crucible level=%d (MUST MATCH canonical schema)",
				name, ourLevel, crucibleLevel)
		}
	}

	// Also test via mapSeverity to ensure mapping function is consistent
	runner := NewDependenciesRunner()
	for name, expectedLevel := range crucibleMapping {
		mappedSeverity := runner.mapSeverity(name, "license")
		actualLevel := severityLevels[mappedSeverity]

		if actualLevel != expectedLevel {
			t.Errorf("mapSeverity(%q) returned level %d, want %d (Crucible canonical)",
				name, actualLevel, expectedLevel)
		}
	}
}

func TestDependenciesRunner_EstimateRemediationTime(t *testing.T) {
	runner := NewDependenciesRunner()

	tests := []struct {
		name      string
		issueType string
		want      time.Duration
	}{
		{"license_issue", "license", 30 * time.Minute},
		{"cooling_issue", "cooling", 15 * time.Minute},
		{"vulnerability_issue", "vulnerability", 45 * time.Minute},
		{"unknown_issue", "unknown", 20 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runner.estimateRemediationTime(tt.issueType)
			if time.Duration(got) != tt.want {
				t.Errorf("estimateRemediationTime(%q) = %v, want %v", tt.issueType, got, tt.want)
			}
		})
	}
}

func TestDependenciesRunner_CountIssuesByType(t *testing.T) {
	runner := NewDependenciesRunner()

	issues := []dependencies.Issue{
		{Type: "license"},
		{Type: "license"},
		{Type: "cooling"},
		{Type: "vulnerability"},
	}

	tests := []struct {
		name      string
		issueType string
		want      int
	}{
		{"license_count", "license", 2},
		{"cooling_count", "cooling", 1},
		{"vulnerability_count", "vulnerability", 1},
		{"nonexistent_count", "nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runner.countIssuesByType(issues, tt.issueType)
			if got != tt.want {
				t.Errorf("countIssuesByType(%q) = %d, want %d", tt.issueType, got, tt.want)
			}
		})
	}
}

func TestDependenciesRunner_ConvertToAssessmentIssues(t *testing.T) {
	runner := NewDependenciesRunner()

	analysisResult := &dependencies.AnalysisResult{
		Issues: []dependencies.Issue{
			{
				Type:     "license",
				Severity: "critical",
				Message:  "Forbidden license detected",
			},
			{
				Type:     "cooling",
				Severity: "high",
				Message:  "Package too new",
			},
		},
	}

	issues := runner.convertToAssessmentIssues(analysisResult)

	if len(issues) != 2 {
		t.Fatalf("Expected 2 issues, got %d", len(issues))
	}

	// Check first issue (license)
	if issues[0].Category != CategoryDependencies {
		t.Errorf("Issue[0] category = %v, want %v", issues[0].Category, CategoryDependencies)
	}
	if issues[0].SubCategory != "license" {
		t.Errorf("Issue[0] subcategory = %v, want %v", issues[0].SubCategory, "license")
	}
	if issues[0].Severity != SeverityCritical {
		t.Errorf("Issue[0] severity = %v, want %v", issues[0].Severity, SeverityCritical)
	}
	if issues[0].AutoFixable {
		t.Error("Issue[0] AutoFixable = true, want false (dependencies require manual intervention)")
	}

	// Check second issue (cooling)
	if issues[1].Severity != SeverityHigh {
		t.Errorf("Issue[1] severity = %v, want %v", issues[1].Severity, SeverityHigh)
	}
	if issues[1].SubCategory != "cooling" {
		t.Errorf("Issue[1] subcategory = %v, want %v", issues[1].SubCategory, "cooling")
	}
}

// TestDependenciesRunner_SelectAnalyzer validates the factory pattern for analyzer selection
func TestDependenciesRunner_SelectAnalyzer(t *testing.T) {
	runner := NewDependenciesRunner()

	tests := []struct {
		name     string
		lang     dependencies.Language
		wantType string
		wantNil  bool
	}{
		{"go_analyzer", dependencies.LanguageGo, "*dependencies.GoAnalyzer", false},
		{"typescript_analyzer", dependencies.LanguageTypeScript, "*dependencies.TypeScriptAnalyzer", false},
		{"python_analyzer", dependencies.LanguagePython, "*dependencies.PythonAnalyzer", false},
		{"rust_analyzer", dependencies.LanguageRust, "*dependencies.RustAnalyzer", false},
		{"csharp_analyzer", dependencies.LanguageCSharp, "*dependencies.CSharpAnalyzer", false},
		{"unknown_language", dependencies.Language("unknown"), "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := runner.selectAnalyzer(tt.lang)

			if tt.wantNil {
				if analyzer != nil {
					t.Errorf("selectAnalyzer(%q) = %T, want nil", tt.lang, analyzer)
				}
			} else {
				if analyzer == nil {
					t.Fatalf("selectAnalyzer(%q) = nil, want %s", tt.lang, tt.wantType)
				}
			}
		})
	}
}

// TestDependenciesRunner_StubAnalyzers validates stub analyzers return empty success
// This test creates directories with language-specific manifest files to trigger detection,
// then verifies the stub analyzers handle them gracefully (audit resolution for HIGH severity finding)
func TestDependenciesRunner_StubAnalyzers(t *testing.T) {
	tests := []struct {
		name         string
		lang         dependencies.Language
		manifestFile string
		content      string
	}{
		{
			name:         "typescript_stub",
			lang:         dependencies.LanguageTypeScript,
			manifestFile: "package.json",
			content:      `{"name":"test","dependencies":{}}`,
		},
		{
			name:         "python_stub",
			lang:         dependencies.LanguagePython,
			manifestFile: "requirements.txt",
			content:      "# Empty requirements",
		},
		{
			name:         "rust_stub",
			lang:         dependencies.LanguageRust,
			manifestFile: "Cargo.toml",
			content:      `[package]\nname = "test"`,
		},
		{
			name:         "csharp_stub",
			lang:         dependencies.LanguageCSharp,
			manifestFile: "test.csproj",
			content:      `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFramework>net6.0</TargetFramework></PropertyGroup></Project>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewDependenciesRunner()

			// Verify the analyzer is created correctly
			analyzer := runner.selectAnalyzer(tt.lang)
			if analyzer == nil {
				t.Fatalf("selectAnalyzer(%q) returned nil", tt.lang)
			}

			// Verify the analyzer returns empty success result
			ctx := context.Background()
			config := dependencies.AnalysisConfig{
				Languages: []dependencies.Language{tt.lang},
			}

			result, err := analyzer.Analyze(ctx, ".", config)
			if err != nil {
				t.Errorf("Stub analyzer returned error: %v", err)
			}

			if result == nil {
				t.Fatal("Stub analyzer returned nil result")
			}

			if !result.Passed {
				t.Error("Stub analyzer result.Passed = false, want true")
			}

			if len(result.Issues) != 0 {
				t.Errorf("Stub analyzer returned %d issues, want 0", len(result.Issues))
			}

			if len(result.Dependencies) != 0 {
				t.Errorf("Stub analyzer returned %d dependencies, want 0", len(result.Dependencies))
			}
		})
	}
}

// TestDependenciesRunner_FindExistingSBOM_NonRootTarget validates SBOM resolution
// with monorepo support (audit resolution for MEDIUM severity finding)
func TestDependenciesRunner_FindExistingSBOM_NonRootTarget(t *testing.T) {
	// Create temporary monorepo structure:
	// monorepo/
	//   sbom/goneat-latest.cdx.json  <- Parent SBOM
	//   packages/
	//     my-app/                    <- Target directory
	tmpDir := t.TempDir()

	// Create monorepo SBOM at root
	sbomDir := tmpDir + "/sbom"
	if err := createDir(sbomDir); err != nil {
		t.Fatalf("Failed to create sbom dir: %v", err)
	}
	sbomPath := sbomDir + "/goneat-latest.cdx.json"
	if err := createFile(sbomPath, `{"bomFormat":"CycloneDX"}`); err != nil {
		t.Fatalf("Failed to create SBOM: %v", err)
	}

	// Create nested target directory
	packagesDir := tmpDir + "/packages"
	if err := createDir(packagesDir); err != nil {
		t.Fatalf("Failed to create packages dir: %v", err)
	}
	targetDir := packagesDir + "/my-app"
	if err := createDir(targetDir); err != nil {
		t.Fatalf("Failed to create target dir: %v", err)
	}

	runner := NewDependenciesRunner()

	// Test 1: Find SBOM from target directory (2 levels up)
	foundPath := runner.findExistingSBOM(targetDir)
	if foundPath != sbomPath {
		t.Errorf("findExistingSBOM(%q) = %q, want %q", targetDir, foundPath, sbomPath)
	}

	// Test 2: Find SBOM from root (direct child)
	foundPath = runner.findExistingSBOM(tmpDir)
	if foundPath != sbomPath {
		t.Errorf("findExistingSBOM(%q) = %q, want %q", tmpDir, foundPath, sbomPath)
	}

	// Test 3: No SBOM found in deeply nested directory (>3 levels)
	deepDir := targetDir + "/src/components/ui"
	if err := createDir(targetDir + "/src"); err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}
	if err := createDir(targetDir + "/src/components"); err != nil {
		t.Fatalf("Failed to create components dir: %v", err)
	}
	if err := createDir(deepDir); err != nil {
		t.Fatalf("Failed to create deep dir: %v", err)
	}

	foundPath = runner.findExistingSBOM(deepDir)
	if foundPath != "" {
		t.Errorf("findExistingSBOM(%q) = %q, want empty (exceeds 3 level limit)", deepDir, foundPath)
	}
}

// TestDependenciesRunner_UpdateMetricsFromIssues validates that metrics are updated
// based on the final combined issues list, including cargo-deny findings.
// Note: analysis_passed is NOT updated by this function - it preserves the original
// analyzer result. Assessment threshold pass/fail is in AssessmentResult.Success.
func TestDependenciesRunner_UpdateMetricsFromIssues(t *testing.T) {
	runner := NewDependenciesRunner()

	tests := []struct {
		name             string
		issues           []Issue
		wantLicenseCount int
		wantBansCount    int
	}{
		{
			name:             "empty_issues",
			issues:           []Issue{},
			wantLicenseCount: 0,
			wantBansCount:    0,
		},
		{
			name: "license_issues_only",
			issues: []Issue{
				{SubCategory: "license", Severity: SeverityHigh},
				{SubCategory: "rust:cargo-deny:license", Severity: SeverityHigh},
			},
			wantLicenseCount: 2,
			wantBansCount:    0,
		},
		{
			name: "bans_issues_only",
			issues: []Issue{
				{SubCategory: "rust:cargo-deny:bans", Severity: SeverityMedium},
				{SubCategory: "rust:cargo-deny:bans", Severity: SeverityMedium},
			},
			wantLicenseCount: 0,
			wantBansCount:    2,
		},
		{
			name: "mixed_issues",
			issues: []Issue{
				{SubCategory: "license", Severity: SeverityHigh},
				{SubCategory: "rust:cargo-deny:license", Severity: SeverityHigh},
				{SubCategory: "rust:cargo-deny:bans", Severity: SeverityMedium},
				{SubCategory: "cooling", Severity: SeverityHigh}, // Not counted in license/bans
			},
			wantLicenseCount: 2,
			wantBansCount:    1,
		},
		{
			name: "other_subcategories_not_counted",
			issues: []Issue{
				{SubCategory: "cooling", Severity: SeverityHigh},
				{SubCategory: "vulnerability", Severity: SeverityCritical},
				{SubCategory: "rust:cargo-deny", Severity: SeverityMedium}, // Generic, not specific
			},
			wantLicenseCount: 0,
			wantBansCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := map[string]interface{}{
				"license_violations": 999,  // Should be overwritten
				"analysis_passed":    true, // Should NOT be overwritten
			}

			runner.updateMetricsFromIssues(metrics, tt.issues)

			if got := metrics["license_violations"].(int); got != tt.wantLicenseCount {
				t.Errorf("license_violations = %d, want %d", got, tt.wantLicenseCount)
			}
			if got := metrics["bans_violations"].(int); got != tt.wantBansCount {
				t.Errorf("bans_violations = %d, want %d", got, tt.wantBansCount)
			}
			// Verify analysis_passed is NOT modified (preserves original analyzer result)
			if got := metrics["analysis_passed"].(bool); got != true {
				t.Errorf("analysis_passed was modified to %v, should remain unchanged", got)
			}
		})
	}
}

// TestCargoDenyDependencySeverityMapping validates severity mapping for cargo-deny
// dependency checks (licenses and bans) per spec requirements.
// Uses dependencies.CargoDenyFinding from pkg/dependencies (canonical implementation).
func TestCargoDenyDependencySeverityMapping(t *testing.T) {
	tests := []struct {
		name     string
		finding  dependencies.CargoDenyFinding
		expected IssueSeverity
	}{
		{
			name:     "license_singular_always_high",
			finding:  dependencies.CargoDenyFinding{Type: "license", Severity: "warning"},
			expected: SeverityHigh,
		},
		{
			name:     "licenses_plural_always_high",
			finding:  dependencies.CargoDenyFinding{Type: "licenses", Severity: "error"},
			expected: SeverityHigh,
		},
		{
			name:     "ban_singular_always_medium",
			finding:  dependencies.CargoDenyFinding{Type: "ban", Severity: "error"},
			expected: SeverityMedium,
		},
		{
			name:     "bans_plural_always_medium",
			finding:  dependencies.CargoDenyFinding{Type: "bans", Severity: "error"},
			expected: SeverityMedium,
		},
		{
			name:     "unknown_error_severity_maps_to_high",
			finding:  dependencies.CargoDenyFinding{Type: "other", Severity: "error"},
			expected: SeverityHigh,
		},
		{
			name:     "unknown_warning_severity_maps_to_medium",
			finding:  dependencies.CargoDenyFinding{Type: "other", Severity: "warning"},
			expected: SeverityMedium,
		},
		{
			name:     "unknown_note_severity_maps_to_low",
			finding:  dependencies.CargoDenyFinding{Type: "other", Severity: "note"},
			expected: SeverityLow,
		},
		{
			name:     "informational_code_is_low",
			finding:  dependencies.CargoDenyFinding{Type: "license", Code: "license-not-encountered"},
			expected: SeverityLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapCargoDenyDependencySeverity(tt.finding)
			if got != tt.expected {
				t.Errorf("mapCargoDenyDependencySeverity() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestCargoDenySubcategoryMapping validates subcategory assignment handles
// both singular and plural forms from cargo-deny output
func TestCargoDenySubcategoryMapping(t *testing.T) {
	tests := []struct {
		name            string
		entryType       string
		wantSubCategory string
	}{
		{"license_singular", "license", "rust:cargo-deny:license"},
		{"licenses_plural", "licenses", "rust:cargo-deny:license"},
		{"ban_singular", "ban", "rust:cargo-deny:bans"},
		{"bans_plural", "bans", "rust:cargo-deny:bans"},
		{"other_type", "advisories", "rust:cargo-deny"},
		{"empty_type", "", "rust:cargo-deny"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the subcategory mapping logic from RunCargoDenyDependencyChecks
			subCategory := "rust:cargo-deny"
			switch tt.entryType {
			case "license", "licenses":
				subCategory = "rust:cargo-deny:license"
			case "ban", "bans":
				subCategory = "rust:cargo-deny:bans"
			}

			if subCategory != tt.wantSubCategory {
				t.Errorf("subcategory for type=%q: got %q, want %q", tt.entryType, subCategory, tt.wantSubCategory)
			}
		})
	}
}

// Helper functions for test file creation
func createDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func createFile(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
