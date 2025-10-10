//go:build integration
// +build integration

package dependencies

import (
	"context"
	"testing"

	"github.com/fulmenhq/goneat/pkg/config"
)

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
