package assess

import (
	"context"
	"errors"
	"testing"
	"time"
)

// helper to build a simple issue
func mkIssue(file string, sev IssueSeverity) Issue {
	return Issue{File: file, Line: 1, Severity: sev, Message: "m", Category: CategoryFormat}
}

func TestEngine_RunAssessment_SuccessAndOrdering(t *testing.T) {
	// Save current registry and restore after
	old := GetAssessmentRunnerRegistry()
	// Use a fresh registry for test isolation
	globalRunnerRegistry = NewAssessmentRunnerRegistry()
	t.Cleanup(func() { globalRunnerRegistry = old })

	// Register two categories with different priorities (format=1, lint=4)
	rFormat := &fakeRunner{category: CategoryFormat, available: true, canParallel: true,
		delay:  10 * time.Millisecond,
		issues: []Issue{{File: "a.go", Line: 3, Severity: SeverityLow, Message: "fmt", Category: CategoryFormat}},
	}
	rLint := &fakeRunner{category: CategoryLint, available: true, canParallel: true,
		delay:  10 * time.Millisecond,
		issues: []Issue{{File: "b.go", Line: 2, Severity: SeverityHigh, Message: "lint", Category: CategoryLint}},
	}
	RegisterAssessmentRunner(CategoryFormat, rFormat)
	RegisterAssessmentRunner(CategoryLint, rLint)

	engine := NewAssessmentEngine()
	cfg := DefaultAssessmentConfig()
	cfg.Concurrency = 1 // deterministic ordering path

	rpt, err := engine.RunAssessment(context.Background(), ".", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rpt.Categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(rpt.Categories))
	}
	// Summary rollup
	if rpt.Summary.TotalIssues != 2 {
		t.Fatalf("expected total issues 2, got %d", rpt.Summary.TotalIssues)
	}
	// Workflow phases are grouped by priority; Phase 1 should include format
	foundPhase1 := false
	for _, ph := range rpt.Workflow.Phases {
		if ph.Priority == 1 {
			foundPhase1 = true
			// Contains format
			hasFmt := false
			for _, c := range ph.Categories {
				if c == CategoryFormat {
					hasFmt = true
					break
				}
			}
			if !hasFmt {
				t.Fatalf("phase 1 missing format category")
			}
		}
	}
	if !foundPhase1 {
		t.Fatalf("expected a priority-1 phase in workflow")
	}
}

func TestEngine_RunAssessment_ErrorAndTimeout(t *testing.T) {
	old := GetAssessmentRunnerRegistry()
	globalRunnerRegistry = NewAssessmentRunnerRegistry()
	t.Cleanup(func() { globalRunnerRegistry = old })

	// Runner that errors immediately
	rErr := &fakeRunner{category: CategoryLint, available: true, returnError: errors.New("boom")}
	// Runner that exceeds timeout and observes ctx cancellation
	rSlow := &fakeRunner{category: CategoryFormat, available: true, delay: 200 * time.Millisecond}
	RegisterAssessmentRunner(CategoryLint, rErr)
	RegisterAssessmentRunner(CategoryFormat, rSlow)

	engine := NewAssessmentEngine()
	cfg := DefaultAssessmentConfig()
	cfg.Timeout = 50 * time.Millisecond
	cfg.Concurrency = 1

	rpt, err := engine.RunAssessment(context.Background(), ".", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lf, ok := rpt.Categories[string(CategoryLint)]
	if !ok || lf.Status != "error" || lf.Error == "" {
		t.Fatalf("expected lint category error recorded, got: %+v", lf)
	}
	ff, ok := rpt.Categories[string(CategoryFormat)]
	if !ok || ff.Status != "error" { // timeout manifests as error
		t.Fatalf("expected format timeout recorded as error, got: %+v", ff)
	}
}

func TestEngine_SelectedCategoriesFilter(t *testing.T) {
	old := GetAssessmentRunnerRegistry()
	globalRunnerRegistry = NewAssessmentRunnerRegistry()
	t.Cleanup(func() { globalRunnerRegistry = old })

	RegisterAssessmentRunner(CategoryFormat, &fakeRunner{category: CategoryFormat, available: true})
	RegisterAssessmentRunner(CategoryLint, &fakeRunner{category: CategoryLint, available: true})

	engine := NewAssessmentEngine()
	cfg := DefaultAssessmentConfig()
	cfg.SelectedCategories = []string{"lint"}
	cfg.Concurrency = 1
	rpt, err := engine.RunAssessment(context.Background(), ".", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rpt.Categories) != 1 {
		t.Fatalf("expected 1 category due to filter, got %d", len(rpt.Categories))
	}
	if _, ok := rpt.Categories[string(CategoryLint)]; !ok {
		t.Fatalf("expected lint category present after filter")
	}
}

func TestEngine_ConcurrencyMappingAffectsWallTime(t *testing.T) {
	old := GetAssessmentRunnerRegistry()
	globalRunnerRegistry = NewAssessmentRunnerRegistry()
	t.Cleanup(func() { globalRunnerRegistry = old })

	// Two runners with noticeable delay
	RegisterAssessmentRunner(CategoryFormat, &fakeRunner{category: CategoryFormat, available: true, delay: 150 * time.Millisecond})
	RegisterAssessmentRunner(CategoryLint, &fakeRunner{category: CategoryLint, available: true, delay: 150 * time.Millisecond})

	engine := NewAssessmentEngine()

	// Sequential
	cfgSeq := DefaultAssessmentConfig()
	cfgSeq.Concurrency = 1
	rptSeq, err := engine.RunAssessment(context.Background(), ".", cfgSeq)
	if err != nil {
		t.Fatalf("seq run error: %v", err)
	}

	// Parallel-ish (2 workers)
	cfgPar := DefaultAssessmentConfig()
	cfgPar.Concurrency = 2
	rptPar, err := engine.RunAssessment(context.Background(), ".", cfgPar)
	if err != nil {
		t.Fatalf("par run error: %v", err)
	}

	// Expect parallel execution to be faster than sequential by a meaningful margin
	if rptPar.Metadata.ExecutionTime >= rptSeq.Metadata.ExecutionTime {
		t.Fatalf("expected parallel exec time %v < sequential %v", rptPar.Metadata.ExecutionTime, rptSeq.Metadata.ExecutionTime)
	}
}

func TestEngine_CustomPriorities(t *testing.T) {
	old := GetAssessmentRunnerRegistry()
	globalRunnerRegistry = NewAssessmentRunnerRegistry()
	t.Cleanup(func() { globalRunnerRegistry = old })

	// Register runners with default priorities (format=1, lint=4)
	RegisterAssessmentRunner(CategoryFormat, &fakeRunner{category: CategoryFormat, available: true,
		issues: []Issue{mkIssue("a.go", SeverityLow)}})
	RegisterAssessmentRunner(CategoryLint, &fakeRunner{category: CategoryLint, available: true,
		issues: []Issue{mkIssue("b.go", SeverityHigh)}})

	engine := NewAssessmentEngine()
	cfg := DefaultAssessmentConfig()
	cfg.Concurrency = 1
	cfg.PriorityString = "lint=1,format=2" // Swap priorities

	rpt, err := engine.RunAssessment(context.Background(), ".", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With swapped priorities, lint should run first (phase 1), format second (phase 2)
	phases := rpt.Workflow.Phases
	if len(phases) < 2 {
		t.Fatalf("expected at least 2 phases, got %d", len(phases))
	}

	// Phase 1 should contain lint (priority 1)
	phase1 := phases[0]
	if phase1.Priority != 1 {
		t.Fatalf("expected phase 1 priority to be 1, got %d", phase1.Priority)
	}
	hasLint := false
	for _, cat := range phase1.Categories {
		if cat == CategoryLint {
			hasLint = true
			break
		}
	}
	if !hasLint {
		t.Fatalf("expected lint category in phase 1")
	}
}

func TestEngine_InvalidPriorityString(t *testing.T) {
	old := GetAssessmentRunnerRegistry()
	globalRunnerRegistry = NewAssessmentRunnerRegistry()
	t.Cleanup(func() { globalRunnerRegistry = old })

	RegisterAssessmentRunner(CategoryFormat, &fakeRunner{category: CategoryFormat, available: true})

	engine := NewAssessmentEngine()
	cfg := DefaultAssessmentConfig()
	cfg.PriorityString = "invalid-priority-string"

	_, err := engine.RunAssessment(context.Background(), ".", cfg)
	if err == nil {
		t.Fatalf("expected error for invalid priority string")
	}
}

func TestEngine_WorkflowPlanningEdgeCases(t *testing.T) {
	old := GetAssessmentRunnerRegistry()
	globalRunnerRegistry = NewAssessmentRunnerRegistry()
	t.Cleanup(func() { globalRunnerRegistry = old })

	// Test with no issues
	RegisterAssessmentRunner(CategoryFormat, &fakeRunner{category: CategoryFormat, available: true, issues: []Issue{}})

	engine := NewAssessmentEngine()
	cfg := DefaultAssessmentConfig()
	cfg.Concurrency = 1

	rpt, err := engine.RunAssessment(context.Background(), ".", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have no phases when no issues
	if len(rpt.Workflow.Phases) != 0 {
		t.Fatalf("expected no phases with no issues, got %d", len(rpt.Workflow.Phases))
	}

	// Health should be perfect (1.0)
	if rpt.Summary.OverallHealth != 1.0 {
		t.Fatalf("expected perfect health with no issues, got %f", rpt.Summary.OverallHealth)
	}
}

func TestEngine_HealthCalculation(t *testing.T) {
	old := GetAssessmentRunnerRegistry()
	globalRunnerRegistry = NewAssessmentRunnerRegistry()
	t.Cleanup(func() { globalRunnerRegistry = old })

	// Test various severity combinations
	testCases := []struct {
		name        string
		issues      []Issue
		expectedMin float64
		expectedMax float64
	}{
		{"no issues", []Issue{}, 1.0, 1.0},
		{"low severity", []Issue{mkIssue("a.go", SeverityLow)}, 0.99, 1.0},
		{"high severity", []Issue{mkIssue("a.go", SeverityHigh)}, 0.95, 0.96},
		{"critical severity", []Issue{mkIssue("a.go", SeverityCritical)}, 0.9, 0.91},
		{"mixed severities", []Issue{
			mkIssue("a.go", SeverityLow),
			mkIssue("b.go", SeverityHigh),
			mkIssue("c.go", SeverityCritical),
		}, 0.83, 0.85},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			globalRunnerRegistry = NewAssessmentRunnerRegistry()
			RegisterAssessmentRunner(CategoryFormat, &fakeRunner{
				category:  CategoryFormat,
				available: true,
				issues:    tc.issues,
			})

			engine := NewAssessmentEngine()
			cfg := DefaultAssessmentConfig()
			cfg.Concurrency = 1

			rpt, err := engine.RunAssessment(context.Background(), ".", cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			health := rpt.Summary.OverallHealth
			if health < tc.expectedMin || health > tc.expectedMax {
				t.Fatalf("expected health between %f and %f, got %f", tc.expectedMin, tc.expectedMax, health)
			}
		})
	}
}

func TestEngine_ConcurrencyPercentCalculation(t *testing.T) {
	old := GetAssessmentRunnerRegistry()
	globalRunnerRegistry = NewAssessmentRunnerRegistry()
	t.Cleanup(func() { globalRunnerRegistry = old })

	RegisterAssessmentRunner(CategoryFormat, &fakeRunner{category: CategoryFormat, available: true})

	engine := NewAssessmentEngine()

	testCases := []struct {
		name            string
		concurrency     int
		concurrencyPct  int
		expectedWorkers int
	}{
		{"explicit concurrency", 3, 50, 3},
		{"percent calculation", 0, 100, 1}, // Will be at least 1
		{"zero percent defaults", 0, 0, 1}, // Will be at least 1
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultAssessmentConfig()
			cfg.Concurrency = tc.concurrency
			cfg.ConcurrencyPercent = tc.concurrencyPct

			// We can't easily test the exact worker count without mocking runtime.NumCPU()
			// But we can verify the assessment runs without error
			_, err := engine.RunAssessment(context.Background(), ".", cfg)
			if err != nil {
				t.Fatalf("unexpected error with concurrency config: %v", err)
			}
		})
	}
}

func TestEngine_TimeoutHandling(t *testing.T) {
	old := GetAssessmentRunnerRegistry()
	globalRunnerRegistry = NewAssessmentRunnerRegistry()
	t.Cleanup(func() { globalRunnerRegistry = old })

	// Runner that takes longer than timeout
	RegisterAssessmentRunner(CategoryFormat, &fakeRunner{
		category:  CategoryFormat,
		available: true,
		delay:     200 * time.Millisecond,
	})

	engine := NewAssessmentEngine()
	cfg := DefaultAssessmentConfig()
	cfg.Timeout = 50 * time.Millisecond
	cfg.Concurrency = 1

	rpt, err := engine.RunAssessment(context.Background(), ".", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have recorded timeout as error
	formatResult, exists := rpt.Categories[string(CategoryFormat)]
	if !exists {
		t.Fatalf("expected format category result")
	}
	if formatResult.Status != "error" {
		t.Fatalf("expected timeout to be recorded as error, got status: %s", formatResult.Status)
	}
}

func TestEngine_UnavailableRunners(t *testing.T) {
	old := GetAssessmentRunnerRegistry()
	globalRunnerRegistry = NewAssessmentRunnerRegistry()
	t.Cleanup(func() { globalRunnerRegistry = old })

	// Register runner that's not available
	RegisterAssessmentRunner(CategoryFormat, &fakeRunner{category: CategoryFormat, available: false})
	// Register runner that is available
	RegisterAssessmentRunner(CategoryLint, &fakeRunner{category: CategoryLint, available: true,
		issues: []Issue{mkIssue("a.go", SeverityLow)}})

	engine := NewAssessmentEngine()
	cfg := DefaultAssessmentConfig()
	cfg.Concurrency = 1

	rpt, err := engine.RunAssessment(context.Background(), ".", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only have results for available categories
	if len(rpt.Categories) != 1 {
		t.Fatalf("expected 1 category result (only available ones), got %d", len(rpt.Categories))
	}

	if _, exists := rpt.Categories[string(CategoryLint)]; !exists {
		t.Fatalf("expected lint category result")
	}
}

func TestFormatRunner_IsAvailable(t *testing.T) {
	runner := NewFormatAssessmentRunner()

	// Format runner should be available (doesn't depend on external tools)
	if !runner.IsAvailable() {
		t.Fatalf("expected format runner to be available")
	}
}

func TestFormatRunner_GetCategory(t *testing.T) {
	runner := NewFormatAssessmentRunner()

	if runner.GetCategory() != CategoryFormat {
		t.Fatalf("expected CategoryFormat, got %s", runner.GetCategory())
	}
}

func TestFormatRunner_CanRunInParallel(t *testing.T) {
	runner := NewFormatAssessmentRunner()

	// Format runner should support parallel execution
	if !runner.CanRunInParallel() {
		t.Fatalf("expected format runner to support parallel execution")
	}
}

func TestSecurityRunner_IsAvailable(t *testing.T) {
	runner := NewSecurityAssessmentRunner()

	// Security runner availability depends on external tools
	// We can't guarantee availability in test environment, so just check the method exists
	_ = runner.IsAvailable() // Should not panic
}

func TestSecurityRunner_GetCategory(t *testing.T) {
	runner := NewSecurityAssessmentRunner()

	if runner.GetCategory() != CategorySecurity {
		t.Fatalf("expected CategorySecurity, got %s", runner.GetCategory())
	}
}

func TestLintRunner_IsAvailable(t *testing.T) {
	runner := NewLintAssessmentRunner()

	// Lint runner availability depends on external tools like golangci-lint
	// We can't guarantee availability in test environment, so just check the method exists
	_ = runner.IsAvailable() // Should not panic
}

func TestLintRunner_GetCategory(t *testing.T) {
	runner := NewLintAssessmentRunner()

	if runner.GetCategory() != CategoryLint {
		t.Fatalf("expected CategoryLint, got %s", runner.GetCategory())
	}
}

func TestStaticAnalysisRunner_IsAvailable(t *testing.T) {
	runner := NewStaticAnalysisAssessmentRunner()

	// Static analysis runner availability depends on external tools
	// We can't guarantee availability in test environment, so just check the method exists
	_ = runner.IsAvailable() // Should not panic
}

func TestStaticAnalysisRunner_GetCategory(t *testing.T) {
	runner := NewStaticAnalysisAssessmentRunner()

	if runner.GetCategory() != CategoryStaticAnalysis {
		t.Fatalf("expected CategoryStaticAnalysis, got %s", runner.GetCategory())
	}
}
