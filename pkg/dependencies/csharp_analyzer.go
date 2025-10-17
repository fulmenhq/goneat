/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package dependencies

import (
	"context"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// CSharpAnalyzer implements Analyzer for C# dependencies (stub)
type CSharpAnalyzer struct{}

// NewCSharpAnalyzer creates a new C# dependency analyzer
func NewCSharpAnalyzer() Analyzer {
	return &CSharpAnalyzer{}
}

// Analyze implements Analyzer.Analyze for C# (stub implementation)
func (a *CSharpAnalyzer) Analyze(ctx context.Context, target string, config AnalysisConfig) (*AnalysisResult, error) {
	logger.Info("C# dependency analyzer is a stub (not yet implemented)")
	return &AnalysisResult{
		Dependencies: []Dependency{},
		Issues:       []Issue{},
		Passed:       true,
		Duration:     time.Duration(0),
	}, nil
}

// DetectLanguages implements Analyzer.DetectLanguages for C#
func (a *CSharpAnalyzer) DetectLanguages(target string) ([]Language, error) {
	return []Language{LanguageCSharp}, nil
}
