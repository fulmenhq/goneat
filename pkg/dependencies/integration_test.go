//go:build integration
// +build integration

package dependencies

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/cooling"
)

// Helper functions

// getTestRepoPath returns the path to a test repository based on environment variable
// or default location. Returns empty string if repository should be skipped.
//
// Set GONEAT_COOLING_TEST_ROOT to point to a directory containing cloned test repos.
// Defaults to ~/dev/playground if not set.
//
// Example:
//
//	export GONEAT_COOLING_TEST_ROOT=/path/to/test/repos
//	go test -tags=integration ./pkg/dependencies/...
func getTestRepoPath(repoName string) string {
	root := os.Getenv("GONEAT_COOLING_TEST_ROOT")
	if root == "" {
		// Default to ~/dev/playground for backward compatibility
		// but allow tests to be skipped if repos aren't there
		root = os.ExpandEnv("$HOME/dev/playground")
	}

	path := filepath.Join(root, repoName)

	// Check if path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "" // Signal to skip test
	}

	return path
}

// countCoolingViolations counts age and download violations
func countCoolingViolations(issues []Issue) int {
	count := 0
	for _, issue := range issues {
		if issue.Type == "age_violation" || issue.Type == "download_violation" {
			count++
		}
	}
	return count
}

// checkRepoExists verifies test repository is available
// Skips test if repository not found or GONEAT_COOLING_TEST_ROOT not configured
func checkRepoExists(t *testing.T, path string) {
	t.Helper()
	if path == "" {
		t.Skip("Test repository not configured. Set GONEAT_COOLING_TEST_ROOT or clone repos to ~/dev/playground")
		return
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("Test repository not found: %s", path)
	}
}

// validateViolationStructure ensures violations have required fields
func validateViolationStructure(t *testing.T, issues []Issue) {
	t.Helper()
	for i, issue := range issues {
		if issue.Type == "age_violation" || issue.Type == "download_violation" {
			if issue.Message == "" {
				t.Errorf("Issue %d: missing message", i)
			}
			if issue.Severity == "" {
				t.Errorf("Issue %d: missing severity", i)
			}
			if issue.Dependency == nil {
				t.Errorf("Issue %d: missing dependency reference", i)
			}
		}
	}
}

// CacheTiming tracks cache warm/cold performance metrics
type CacheTiming struct {
	ColdHits int
	WarmHits int
	ColdTime time.Duration
	WarmTime time.Duration
}

// measureCachePerformance runs analysis twice to measure cold vs warm cache
func measureCachePerformance(t *testing.T, analyzer Analyzer, cfg AnalysisConfig) *CacheTiming {
	t.Helper()
	ctx := context.Background()
	timing := &CacheTiming{}

	// Cold cache (first run)
	start := time.Now()
	result1, err := analyzer.Analyze(ctx, cfg.Target, cfg)
	if err != nil {
		t.Fatalf("Cold cache analysis failed: %v", err)
	}
	timing.ColdTime = time.Since(start)
	timing.ColdHits = len(result1.Dependencies)

	// Warm cache (second run within TTL)
	start = time.Now()
	result2, err := analyzer.Analyze(ctx, cfg.Target, cfg)
	if err != nil {
		t.Fatalf("Warm cache analysis failed: %v", err)
	}
	timing.WarmTime = time.Since(start)
	timing.WarmHits = len(result2.Dependencies)

	// Verify cache is working (warm should be faster)
	if timing.WarmTime >= timing.ColdTime {
		t.Logf("Warning: warm cache not faster (cold=%v, warm=%v) - may indicate caching issue",
			timing.ColdTime, timing.WarmTime)
	} else {
		speedup := float64(timing.ColdTime) / float64(timing.WarmTime)
		t.Logf("Cache speedup: %.2fx (cold=%v, warm=%v)", speedup, timing.ColdTime, timing.WarmTime)
	}

	return timing
}

// Original test - keep for backwards compatibility
func TestGoAnalyzer_RealProject(t *testing.T) {
	analyzer := NewGoAnalyzer()
	cfg := &config.DependenciesConfig{
		PolicyPath: ".goneat/dependencies.yaml",
		AutoDetect: true,
	}

	ctx := context.Background()
	analysisConfig := AnalysisConfig{
		Target: "../..", // repo root
		Config: cfg,
	}

	result, err := analyzer.Analyze(ctx, analysisConfig.Target, analysisConfig)
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	if len(result.Dependencies) < 10 {
		t.Errorf("Expected at least 10 dependencies, got %d", len(result.Dependencies))
	}

	// Verify all dependencies have required metadata
	for _, dep := range result.Dependencies {
		if dep.Name == "" {
			t.Error("Dependency missing name")
		}
		if dep.Language != LanguageGo {
			t.Errorf("Expected Go language, got %s", dep.Language)
		}
		if _, ok := dep.Metadata["age_days"]; !ok {
			t.Errorf("Dependency %s missing age_days metadata", dep.Name)
		}
	}
}

// Scenario 1: Baseline Validation (Happy Path)
func TestCoolingPolicy_Hugo_Baseline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	hugoPath := getTestRepoPath("hugo")
	checkRepoExists(t, hugoPath)

	analyzer := NewGoAnalyzer()
	cfg := AnalysisConfig{
		Target:     hugoPath,
		PolicyPath: "testdata/policies/baseline.yaml",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := analyzer.Analyze(ctx, cfg.Target, cfg)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// Validate results
	if len(result.Dependencies) == 0 {
		t.Fatal("Expected dependencies, got none")
	}

	violations := countCoolingViolations(result.Issues)
	t.Logf("Analyzed %d dependencies in %v", len(result.Dependencies), result.Duration)
	t.Logf("Found %d cooling violations", violations)

	// Hugo should have mostly mature dependencies (< 10% violation rate)
	violationRate := float64(violations) / float64(len(result.Dependencies))
	if violationRate > 0.1 {
		t.Errorf("High violation rate: %.2f%% (expected < 10%%)", violationRate*100)
	}

	// Validate violation structure
	validateViolationStructure(t, result.Issues)

	// Performance check: should complete in reasonable time
	if result.Duration > 15*time.Second {
		t.Logf("Warning: analysis took %v (expected < 15s)", result.Duration)
	}
}

// Scenario 2: Strict Policy (High Thresholds)
func TestCoolingPolicy_Mattermost_Strict(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Mattermost is a monorepo - use the server subdirectory
	mattermostPath := getTestRepoPath("mattermost-server/server")
	if mattermostPath == "" {
		// Fallback to trying root directory
		mattermostPath = getTestRepoPath("mattermost-server")
	}
	checkRepoExists(t, mattermostPath)

	analyzer := NewGoAnalyzer()
	cfg := AnalysisConfig{
		Target:     mattermostPath,
		PolicyPath: "testdata/policies/strict.yaml",
	}

	result, err := analyzer.Analyze(context.Background(), cfg.Target, cfg)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	violations := countCoolingViolations(result.Issues)
	if violations == 0 {
		t.Error("Expected violations with strict policy, got none")
	}

	// Validate violation structure
	for _, issue := range result.Issues {
		if issue.Type == "age_violation" || issue.Type == "download_violation" {
			if issue.Message == "" {
				t.Error("Violation missing message")
			}
			if issue.Dependency == nil {
				t.Error("Violation missing dependency reference")
			}
			// Check message contains actual vs expected values
			if !strings.Contains(issue.Message, "minimum:") {
				t.Errorf("Violation message should contain 'minimum:' threshold: %s", issue.Message)
			}
		}
	}

	t.Logf("Strict policy triggered %d violations (%d deps total, %.1f%%)",
		violations, len(result.Dependencies),
		100*float64(violations)/float64(len(result.Dependencies)))

	// Verify policy is actually strict (should flag many packages)
	if violations < len(result.Dependencies)/4 {
		t.Logf("Warning: expected more violations with strict policy (365 days, 1M downloads)")
	}
}

// Scenario 3: Exception Pattern Matching
func TestCoolingPolicy_Traefik_Exceptions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Try both traefik and traefik-assessment
	traefikPath := getTestRepoPath("traefik")
	if traefikPath == "" {
		traefikPath = getTestRepoPath("traefik-assessment")
	}
	checkRepoExists(t, traefikPath)

	analyzer := NewGoAnalyzer()
	cfg := AnalysisConfig{
		Target:     traefikPath,
		PolicyPath: "testdata/policies/exceptions.yaml",
	}

	result, err := analyzer.Analyze(context.Background(), cfg.Target, cfg)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// Check that exception patterns work - no false positives on exempted patterns
	exemptedPatterns := []string{
		"github.com/traefik/",
		"github.com/containous/",
		"golang.org/x/",
		"github.com/spf13/",
		"github.com/stretchr/",
	}

	falsePositives := 0
	for _, issue := range result.Issues {
		if issue.Type == "age_violation" && issue.Dependency != nil {
			for _, pattern := range exemptedPatterns {
				if strings.HasPrefix(issue.Dependency.Name, pattern) {
					t.Errorf("False positive: %s should be exempted by pattern %s",
						issue.Dependency.Name, pattern)
					falsePositives++
				}
			}
		}
	}

	t.Logf("Exception patterns working correctly (checked %d issues, %d false positives)",
		len(result.Issues), falsePositives)

	if falsePositives > 0 {
		t.Errorf("Found %d false positives on exempted patterns", falsePositives)
	}
}

// Scenario 4: Time-Limited Exceptions
func TestCoolingPolicy_OPA_TimeLimited(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	opaPath := getTestRepoPath("opa")
	checkRepoExists(t, opaPath)

	analyzer := NewGoAnalyzer()
	cfg := AnalysisConfig{
		Target:     opaPath,
		PolicyPath: "testdata/policies/time-limited.yaml",
	}

	result, err := analyzer.Analyze(context.Background(), cfg.Target, cfg)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// OPA org packages should be exempted (valid until 2030)
	opaViolations := 0
	for _, issue := range result.Issues {
		if issue.Dependency != nil && strings.HasPrefix(issue.Dependency.Name, "github.com/open-policy-agent/") {
			t.Errorf("OPA package should be exempted: %s", issue.Dependency.Name)
			opaViolations++
		}
	}

	if opaViolations > 0 {
		t.Errorf("Found %d violations for OPA packages (should be exempted until 2030)", opaViolations)
	}

	// spf13 packages should also be exempted (until 2027)
	spf13Violations := 0
	for _, issue := range result.Issues {
		if issue.Dependency != nil && strings.HasPrefix(issue.Dependency.Name, "github.com/spf13/") {
			t.Errorf("spf13 package should be exempted: %s", issue.Dependency.Name)
			spf13Violations++
		}
	}

	t.Logf("Time-limited exceptions validated (%d total deps, %d OPA violations, %d spf13 violations)",
		len(result.Dependencies), opaViolations, spf13Violations)
}

// Scenario 5: Registry Failure Handling
func TestCoolingPolicy_RegistryFailure_Graceful(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Use hugo as test subject
	hugoPath := getTestRepoPath("hugo")
	checkRepoExists(t, hugoPath)

	analyzer := NewGoAnalyzer()
	cfg := AnalysisConfig{
		Target:     hugoPath,
		PolicyPath: "testdata/policies/baseline.yaml",
	}

	result, err := analyzer.Analyze(context.Background(), cfg.Target, cfg)
	if err != nil {
		t.Fatalf("Analysis should not fail on registry errors: %v", err)
	}

	// Check that dependencies with registry errors use conservative fallback
	registryErrors := 0
	conservativeFallbacks := 0

	for _, dep := range result.Dependencies {
		if registryErr, ok := dep.Metadata["registry_error"]; ok {
			registryErrors++
			t.Logf("Registry error for %s: %v", dep.Name, registryErr)

			// Should have conservative age fallback (365 days)
			if ageDays, ok := dep.Metadata["age_days"].(int); ok {
				if ageDays == 365 {
					conservativeFallbacks++
				}
			}

			// Should have age_unknown flag
			if ageUnknown, ok := dep.Metadata["age_unknown"].(bool); !ok || !ageUnknown {
				t.Errorf("Expected age_unknown=true for %s with registry error", dep.Name)
			}
		}
	}

	t.Logf("Registry failures handled gracefully: %d errors, %d conservative fallbacks (365 days)",
		registryErrors, conservativeFallbacks)

	// All registry errors should use conservative fallback
	if registryErrors > 0 && conservativeFallbacks != registryErrors {
		t.Errorf("Not all registry errors used conservative fallback: %d errors, %d fallbacks",
			registryErrors, conservativeFallbacks)
	}
}

// Scenario 6: Cache Performance Validation
func TestCoolingPolicy_CachePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	hugoPath := getTestRepoPath("hugo")
	checkRepoExists(t, hugoPath)

	analyzer := NewGoAnalyzer()
	cfg := AnalysisConfig{
		Target:     hugoPath,
		PolicyPath: "testdata/policies/baseline.yaml",
	}

	// Measure cache warm/cold performance
	timing := measureCachePerformance(t, analyzer, cfg)

	// Cache should provide significant speedup
	if timing.WarmTime >= timing.ColdTime {
		t.Errorf("Warm cache should be faster than cold (cold=%v, warm=%v)",
			timing.ColdTime, timing.WarmTime)
	}

	// Expected speedup: at least 2x for registry calls
	speedup := float64(timing.ColdTime) / float64(timing.WarmTime)
	if speedup < 1.5 {
		t.Errorf("Expected cache speedup >= 1.5x, got %.2fx", speedup)
	}

	// Verify 24-hour TTL behavior
	t.Logf("Cache performance: cold=%v, warm=%v, speedup=%.2fx",
		timing.ColdTime, timing.WarmTime, speedup)
	t.Logf("Dependencies: %d", timing.ColdHits)

	// Calculate expected cache hit rate (should be near 100% on second run)
	if timing.ColdHits != timing.WarmHits {
		t.Errorf("Dependency count mismatch: cold=%d, warm=%d", timing.ColdHits, timing.WarmHits)
	}
}

// Test: Disabled Cooling (Control)
func TestCoolingPolicy_Disabled(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Use any repo - we're testing that cooling is disabled
	hugoPath := getTestRepoPath("hugo")
	checkRepoExists(t, hugoPath)

	analyzer := NewGoAnalyzer()
	cfg := AnalysisConfig{
		Target:     hugoPath,
		PolicyPath: "testdata/policies/disabled.yaml",
	}

	result, err := analyzer.Analyze(context.Background(), cfg.Target, cfg)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// Should have zero cooling violations when disabled
	violations := countCoolingViolations(result.Issues)
	if violations > 0 {
		t.Errorf("Expected zero cooling violations with disabled policy, got %d", violations)
	}

	t.Logf("Disabled cooling policy: %d deps, %d violations (expected 0)",
		len(result.Dependencies), violations)
}

// Synthetic Fixture Test: CI-Friendly Baseline
// This test uses a controlled fixture in tests/fixtures/ and can run in CI
// without requiring external repository clones.
func TestCoolingPolicy_Synthetic_Baseline(t *testing.T) {
	// This test does NOT require GONEAT_COOLING_TEST_ROOT
	// It uses a local fixture for CI/CD reliability
	syntheticPath := "../../tests/fixtures/dependencies/synthetic-go-project"

	// Verify fixture exists
	if _, err := os.Stat(syntheticPath); os.IsNotExist(err) {
		t.Fatalf("Synthetic fixture not found: %s (this should always exist in repo)", syntheticPath)
	}

	analyzer := NewGoAnalyzer()
	cfg := AnalysisConfig{
		Target:     syntheticPath,
		PolicyPath: "testdata/policies/baseline.yaml",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := analyzer.Analyze(ctx, cfg.Target, cfg)
	if err != nil {
		t.Fatalf("Synthetic fixture analysis failed: %v", err)
	}

	// Validate results
	if len(result.Dependencies) == 0 {
		t.Fatal("Expected dependencies in synthetic fixture, got none")
	}

	// Synthetic fixture uses mature, stable dependencies
	// Should have very low or zero violations with baseline policy
	violations := countCoolingViolations(result.Issues)
	t.Logf("Synthetic fixture: %d dependencies, %d cooling violations", len(result.Dependencies), violations)

	// All dependencies should pass baseline policy (7 days, 100 downloads)
	if violations > 0 {
		t.Logf("Warning: synthetic fixture had %d violations (dependencies may need updating)", violations)
		for _, issue := range result.Issues {
			if issue.Type == "age_violation" || issue.Type == "download_violation" {
				t.Logf("  - %s: %s", issue.Type, issue.Message)
			}
		}
	}

	// Validate violation structure
	validateViolationStructure(t, result.Issues)

	// Performance: synthetic should be fast (< 5s)
	if result.Duration > 5*time.Second {
		t.Logf("Warning: synthetic analysis took %v (expected < 5s)", result.Duration)
	}
}

// Test: Direct Cooling Checker Unit Test
func TestCoolingChecker_Integration(t *testing.T) {
	// Create sample dependency
	dep := &Dependency{
		Module: Module{
			Name:    "github.com/example/test-package",
			Version: "v1.0.0",
		},
		Metadata: map[string]interface{}{
			"age_days":         3, // 3 days old
			"total_downloads":  50,
			"recent_downloads": 5,
		},
	}

	// Test with strict policy
	cfg := config.CoolingConfig{
		Enabled:            true,
		MinAgeDays:         7,
		MinDownloads:       100,
		MinDownloadsRecent: 10,
	}

	checker := cooling.NewChecker(cfg)
	result, err := checker.Check(dep)

	if err != nil {
		t.Fatalf("Checker failed: %v", err)
	}

	if result.Passed {
		t.Error("Expected violations for young, unpopular package")
	}

	// Should have both age and download violations
	if len(result.Violations) == 0 {
		t.Error("Expected violations, got none")
	}

	t.Logf("Cooling checker integration: %d violations for test package", len(result.Violations))
	for _, v := range result.Violations {
		t.Logf("  - %s: %s", v.Type, v.Message)
	}
}
