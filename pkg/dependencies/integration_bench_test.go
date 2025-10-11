//go:build integration
// +build integration

package dependencies

import (
	"context"
	"testing"
)

// Benchmark: Hugo (Small-Medium Repository)
func BenchmarkCoolingPolicy_Hugo(b *testing.B) {
	hugoPath := getTestRepoPath("hugo")
	if hugoPath == "" {
		b.Skip("Hugo repository not configured. Set GONEAT_COOLING_TEST_ROOT or clone to ~/dev/playground")
	}

	analyzer := NewGoAnalyzer()
	cfg := AnalysisConfig{
		Target:     hugoPath,
		PolicyPath: "testdata/policies/baseline.yaml",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(ctx, cfg.Target, cfg)
		if err != nil {
			b.Fatalf("Analysis failed: %v", err)
		}
	}
}

// Benchmark: OPA (Medium Repository)
func BenchmarkCoolingPolicy_OPA(b *testing.B) {
	opaPath := getTestRepoPath("opa")
	if opaPath == "" {
		b.Skip("OPA repository not configured. Set GONEAT_COOLING_TEST_ROOT or clone to ~/dev/playground")
	}

	analyzer := NewGoAnalyzer()
	cfg := AnalysisConfig{
		Target:     opaPath,
		PolicyPath: "testdata/policies/baseline.yaml",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(ctx, cfg.Target, cfg)
		if err != nil {
			b.Fatalf("Analysis failed: %v", err)
		}
	}
}

// Benchmark: Traefik (Large Repository)
func BenchmarkCoolingPolicy_Traefik(b *testing.B) {
	traefikPath := getTestRepoPath("traefik")
	if traefikPath == "" {
		traefikPath = getTestRepoPath("traefik-assessment")
	}
	if traefikPath == "" {
		b.Skip("Traefik repository not configured. Set GONEAT_COOLING_TEST_ROOT or clone to ~/dev/playground")
	}

	analyzer := NewGoAnalyzer()
	cfg := AnalysisConfig{
		Target:     traefikPath,
		PolicyPath: "testdata/policies/baseline.yaml",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(ctx, cfg.Target, cfg)
		if err != nil {
			b.Fatalf("Analysis failed: %v", err)
		}
	}
}

// Benchmark: Mattermost (Very Large Repository)
func BenchmarkCoolingPolicy_Mattermost(b *testing.B) {
	mattermostPath := getTestRepoPath("mattermost-server/server")
	if mattermostPath == "" {
		mattermostPath = getTestRepoPath("mattermost-server")
	}
	if mattermostPath == "" {
		b.Skip("Mattermost repository not configured. Set GONEAT_COOLING_TEST_ROOT or clone to ~/dev/playground")
	}

	analyzer := NewGoAnalyzer()
	cfg := AnalysisConfig{
		Target:     mattermostPath,
		PolicyPath: "testdata/policies/baseline.yaml",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(ctx, cfg.Target, cfg)
		if err != nil {
			b.Fatalf("Analysis failed: %v", err)
		}
	}
}

// Benchmark: Strict Policy Overhead
func BenchmarkCoolingPolicy_StrictPolicy(b *testing.B) {
	hugoPath := getTestRepoPath("hugo")
	if hugoPath == "" {
		b.Skip("Hugo repository not configured. Set GONEAT_COOLING_TEST_ROOT or clone to ~/dev/playground")
	}

	analyzer := NewGoAnalyzer()
	cfg := AnalysisConfig{
		Target:     hugoPath,
		PolicyPath: "testdata/policies/strict.yaml",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(ctx, cfg.Target, cfg)
		if err != nil {
			b.Fatalf("Analysis failed: %v", err)
		}
	}
}

// Benchmark: Exception Pattern Matching Overhead
func BenchmarkCoolingPolicy_Exceptions(b *testing.B) {
	traefikPath := getTestRepoPath("traefik")
	if traefikPath == "" {
		traefikPath = getTestRepoPath("traefik-assessment")
	}
	if traefikPath == "" {
		b.Skip("Traefik repository not configured. Set GONEAT_COOLING_TEST_ROOT or clone to ~/dev/playground")
	}

	analyzer := NewGoAnalyzer()
	cfg := AnalysisConfig{
		Target:     traefikPath,
		PolicyPath: "testdata/policies/exceptions.yaml",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(ctx, cfg.Target, cfg)
		if err != nil {
			b.Fatalf("Analysis failed: %v", err)
		}
	}
}

// Benchmark: Disabled Cooling (Baseline Performance)
func BenchmarkCoolingPolicy_Disabled(b *testing.B) {
	hugoPath := getTestRepoPath("hugo")
	if hugoPath == "" {
		b.Skip("Hugo repository not configured. Set GONEAT_COOLING_TEST_ROOT or clone to ~/dev/playground")
	}

	analyzer := NewGoAnalyzer()
	cfg := AnalysisConfig{
		Target:     hugoPath,
		PolicyPath: "testdata/policies/disabled.yaml",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(ctx, cfg.Target, cfg)
		if err != nil {
			b.Fatalf("Analysis failed: %v", err)
		}
	}
}
