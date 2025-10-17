/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package dependencies

import (
	"context"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// TypeScriptAnalyzer implements Analyzer for TypeScript/JavaScript dependencies (stub)
type TypeScriptAnalyzer struct{}

// NewTypeScriptAnalyzer creates a new TypeScript dependency analyzer
func NewTypeScriptAnalyzer() Analyzer {
	return &TypeScriptAnalyzer{}
}

// Analyze implements Analyzer.Analyze for TypeScript (stub implementation)
func (a *TypeScriptAnalyzer) Analyze(ctx context.Context, target string, config AnalysisConfig) (*AnalysisResult, error) {
	logger.Info("TypeScript dependency analyzer is a stub (not yet implemented)")
	return &AnalysisResult{
		Dependencies: []Dependency{},
		Issues:       []Issue{},
		Passed:       true,
		Duration:     time.Duration(0),
	}, nil
}

// DetectLanguages implements Analyzer.DetectLanguages for TypeScript
func (a *TypeScriptAnalyzer) DetectLanguages(target string) ([]Language, error) {
	return []Language{LanguageTypeScript}, nil
}
