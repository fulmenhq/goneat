package assess

import (
	"context"
	"testing"
	"time"
)

func TestEnginePromotesSuppressions(t *testing.T) {
	old := GetAssessmentRunnerRegistry()
	globalRunnerRegistry = NewAssessmentRunnerRegistry()
	defer func() { globalRunnerRegistry = old }()

	// Fake security runner that returns metrics with _suppressions
	supps := []Suppression{{File: "a.go", Line: 10, RuleID: "G404", Reason: "intentional"}}
	RegisterAssessmentRunner(CategorySecurity, &statusRunner{
		category: CategorySecurity,
		result: &AssessmentResult{
			CommandName:   "security",
			Success:       true,
			ExecutionTime: HumanReadableDuration(time.Second),
			Issues:        []Issue{},
			Metrics:       map[string]interface{}{"_suppressions": supps},
		},
	})

	cfg := DefaultAssessmentConfig()
	cfg.TrackSuppressions = true
	eng := NewAssessmentEngine()
	rpt, err := eng.RunAssessment(context.Background(), ".", cfg)
	if err != nil {
		t.Fatalf("engine run failed: %v", err)
	}
	cr, ok := rpt.Categories[string(CategorySecurity)]
	if !ok {
		t.Fatalf("security category missing")
	}
	if cr.SuppressionReport == nil || len(cr.SuppressionReport.Suppressions) != 1 {
		t.Fatalf("suppression report not promoted: %#v", cr.SuppressionReport)
	}
}
