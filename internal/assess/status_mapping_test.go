package assess

import (
	"context"
	"testing"
	"time"
)

// statusRunner allows simulating various result/err combinations.
type statusRunner struct {
	category AssessmentCategory
	result   *AssessmentResult
	err      error
}

func (s *statusRunner) Assess(ctx context.Context, target string, _ AssessmentConfig) (*AssessmentResult, error) {
	time.Sleep(5 * time.Millisecond)
	if s.result != nil {
		s.result.Category = s.category
	}
	return s.result, s.err
}
func (s *statusRunner) CanRunInParallel() bool                { return true }
func (s *statusRunner) GetCategory() AssessmentCategory       { return s.category }
func (s *statusRunner) GetEstimatedTime(string) time.Duration { return 0 }
func (s *statusRunner) IsAvailable() bool                     { return true }

func TestStatusMapping(t *testing.T) {
	old := GetAssessmentRunnerRegistry()
	globalRunnerRegistry = NewAssessmentRunnerRegistry()
	t.Cleanup(func() { globalRunnerRegistry = old })

	// success
	eng := NewAssessmentEngine()
	RegisterAssessmentRunner(CategoryLint, &statusRunner{category: CategoryLint, result: &AssessmentResult{CommandName: "lint", Success: true}})
	rpt, err := eng.RunAssessment(context.Background(), ".", DefaultAssessmentConfig())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if rpt.Categories[string(CategoryLint)].Status != "success" {
		t.Fatalf("expected success")
	}

	// error via err
	globalRunnerRegistry = NewAssessmentRunnerRegistry()
	eng = NewAssessmentEngine()
	RegisterAssessmentRunner(CategoryLint, &statusRunner{category: CategoryLint, err: assertErr{}})
	rpt, _ = eng.RunAssessment(context.Background(), ".", DefaultAssessmentConfig())
	if rpt.Categories[string(CategoryLint)].Status != "error" {
		t.Fatalf("expected error")
	}

	// skipped when result.Success=false and no error string
	globalRunnerRegistry = NewAssessmentRunnerRegistry()
	eng = NewAssessmentEngine()
	RegisterAssessmentRunner(CategoryLint, &statusRunner{category: CategoryLint, result: &AssessmentResult{CommandName: "lint", Success: false}})
	rpt, _ = eng.RunAssessment(context.Background(), ".", DefaultAssessmentConfig())
	if rpt.Categories[string(CategoryLint)].Status != "skipped" {
		t.Fatalf("expected skipped")
	}

	// error when result.Success=false but Error set
	globalRunnerRegistry = NewAssessmentRunnerRegistry()
	eng = NewAssessmentEngine()
	RegisterAssessmentRunner(CategoryLint, &statusRunner{category: CategoryLint, result: &AssessmentResult{CommandName: "lint", Success: false, Error: "x"}})
	rpt, _ = eng.RunAssessment(context.Background(), ".", DefaultAssessmentConfig())
	if rpt.Categories[string(CategoryLint)].Status != "error" {
		t.Fatalf("expected error")
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "fail" }
