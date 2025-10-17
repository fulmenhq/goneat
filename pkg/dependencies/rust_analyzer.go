/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package dependencies

import (
	"context"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// RustAnalyzer implements Analyzer for Rust dependencies (stub)
type RustAnalyzer struct{}

// NewRustAnalyzer creates a new Rust dependency analyzer
func NewRustAnalyzer() Analyzer {
	return &RustAnalyzer{}
}

// Analyze implements Analyzer.Analyze for Rust (stub implementation)
func (a *RustAnalyzer) Analyze(ctx context.Context, target string, config AnalysisConfig) (*AnalysisResult, error) {
	logger.Info("Rust dependency analyzer is a stub (not yet implemented)")
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
