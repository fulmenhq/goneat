package dependencies

import (
	"context"
)

// ShouldAlsoCoolRust reports whether a non-Rust primary language still has a
// Cargo.toml that must be cooled. First-match detection (go.mod before
// Cargo.toml) must not vacuous-pass polyglot trees.
func ShouldAlsoCoolRust(target string, primary Language) bool {
	if primary == LanguageRust {
		return false
	}
	project := DetectRustProject(target)
	return project != nil && project.CargoTomlPath != ""
}

// MergeAnalysisResults concatenates inventories and issues. Passed is AND.
func MergeAnalysisResults(base, extra *AnalysisResult) *AnalysisResult {
	if extra == nil {
		return base
	}
	if base == nil {
		return extra
	}
	merged := &AnalysisResult{
		Dependencies:    append(append([]Dependency{}, base.Dependencies...), extra.Dependencies...),
		Issues:          append(append([]Issue{}, base.Issues...), extra.Issues...),
		Passed:          base.Passed && extra.Passed,
		Duration:        base.Duration + extra.Duration,
		PackagesScanned: base.PackagesScanned,
	}
	if extra.PackagesScanned > merged.PackagesScanned {
		merged.PackagesScanned = extra.PackagesScanned
	}
	return merged
}

// RunCoolingAwareAnalysis runs the primary analyzer, then Rust cooling when
// CheckCooling is set and a Cargo.toml exists beside another language.
func RunCoolingAwareAnalysis(ctx context.Context, target string, cfg AnalysisConfig, primary Language, primaryAnalyzer, rustAnalyzer Analyzer) (*AnalysisResult, error) {
	result, err := primaryAnalyzer.Analyze(ctx, target, cfg)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = &AnalysisResult{Dependencies: []Dependency{}, Issues: []Issue{}, Passed: true}
	}

	if !cfg.CheckCooling || rustAnalyzer == nil || !ShouldAlsoCoolRust(target, primary) {
		return result, nil
	}

	rustCfg := cfg
	rustCfg.CheckLicenses = false
	rustCfg.CheckCooling = true
	rustCfg.Languages = []Language{LanguageRust}

	extra, err := rustAnalyzer.Analyze(ctx, target, rustCfg)
	if err != nil {
		return nil, err
	}
	return MergeAnalysisResults(result, extra), nil
}
