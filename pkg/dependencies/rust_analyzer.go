/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package dependencies

import (
	"context"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// RustAnalyzer implements Analyzer for Rust dependencies.
// Note: The actual cargo-deny integration is handled in DependenciesRunner
// to avoid import cycles between internal/assess and pkg/dependencies.
type RustAnalyzer struct{}

// NewRustAnalyzer creates a new Rust dependency analyzer
func NewRustAnalyzer() Analyzer {
	return &RustAnalyzer{}
}

// Analyze implements Analyzer.Analyze for Rust.
// Returns empty result as cargo-deny checks are handled directly by DependenciesRunner.
func (a *RustAnalyzer) Analyze(ctx context.Context, target string, config AnalysisConfig) (*AnalysisResult, error) {
	logger.Debug("Rust dependency analysis delegated to cargo-deny in DependenciesRunner")
	return &AnalysisResult{
		Dependencies: []Dependency{},
		Issues:       []Issue{},
		Passed:       true,
		Duration:     time.Duration(0),
	}, nil
}

// DetectLanguages implements Analyzer.DetectLanguages for Rust
func (a *RustAnalyzer) DetectLanguages(target string) ([]Language, error) {
	return []Language{LanguageRust}, nil
}
