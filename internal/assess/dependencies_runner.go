/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/dependencies"
	"github.com/fulmenhq/goneat/pkg/logger"
)

// DependenciesRunner implements AssessmentRunner for the dependencies command
type DependenciesRunner struct {
	analyzer dependencies.Analyzer
	detector *dependencies.Detector
}

// NewDependenciesRunner creates a new dependencies assessment runner
func NewDependenciesRunner() *DependenciesRunner {
	return &DependenciesRunner{
		// analyzer will be selected in Assess() based on detected language
	}
}

// Assess implements AssessmentRunner.Assess
func (r *DependenciesRunner) Assess(ctx context.Context, target string, assessConfig AssessmentConfig) (*AssessmentResult, error) {
	startTime := time.Now()
	logger.Info(fmt.Sprintf("Running dependencies assessment on %s", target))

	// Load project config
	cfg, err := config.LoadProjectConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	depsCfg := cfg.GetDependenciesConfig()
	r.detector = dependencies.NewDetector(&depsCfg).(*dependencies.Detector)

	// Detect language
	lang, found, err := r.detector.Detect(target)
	if err != nil || !found || lang == "" {
		// Return skipped result
		logger.Info("No supported language detected, skipping dependencies assessment")
		return &AssessmentResult{
			CommandName:   "dependencies",
			Category:      CategoryDependencies,
			Success:       true,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Issues:        []Issue{},
			Metrics: map[string]interface{}{
				"status": "skipped",
				"reason": "no supported language detected",
			},
		}, nil
	}

	logger.Debug(fmt.Sprintf("Detected language: %s", lang))

	// Select appropriate analyzer for detected language
	r.analyzer = r.selectAnalyzer(lang)
	if r.analyzer == nil {
		logger.Warn(fmt.Sprintf("No analyzer available for language: %s", lang))
		return &AssessmentResult{
			CommandName:   "dependencies",
			Category:      CategoryDependencies,
			Success:       true,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Issues:        []Issue{},
			Metrics: map[string]interface{}{
				"status":            "skipped",
				"reason":            "no analyzer for detected language",
				"detected_language": string(lang),
			},
		}, nil
	}

	// Run analysis
	analysisConfig := dependencies.AnalysisConfig{
		PolicyPath:    depsCfg.PolicyPath,
		EngineType:    depsCfg.Engine.Type,
		Languages:     []dependencies.Language{lang},
		Target:        target,
		CheckLicenses: true,
		CheckCooling:  true,
		Config:        &depsCfg,
	}

	result, err := r.analyzer.Analyze(ctx, target, analysisConfig)
	if err != nil {
		logger.Error(fmt.Sprintf("Dependencies analysis failed: %v", err))
		return &AssessmentResult{
			CommandName:   "dependencies",
			Category:      CategoryDependencies,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Issues:        []Issue{},
			Error:         err.Error(),
		}, nil
	}

	// Convert to assessment issues
	issues := r.convertToAssessmentIssues(result)

	// For Rust projects, run cargo-deny license and bans checks directly
	// (RustAnalyzer is a stub to avoid import cycles)
	if lang == dependencies.LanguageRust {
		rustIssues, rustErr := RunCargoDenyDependencyChecks(target, assessConfig.Timeout)
		if rustErr != nil {
			logger.Warn(fmt.Sprintf("cargo-deny dependency check failed: %v", rustErr))
		} else if rustIssues != nil {
			issues = append(issues, rustIssues...)
			logger.Info(fmt.Sprintf("cargo-deny found %d dependency issues", len(rustIssues)))
		}
	}

	// Determine success based on fail threshold
	success := r.shouldPass(issues, assessConfig.FailOnSeverity)

	logger.Info(fmt.Sprintf("Dependencies assessment completed: %d issues found, passed: %t", len(issues), success))

	// Build metrics using the final combined issues list (not just analyzer result)
	// This ensures Rust cargo-deny issues are reflected in metrics
	metrics := r.buildMetrics(result, target)
	r.updateMetricsFromIssues(metrics, issues)

	return &AssessmentResult{
		CommandName:   "dependencies",
		Category:      CategoryDependencies,
		Success:       success,
		ExecutionTime: HumanReadableDuration(time.Since(startTime)),
		Issues:        issues,
		Metrics:       metrics,
	}, nil
}

// selectAnalyzer returns the appropriate analyzer for the detected language
func (r *DependenciesRunner) selectAnalyzer(lang dependencies.Language) dependencies.Analyzer {
	switch lang {
	case dependencies.LanguageGo:
		return dependencies.NewGoAnalyzer()
	case dependencies.LanguageTypeScript:
		return dependencies.NewTypeScriptAnalyzer()
	case dependencies.LanguagePython:
		return dependencies.NewPythonAnalyzer()
	case dependencies.LanguageRust:
		return dependencies.NewRustAnalyzer()
	case dependencies.LanguageCSharp:
		return dependencies.NewCSharpAnalyzer()
	default:
		return nil
	}
}

// convertToAssessmentIssues converts dependency analysis issues to assessment issues
func (r *DependenciesRunner) convertToAssessmentIssues(result *dependencies.AnalysisResult) []Issue {
	issues := make([]Issue, 0, len(result.Issues))

	for _, depIssue := range result.Issues {
		issue := Issue{
			File:          "", // Dependencies are repo-wide, not file-specific
			Severity:      r.mapSeverity(depIssue.Severity, depIssue.Type),
			Message:       depIssue.Message,
			Category:      CategoryDependencies,
			SubCategory:   depIssue.Type, // "license", "cooling"
			AutoFixable:   false,         // Dependency issues require manual intervention
			EstimatedTime: r.estimateRemediationTime(depIssue.Type),
		}
		issues = append(issues, issue)
	}

	return issues
}

// mapSeverity maps dependency issue severity to assessment severity.
// IMPORTANT: This mapping MUST match the canonical Crucible severity definitions at:
// schemas/crucible-go/assessment/v1.0.0/severity-definitions.schema.json
// The mapping is validated in TestDependenciesRunner_SeverityLevelsMatchCrucibleSchema.
func (r *DependenciesRunner) mapSeverity(depSeverity string, issueType string) IssueSeverity {
	// Canonical Crucible severity mapping: info=0, low=1, medium=2, high=3, critical=4
	switch depSeverity {
	case "critical":
		return SeverityCritical // level 4
	case "high":
		return SeverityHigh // level 3
	case "medium":
		return SeverityMedium // level 2
	case "low":
		return SeverityLow // level 1
	case "info":
		return SeverityInfo // level 0
	default:
		// Default by issue type
		if issueType == "cooling" {
			return SeverityHigh // Cooling violations are high by default (supply-chain risk)
		}
		return SeverityMedium
	}
}

// estimateRemediationTime estimates time to remediate an issue by type
func (r *DependenciesRunner) estimateRemediationTime(issueType string) HumanReadableDuration {
	switch issueType {
	case "license":
		return HumanReadableDuration(30 * time.Minute) // Review license + replace dependency
	case "cooling":
		return HumanReadableDuration(15 * time.Minute) // Add exception or wait for package maturity
	case "vulnerability":
		return HumanReadableDuration(45 * time.Minute) // Research + update + test
	default:
		return HumanReadableDuration(20 * time.Minute) // Generic fallback
	}
}

// buildMetrics constructs metrics payload for assessment result
func (r *DependenciesRunner) buildMetrics(result *dependencies.AnalysisResult, target string) map[string]interface{} {
	metrics := map[string]interface{}{
		"dependency_count":     len(result.Dependencies),
		"license_violations":   r.countIssuesByType(result.Issues, "license"),
		"cooling_violations":   r.countIssuesByType(result.Issues, "cooling"),
		"analysis_passed":      result.Passed,
		"analysis_duration_ms": result.Duration.Milliseconds(),
	}

	// Add SBOM metadata if available (check for existing SBOM file)
	if sbomPath := r.findExistingSBOM(target); sbomPath != "" {
		metrics["sbom_metadata"] = map[string]string{
			"path":   sbomPath,
			"status": "available",
		}
	} else {
		metrics["sbom_metadata"] = map[string]string{
			"status": "not_generated",
		}
	}

	return metrics
}

// countIssuesByType counts issues of a specific type
func (r *DependenciesRunner) countIssuesByType(issues []dependencies.Issue, issueType string) int {
	count := 0
	for _, issue := range issues {
		if issue.Type == issueType {
			count++
		}
	}
	return count
}

// updateMetricsFromIssues updates metrics based on the final combined issues list.
// This ensures metrics reflect all issues including cargo-deny findings for Rust.
// Note: Does NOT override analysis_passed - that reflects the underlying analyzer result.
// The assessment pass/fail based on --fail-on threshold is in AssessmentResult.Success.
func (r *DependenciesRunner) updateMetricsFromIssues(metrics map[string]interface{}, issues []Issue) {
	// Count license and bans violations from assessment issues
	licenseCount := 0
	bansCount := 0
	for _, issue := range issues {
		switch issue.SubCategory {
		case "license", "rust:cargo-deny:license":
			licenseCount++
		case "rust:cargo-deny:bans":
			bansCount++
		}
	}

	// Update metrics with final counts
	// Note: license_violations is overwritten to include cargo-deny license issues
	metrics["license_violations"] = licenseCount
	metrics["bans_violations"] = bansCount
	// analysis_passed intentionally NOT updated - preserves original analyzer result
	// Assessment threshold pass/fail is captured in AssessmentResult.Success
}

// findExistingSBOM looks for existing SBOM files in standard locations
// Supports monorepo patterns by checking target directory and parent directories
func (r *DependenciesRunner) findExistingSBOM(target string) string {
	// Ensure target is absolute for consistent path operations
	absTarget, err := filepath.Abs(target)
	if err != nil {
		absTarget = target
	}

	// Standard SBOM locations relative to target
	targetPaths := []string{
		filepath.Join(absTarget, "sbom/goneat-latest.cdx.json"),
		filepath.Join(absTarget, "sbom.json"),
		filepath.Join(absTarget, ".sbom/cyclonedx.json"),
	}

	// Check target directory first
	for _, path := range targetPaths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}

	// For monorepo support: check parent directories (up to 3 levels)
	// This handles cases like: monorepo/packages/my-app where SBOM is at monorepo/sbom/
	currentDir := absTarget
	for i := 0; i < 3; i++ {
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break // Reached filesystem root
		}
		currentDir = parent

		parentPaths := []string{
			filepath.Join(currentDir, "sbom/goneat-latest.cdx.json"),
			filepath.Join(currentDir, "sbom.json"),
			filepath.Join(currentDir, ".sbom/cyclonedx.json"),
		}

		for _, path := range parentPaths {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path
			}
		}
	}

	return ""
}

// shouldPass determines if assessment should pass based on failure threshold.
// Uses canonical Crucible severity levels for comparison.
func (r *DependenciesRunner) shouldPass(issues []Issue, threshold IssueSeverity) bool {
	if len(issues) == 0 {
		return true
	}

	// Canonical Crucible severity levels (schemas/crucible-go/assessment/v1.0.0/severity-definitions.schema.json)
	// IMPORTANT: These values MUST match severityMapping in the Crucible schema.
	// Validated in TestDependenciesRunner_SeverityLevelsMatchCrucibleSchema.
	severityLevels := map[IssueSeverity]int{
		SeverityInfo:     0,
		SeverityLow:      1,
		SeverityMedium:   2,
		SeverityHigh:     3,
		SeverityCritical: 4,
	}

	thresholdLevel := severityLevels[threshold]

	// Fail if any issue meets or exceeds threshold
	for _, issue := range issues {
		if severityLevels[issue.Severity] >= thresholdLevel {
			return false
		}
	}

	return true
}

// CanRunInParallel implements AssessmentRunner.CanRunInParallel
func (r *DependenciesRunner) CanRunInParallel() bool {
	return false // Network calls to registries
}

// GetCategory implements AssessmentRunner.GetCategory
func (r *DependenciesRunner) GetCategory() AssessmentCategory {
	return CategoryDependencies
}

// GetEstimatedTime implements AssessmentRunner.GetEstimatedTime
func (r *DependenciesRunner) GetEstimatedTime(target string) time.Duration {
	return 15 * time.Second // Conservative estimate for dependency analysis
}

// IsAvailable implements AssessmentRunner.IsAvailable
// Returns true as we have analyzers (or stubs) for all detected languages
func (r *DependenciesRunner) IsAvailable() bool {
	return true
}

// init registers the dependencies assessment runner
func init() {
	RegisterAssessmentRunner(CategoryDependencies, NewDependenciesRunner())
}
