/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package dependencies

import (
	"context"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// PythonAnalyzer implements Analyzer for Python dependencies (stub)
type PythonAnalyzer struct{}

// NewPythonAnalyzer creates a new Python dependency analyzer
func NewPythonAnalyzer() Analyzer {
	return &PythonAnalyzer{}
}

// Analyze implements Analyzer.Analyze for Python (stub implementation)
func (a *PythonAnalyzer) Analyze(ctx context.Context, target string, config AnalysisConfig) (*AnalysisResult, error) {
	logger.Info("Python dependency analyzer is a stub (not yet implemented)")
	return &AnalysisResult{
		Dependencies: []Dependency{},
		Issues:       []Issue{},
		Passed:       true,
		Duration:     time.Duration(0),
	}, nil
}

// DetectLanguages implements Analyzer.DetectLanguages for Python
func (a *PythonAnalyzer) DetectLanguages(target string) ([]Language, error) {
	return []Language{LanguagePython}, nil
}
